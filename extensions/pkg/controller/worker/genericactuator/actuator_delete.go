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
	"errors"
	"fmt"
	"strings"
	"time"

	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/utils/flow"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	retryutils "github.com/gardener/gardener/pkg/utils/retry"
)

const (
	forceDeletionLabelKey   = "force-deletion"
	forceDeletionLabelValue = "True"
)

func (a *genericActuator) Delete(ctx context.Context, worker *extensionsv1alpha1.Worker, cluster *extensionscontroller.Cluster) error {
	logger := a.logger.WithValues("worker", client.ObjectKeyFromObject(worker), "operation", "delete")

	workerDelegate, err := a.delegateFactory.WorkerDelegate(ctx, worker, cluster)
	if err != nil {
		return fmt.Errorf("could not instantiate actuator context: %w", err)
	}

	// Make sure machine-controller-manager is awake before deleting the machines.
	var replicaFunc = func() (int32, error) {
		return 1, nil
	}

	// Deploy the machine-controller-manager into the cluster to make sure worker nodes can be removed.
	if err := a.deployMachineControllerManager(ctx, logger, worker, cluster, workerDelegate, replicaFunc); err != nil {
		return err
	}

	// Redeploy generated machine classes to update credentials machine-controller-manager used.
	logger.Info("Deploying the machine classes")
	if err := workerDelegate.DeployMachineClasses(ctx); err != nil {
		return fmt.Errorf("failed to deploy the machine classes: %w", err)
	}

	// Wait until the machine class credentials secret has been acquired.
	logger.Info("Waiting until the machine class credentials secret has been acquired")
	if err := a.waitUntilCredentialsSecretAcquiredOrReleased(ctx, true, worker, workerDelegate); err != nil {
		return fmt.Errorf("failed while waiting for the machine class credentials secret to be acquired: %w", err)
	}

	if workerCredentialsDelegate, ok := workerDelegate.(WorkerCredentialsDelegate); ok {
		// Update cloud credentials for all existing machine class secrets
		cloudCredentials, err := workerCredentialsDelegate.GetMachineControllerManagerCloudCredentials(ctx)
		if err != nil {
			return fmt.Errorf("failed to get the cloud credentials in namespace %s: %w", worker.Namespace, err)
		}
		if err = a.updateCloudCredentialsInAllMachineClassSecrets(ctx, logger, cloudCredentials, worker.Namespace); err != nil {
			return fmt.Errorf("failed to update cloud credentials in machine class secrets for namespace %s: %w", worker.Namespace, err)
		}
	}

	// Mark all existing machines to become forcefully deleted.
	logger.Info("Marking all machines to become forcefully deleted")
	if err := a.markAllMachinesForcefulDeletion(ctx, logger, worker.Namespace); err != nil {
		return fmt.Errorf("marking all machines for forceful deletion failed: %w", err)
	}

	// Delete all machine deployments.
	logger.Info("Deleting all machine deployments")
	if err := a.client.DeleteAllOf(ctx, &machinev1alpha1.MachineDeployment{}, client.InNamespace(worker.Namespace)); err != nil {
		return fmt.Errorf("cleaning up all machine deployments failed: %w", err)
	}

	// Delete all machine classes.
	logger.Info("Deleting all machine classes")
	if err := a.client.DeleteAllOf(ctx, workerDelegate.MachineClass(), client.InNamespace(worker.Namespace)); err != nil {
		return fmt.Errorf("cleaning up all machine classes failed: %w", err)
	}

	// Delete all machine class secrets.
	logger.Info("Deleting all machine class secrets")
	if err := a.client.DeleteAllOf(ctx, &corev1.Secret{}, client.InNamespace(worker.Namespace), client.MatchingLabels(getMachineClassSecretLabels())); err != nil {
		return fmt.Errorf("cleaning up all machine class secrets failed: %w", err)
	}

	// Wait until all machine resources have been properly deleted.
	if err := a.waitUntilMachineResourcesDeleted(ctx, logger, worker, workerDelegate); err != nil {
		return gardencorev1beta1helper.DetermineError(err, fmt.Sprintf("Failed while waiting for all machine resources to be deleted: '%s'", err.Error()))
	}

	// Wait until the machine class credentials secret has been released.
	logger.Info("Waiting until the machine class credentials secret has been released")
	if err := a.waitUntilCredentialsSecretAcquiredOrReleased(ctx, false, worker, workerDelegate); err != nil {
		return fmt.Errorf("failed while waiting for the machine class credentials secret to be released: %w", err)
	}

	// Delete the machine-controller-manager.
	if err := a.deleteMachineControllerManager(ctx, logger, worker); err != nil {
		return fmt.Errorf("failed deleting machine-controller-manager: %w", err)
	}

	// Cleanup machine dependencies.
	if err := workerDelegate.CleanupMachineDependencies(ctx); err != nil {
		return fmt.Errorf("failed to cleanup machine dependencies: %w", err)
	}

	return nil
}

