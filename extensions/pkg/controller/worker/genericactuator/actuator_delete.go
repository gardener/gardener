// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package genericactuator

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/gardener/gardener/extensions/pkg/controller"

	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	forceDeletionLabelKey   = "force-deletion"
	forceDeletionLabelValue = "True"
)

func (a *genericActuator) Delete(ctx context.Context, worker *extensionsv1alpha1.Worker, cluster *controller.Cluster) error {
	workerDelegate, err := a.delegateFactory.WorkerDelegate(ctx, worker, cluster)
	if err != nil {
		return errors.Wrapf(err, "could not instantiate actuator context")
	}

	// Make sure machine-controller-manager is awake before deleting the machines.
	var replicaFunc = func() (int32, error) {
		return 1, nil
	}

	// Deploy the machine-controller-manager into the cluster to make sure worker nodes can be removed.
	a.logger.Info("Deploying the machine-controller-manager", "worker", fmt.Sprintf("%s/%s", worker.Namespace, worker.Name))
	if err := a.deployMachineControllerManager(ctx, worker, cluster, workerDelegate, replicaFunc); err != nil {
		return err
	}

	// Redeploy generated machine classes to update credentials machine-controller-manager used.
	a.logger.Info("Deploying the machine classes", "worker", fmt.Sprintf("%s/%s", worker.Namespace, worker.Name))
	if err := workerDelegate.DeployMachineClasses(ctx); err != nil {
		return errors.Wrapf(err, "failed to deploy the machine classes")
	}

	// Mark all existing machines to become forcefully deleted.
	a.logger.Info("Deleting all machines", "worker", fmt.Sprintf("%s/%s", worker.Namespace, worker.Name))
	if err := a.markAllMachinesForcefulDeletion(ctx, worker.Namespace); err != nil {
		return errors.Wrapf(err, "marking all machines for forceful deletion failed")
	}

	// Get the list of all existing machine deployments.
	existingMachineDeployments := &machinev1alpha1.MachineDeploymentList{}
	if err := a.client.List(ctx, existingMachineDeployments, client.InNamespace(worker.Namespace)); err != nil {
		return err
	}

	// Delete all machine deployments.
	if err := a.cleanupMachineDeployments(ctx, existingMachineDeployments, nil); err != nil {
		return errors.Wrapf(err, "cleaning up machine deployments failed")
	}

	// Delete all machine classes.
	if err := a.cleanupMachineClasses(ctx, worker.Namespace, workerDelegate.MachineClassList(), nil); err != nil {
		return errors.Wrapf(err, "cleaning up machine classes failed")
	}

	// Delete all machine class secrets.
	if err := a.cleanupMachineClassSecrets(ctx, worker.Namespace, nil); err != nil {
		return errors.Wrapf(err, "cleaning up machine class secrets failed")
	}

	// Wait until all machine resources have been properly deleted.
	timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	if err := a.waitUntilMachineResourcesDeleted(timeoutCtx, worker, workerDelegate); err != nil {
		return gardencorev1beta1helper.DetermineError(err, fmt.Sprintf("Failed while waiting for all machine resources to be deleted: '%s'", err.Error()))
	}

	// Delete the machine-controller-manager.
	if err := a.deleteMachineControllerManager(ctx, worker); err != nil {
		return errors.Wrapf(err, "failed deleting machine-controller-manager")
	}

	return nil
}

// Mark all existing machines to become forcefully deleted.
func (a *genericActuator) markAllMachinesForcefulDeletion(ctx context.Context, namespace string) error {
	// Mark all existing machines to become forcefully deleted.
	existingMachines := &machinev1alpha1.MachineList{}
	if err := a.client.List(ctx, existingMachines, client.InNamespace(namespace)); err != nil {
		return err
	}

	var (
		errorList []error
		wg        sync.WaitGroup
	)

	// TODO: Use github.com/gardener/gardener/pkg/utils/flow.Parallel as soon as we can vendor a new Gardener version again.
	for _, machine := range existingMachines.Items {
		wg.Add(1)
		go func(m machinev1alpha1.Machine) {
			defer wg.Done()
			if err := a.markMachineForcefulDeletion(ctx, &m); err != nil {
				errorList = append(errorList, err)
			}
		}(machine)
	}

	wg.Wait()
	if len(errorList) > 0 {
		return fmt.Errorf("labelling machines (to become forcefully deleted) failed: %v", errorList)
	}

	return nil
}

