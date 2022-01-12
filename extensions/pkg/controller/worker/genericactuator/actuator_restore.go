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
	"fmt"

	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	workercontroller "github.com/gardener/gardener/extensions/pkg/controller/worker"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
)

// Restore uses the Worker's spec to figure out the wanted MachineDeployments. Then it parses the Worker's state.
// If there is a record in the state corresponding to a wanted deployment then the Restore function
// deploys that MachineDeployment with all related MachineSet and Machines.
func (a *genericActuator) Restore(ctx context.Context, worker *extensionsv1alpha1.Worker, cluster *extensionscontroller.Cluster) error {
	logger := a.logger.WithValues("worker", client.ObjectKeyFromObject(worker), "operation", "restore")

	workerDelegate, err := a.delegateFactory.WorkerDelegate(ctx, worker, cluster)
	if err != nil {
		return fmt.Errorf("could not instantiate actuator context: %w", err)
	}

	// Generate the desired machine deployments.
	logger.Info("Generating machine deployments")
	wantedMachineDeployments, err := workerDelegate.GenerateMachineDeployments(ctx)
	if err != nil {
		return fmt.Errorf("failed to generate the machine deployments: %w", err)
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

	// Scale the machine-controller-manager to 0. During restoration MCM must not be working
	if err := a.scaleMachineControllerManager(ctx, logger, worker, 0); err != nil {
		return fmt.Errorf("failed scale down machine-controller-manager: %w", err)
	}

	// Deploy generated machine classes.
	if err := workerDelegate.DeployMachineClasses(ctx); err != nil {
		return fmt.Errorf("failed to deploy the machine classes: %w", err)
	}

	if err := kubernetes.WaitUntilDeploymentScaledToDesiredReplicas(ctx, a.client, kutil.Key(worker.Namespace, McmDeploymentName), 0); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("deadline exceeded while scaling down machine-controller-manager: %w", err)
	}

	// Do the actual restoration
	if err := a.restoreMachineSetsAndMachines(ctx, logger, wantedMachineDeployments); err != nil {
		return fmt.Errorf("failed restoration of the machineSet and the machines: %w", err)
	}

	// Generate machine deployment configuration based on previously computed list of deployments and deploy them.
	if err := a.deployMachineDeployments(ctx, logger, cluster, worker, existingMachineDeployments, wantedMachineDeployments, workerDelegate.MachineClassKind(), true); err != nil {
		return fmt.Errorf("failed to restore the machine deployment config: %w", err)
	}

	// Finally reconcile the worker so that the machine-controller-manager gets scaled up and OwnerReferences between
	// machinedeployments, machinesets and machines are added properly.
	return a.Reconcile(ctx, worker, cluster)
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

func (a *genericActuator) restoreMachineSetsAndMachines(ctx context.Context, logger logr.Logger, wantedMachineDeployments workercontroller.MachineDeployments) error {
	logger.Info("Deploying Machines and MachineSets")
	for _, wantedMachineDeployment := range wantedMachineDeployments {
		for _, machineSet := range wantedMachineDeployment.State.MachineSets {
			if err := a.client.Create(ctx, &machineSet); kutil.IgnoreAlreadyExists(err) != nil {
				return err
			}
		}

		for _, machine := range wantedMachineDeployment.State.Machines {
			newMachine := (&machine).DeepCopy()
			newMachine.Status = machinev1alpha1.MachineStatus{}
			if err := a.client.Create(ctx, newMachine); err != nil {
				if !apierrors.IsAlreadyExists(err) {
					return err
				}

				// machine already exists, get the current object and update the status
				if err := a.client.Get(ctx, client.ObjectKeyFromObject(newMachine), newMachine); err != nil {
					return err
				}
			}

			newMachine.Status = machine.Status
			return a.client.Status().Update(ctx, newMachine)
		}
	}

	return nil
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