// Mark all existing machines to become forcefully deleted.
func (a *genericActuator) markAllMachinesForcefulDeletion(ctx context.Context, logger logr.Logger, namespace string) error {
	logger.Info("Marking all machines for forceful deletion")
	// Mark all existing machines to become forcefully deleted.
	existingMachines := &machinev1alpha1.MachineList{}
	if err := a.client.List(ctx, existingMachines, client.InNamespace(namespace)); err != nil {
		return err
	}

	var tasks []flow.TaskFn
	for _, machine := range existingMachines.Items {
		m := machine
		tasks = append(tasks, func(ctx context.Context) error {
			return a.markMachineForcefulDeletion(ctx, &m)
		})
	}

	if err := flow.Parallel(tasks...)(ctx); err != nil {
		return fmt.Errorf("failed labelling machines for forceful deletion: %w", err)
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
func (a *genericActuator) waitUntilMachineResourcesDeleted(ctx context.Context, logger logr.Logger, worker *extensionsv1alpha1.Worker, workerDelegate WorkerDelegate) error {
	var (
		countMachines            = -1
		countMachineSets         = -1
		countMachineDeployments  = -1
		countMachineClasses      = -1
		countMachineClassSecrets = -1
	)
	logger.Info("Waiting until all machine resources have been deleted")

	return retryutils.UntilTimeout(ctx, 5*time.Second, 5*time.Minute, func(ctx context.Context) (bool, error) {
		msg := ""

		// Check whether all machines have been deleted.
		if countMachines != 0 {
			existingMachines := &machinev1alpha1.MachineList{}
			if err := a.reader.List(ctx, existingMachines, client.InNamespace(worker.Namespace)); err != nil {
				return retryutils.SevereError(err)
			}
			countMachines = len(existingMachines.Items)
			msg += fmt.Sprintf("%d machines, ", countMachines)
		}

		// Check whether all machine sets have been deleted.
		if countMachineSets != 0 {
			existingMachineSets := &machinev1alpha1.MachineSetList{}
			if err := a.reader.List(ctx, existingMachineSets, client.InNamespace(worker.Namespace)); err != nil {
				return retryutils.SevereError(err)
			}
			countMachineSets = len(existingMachineSets.Items)
			msg += fmt.Sprintf("%d machine sets, ", countMachineSets)
		}

		// Check whether all machine deployments have been deleted.
		if countMachineDeployments != 0 {
			existingMachineDeployments := &machinev1alpha1.MachineDeploymentList{}
			if err := a.reader.List(ctx, existingMachineDeployments, client.InNamespace(worker.Namespace)); err != nil {
				return retryutils.SevereError(err)
			}
			countMachineDeployments = len(existingMachineDeployments.Items)
			msg += fmt.Sprintf("%d machine deployments, ", countMachineDeployments)

			// Check whether an operation failed during the deletion process.
			for _, existingMachineDeployment := range existingMachineDeployments.Items {
				for _, failedMachine := range existingMachineDeployment.Status.FailedMachines {
					return retryutils.SevereError(fmt.Errorf("machine %s failed: %s", failedMachine.Name, failedMachine.LastOperation.Description))
				}
			}
		}

		// Check whether all machine classes have been deleted.
		if countMachineClasses != 0 {
			machineClassList := workerDelegate.MachineClassList()
			if err := a.reader.List(ctx, machineClassList, client.InNamespace(worker.Namespace)); err != nil {
				return retryutils.SevereError(err)
			}
			machineClasses, err := meta.ExtractList(machineClassList)
			if err != nil {
				return retryutils.SevereError(err)
			}
			countMachineClasses = len(machineClasses)
			msg += fmt.Sprintf("%d machine classes, ", countMachineClasses)
		}

		// Check whether all machine class secrets have been deleted.
		if countMachineClassSecrets != 0 {
			count := 0
			existingMachineClassSecrets, err := a.listMachineClassSecrets(ctx, worker.Namespace)
			if err != nil {
				return retryutils.SevereError(err)
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
			msg := fmt.Sprintf("Waiting until the following machine resources have been deleted: %s", strings.TrimSuffix(msg, ", "))
			logger.Info(msg)
			return retryutils.MinorError(errors.New(msg))
		}

		return retryutils.Ok()
	})
}

func (a *genericActuator) waitUntilCredentialsSecretAcquiredOrReleased(ctx context.Context, acquired bool, worker *extensionsv1alpha1.Worker, workerDelegate WorkerDelegate) error {
	acquiredOrReleased := false
	return retryutils.UntilTimeout(ctx, 5*time.Second, 5*time.Minute, func(ctx context.Context) (bool, error) {
		// Check whether the finalizer of the machine class credentials secret has been added or removed.
		// This check is only applicable when the given workerDelegate does not implement the
		// deprecated WorkerCredentialsDelegate interface, i.e. machine classes reference a separate
		// Secret for cloud provider credentials.
		if !acquiredOrReleased {
			_, ok := workerDelegate.(WorkerCredentialsDelegate)
			if ok {
				acquiredOrReleased = true
			} else {
				secret, err := kutil.GetSecretByReference(ctx, a.client, &worker.Spec.SecretRef)
				if err != nil {
					return retryutils.SevereError(fmt.Errorf("could not get the secret referenced by worker: %+v", err))
				}

				// We need to check for both mcmFinalizer and mcmProviderFinalizer:
				// - mcmFinalizer is the finalizer used by machine controller manager and its in-tree providers
				// - mcmProviderFinalizer is the finalizer used by out-of-tree machine controller providers
				if (controllerutil.ContainsFinalizer(secret, mcmFinalizer) || controllerutil.ContainsFinalizer(secret, mcmProviderFinalizer)) == acquired {
					acquiredOrReleased = true
				}
			}
		}

		if !acquiredOrReleased {
			return retryutils.MinorError(errors.New("machine class credentials secret has not yet been acquired or released"))
		}
		return retryutils.Ok()
	})
}