// markMachineForcefulDeletion labels a machine object to become forcefully deleted.
func (a *genericActuator) markMachineForcefulDeletion(ctx context.Context, machine *machinev1alpha1.Machine) error {
	if machine.Labels == nil {
		machine.Labels = map[string]string{}
	}

	if val, ok := machine.Labels[forceDeletionLabelKey]; ok && val == forceDeletionLabelValue {
		return nil
	}

	machine.Labels[forceDeletionLabelKey] = forceDeletionLabelValue
	return a.client.Update(ctx, machine)
}

// waitUntilMachineResourcesDeleted waits for a maximum of 30 minutes until all machine resources have been properly
// deleted by the machine-controller-manager. It polls the status every 5 seconds.
// TODO: Parallelise this?
func (a *genericActuator) waitUntilMachineResourcesDeleted(ctx context.Context, worker *extensionsv1alpha1.Worker, workerDelegate WorkerDelegate) error {
	var (
		countMachines            = -1
		countMachineSets         = -1
		countMachineDeployments  = -1
		countMachineClasses      = -1
		countMachineClassSecrets = -1
	)

	return wait.PollUntil(5*time.Second, func() (bool, error) {
		msg := ""

		// Check whether all machines have been deleted.
		if countMachines != 0 {
			existingMachines := &machinev1alpha1.MachineList{}
			if err := a.client.List(ctx, existingMachines, client.InNamespace(worker.Namespace)); err != nil {
				return false, err
			}
			countMachines = len(existingMachines.Items)
			msg += fmt.Sprintf("%d machines, ", countMachines)
		}

		// Check whether all machine sets have been deleted.
		if countMachineSets != 0 {
			existingMachineSets := &machinev1alpha1.MachineSetList{}
			if err := a.client.List(ctx, existingMachineSets, client.InNamespace(worker.Namespace)); err != nil {
				return false, err
			}
			countMachineSets = len(existingMachineSets.Items)
			msg += fmt.Sprintf("%d machine sets, ", countMachineSets)
		}

		// Check whether all machine deployments have been deleted.
		if countMachineDeployments != 0 {
			existingMachineDeployments := &machinev1alpha1.MachineDeploymentList{}
			if err := a.client.List(ctx, existingMachineDeployments, client.InNamespace(worker.Namespace)); err != nil {
				return false, err
			}
			countMachineDeployments = len(existingMachineDeployments.Items)
			msg += fmt.Sprintf("%d machine deployments, ", countMachineDeployments)

			// Check whether an operation failed during the deletion process.
			for _, existingMachineDeployment := range existingMachineDeployments.Items {
				for _, failedMachine := range existingMachineDeployment.Status.FailedMachines {
					return false, fmt.Errorf("Machine %s failed: %s", failedMachine.Name, failedMachine.LastOperation.Description)
				}
			}
		}

		// Check whether all machine classes have been deleted.
		if countMachineClasses != 0 {
			machineClassList := workerDelegate.MachineClassList()
			if err := a.client.List(ctx, machineClassList, client.InNamespace(worker.Namespace)); err != nil {
				return false, err
			}
			machineClasses, err := meta.ExtractList(machineClassList)
			if err != nil {
				return false, err
			}
			countMachineClasses = len(machineClasses)
			msg += fmt.Sprintf("%d machine classes, ", countMachineClasses)
		}

		// Check whether all machine class secrets have been deleted.
		if countMachineClassSecrets != 0 {
			count := 0
			existingMachineClassSecrets, err := a.listMachineClassSecrets(ctx, worker.Namespace)
			if err != nil {
				return false, err
			}
			for _, machineClassSecret := range existingMachineClassSecrets.Items {
				if len(machineClassSecret.Finalizers) != 0 {
					count++
				}
			}
			countMachineClassSecrets = count
			msg += fmt.Sprintf("%d machine class secrets, ", countMachineClassSecrets)
		}

		if countMachines != 0 || countMachineSets != 0 || countMachineDeployments != 0 || countMachineClasses != 0 || countMachineClassSecrets != 0 {
			a.logger.Info(fmt.Sprintf("Waiting until the following machine resources have been processed: %s", strings.TrimSuffix(msg, ", ")), "worker", fmt.Sprintf("%s/%s", worker.Namespace, worker.Name))
			return false, nil
		}
		return true, nil
	}, ctx.Done())
}
