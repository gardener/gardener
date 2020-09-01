// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"encoding/json"
	"time"

	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	workercontroller "github.com/gardener/gardener/extensions/pkg/controller/worker"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	gardeneretry "github.com/gardener/gardener/pkg/utils/retry"
)

// Restore uses the Worker's spec to figure out the wanted MachineDeployments. Then it parses the Worker's state.
// If there is a record in the state corresponding to a wanted deployment then the Restore function
// deploys that MachineDeployment with all related MachineSet and Machines.
func (a *genericActuator) Restore(ctx context.Context, worker *extensionsv1alpha1.Worker, cluster *extensionscontroller.Cluster) error {
	logger := a.logger.WithValues("worker", kutil.KeyFromObject(worker), "operation", "restore")

	workerDelegate, err := a.delegateFactory.WorkerDelegate(ctx, worker, cluster)
	if err != nil {
		return errors.Wrap(err, "could not instantiate actuator context")
	}

	// Generate the desired machine deployments.
	logger.Info("Generating machine deployments")
	wantedMachineDeployments, err := workerDelegate.GenerateMachineDeployments(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to generate the machine deployments")
	}

	// Get the list of all existing machine deployments.
	existingMachineDeployments := &machinev1alpha1.MachineDeploymentList{}
	if err := a.client.List(ctx, existingMachineDeployments, client.InNamespace(worker.Namespace)); err != nil {
		return err
	}

	// Parse the worker state to a separate machineDeployment states and attach them to
	// the corresponding machineDeployments which are to be deployed later
	logger.Info("Extracting state from worker status")
	if err := a.addStateToMachineDeployment(worker, wantedMachineDeployments); err != nil {
		return err
	}

	wantedMachineDeployments = removeWantedDeploymentWithoutState(wantedMachineDeployments)

	// Delete the machine-controller-manager. During restoration MCM must not exist
	if err := a.deleteMachineControllerManager(ctx, logger, worker); err != nil {
		return errors.Wrap(err, "failed deleting machine-controller-manager")
	}

	if err := a.waitUntilMachineControllerManagerIsDeleted(ctx, logger, worker.Namespace); err != nil {
		return errors.Wrap(err, "failed deleting machine-controller-manager")
	}

	// Do the actual restoration
	if err := a.deployMachineSetsAndMachines(ctx, logger, wantedMachineDeployments); err != nil {
		return errors.Wrap(err, "failed restoration of the machineSet and the machines")
	}

	// Generate machine deployment configuration based on previously computed list of deployments and deploy them.
	if err := a.deployMachineDeployments(ctx, logger, cluster, worker, existingMachineDeployments, wantedMachineDeployments, workerDelegate.MachineClassKind(), true); err != nil {
		return errors.Wrap(err, "failed to restore the machine deployment config")
	}

	return nil
}

func (a *genericActuator) addStateToMachineDeployment(worker *extensionsv1alpha1.Worker, wantedMachineDeployments workercontroller.MachineDeployments) error {
	if worker.Status.State == nil || len(worker.Status.State.Raw) <= 0 {
		return nil
	}

	// Parse the worker state to MachineDeploymentStates
	workerState := &workercontroller.State{
		MachineDeployments: make(map[string]*workercontroller.MachineDeploymentState),
	}

	if err := json.Unmarshal(worker.Status.State.Raw, &workerState); err != nil {
		return err
	}

	// Attach the parsed MachineDeploymentStates to the wanted MachineDeployments
	for index, wantedMachineDeployment := range wantedMachineDeployments {
		wantedMachineDeployments[index].State = workerState.MachineDeployments[wantedMachineDeployment.Name]
	}

	return nil
}

func (a *genericActuator) deployMachineSetsAndMachines(ctx context.Context, logger logr.Logger, wantedMachineDeployments workercontroller.MachineDeployments) error {
	logger.Info("Deploying Machines and MachineSets")
	for _, wantedMachineDeployment := range wantedMachineDeployments {
		machineSets := wantedMachineDeployment.State.MachineSets

		for _, machineSet := range machineSets {
			// Create the MachineSet if not already exists. We do not care about the MachineSet status
			// because the MCM will update it
			if err := a.client.Create(ctx, &machineSet); err != nil && !apierrors.IsAlreadyExists(err) {
				return err
			}
		}

		// Deploy each machine owned by the MachineSet which was restored above
		for _, machine := range wantedMachineDeployment.State.Machines {
			// Create the machine if it not exists already
			err := a.client.Create(ctx, &machine)
			if err != nil && !apierrors.IsAlreadyExists(err) {
				return err
			}

			// Attach the Shoot node to the Machine status
			node := machine.Status.Node
			if err := a.waitUntilStatusIsUpdates(ctx, &machine, func() error {
				machine.Status.Node = node
				return nil
			}); err != nil {
				return err
			}
		}
	}

	return nil
}

func (a *genericActuator) waitUntilStatusIsUpdates(ctx context.Context, obj runtime.Object, transform func() error) error {
	return gardeneretry.Until(ctx, 5*time.Second, func(ctx context.Context) (done bool, err error) {
		if err := extensionscontroller.TryUpdateStatus(ctx, retry.DefaultBackoff, a.client, obj, transform); err != nil {
			if apierrors.IsNotFound(err) {
				return gardeneretry.NotOk()
			}
			return gardeneretry.SevereError(err)
		}
		return gardeneretry.Ok()
	})
}

func removeWantedDeploymentWithoutState(wantedMachineDeployments workercontroller.MachineDeployments) workercontroller.MachineDeployments {
	if wantedMachineDeployments == nil {
		return nil
	}

	reducedMachineDeployments := make(workercontroller.MachineDeployments, 0)
	for _, wantedMachineDeployment := range wantedMachineDeployments {
		if wantedMachineDeployment.State != nil && len(wantedMachineDeployment.State.MachineSets) > 0 {
			reducedMachineDeployments = append(reducedMachineDeployments, wantedMachineDeployment)
		}
	}
	return reducedMachineDeployments
}
