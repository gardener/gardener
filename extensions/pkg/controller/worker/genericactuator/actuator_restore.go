// Copyright 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	extensionsworkercontroller "github.com/gardener/gardener/extensions/pkg/controller/worker"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

// RestoreWithoutReconcile restores the worker state without calling 'Reconcile'.
func RestoreWithoutReconcile(
	ctx context.Context,
	log logr.Logger,
	cl client.Client,
	delegateFactory DelegateFactory,
	worker *extensionsv1alpha1.Worker,
	cluster *extensionscontroller.Cluster,
) error {
	log = log.WithValues("operation", "restore")

	workerDelegate, err := delegateFactory.WorkerDelegate(ctx, worker, cluster)
	if err != nil {
		return fmt.Errorf("could not instantiate actuator context: %w", err)
	}

	// Generate the desired machine deployments.
	log.Info("Generating machine deployments")
	wantedMachineDeployments, err := workerDelegate.GenerateMachineDeployments(ctx)
	if err != nil {
		return fmt.Errorf("failed to generate the machine deployments: %w", err)
	}

	// Get the list of all existing machine deployments.
	existingMachineDeployments := &machinev1alpha1.MachineDeploymentList{}
	if err := cl.List(ctx, existingMachineDeployments, client.InNamespace(worker.Namespace)); err != nil {
		return err
	}

	// Parse the worker state to a separate machineDeployment states and attach them to
	// the corresponding machineDeployments which are to be deployed later
	log.Info("Extracting state from worker status")
	if err := addStateToMachineDeployment(worker, wantedMachineDeployments); err != nil {
		return err
	}

	wantedMachineDeployments = removeWantedDeploymentWithoutState(wantedMachineDeployments)

	// Scale the machine-controller-manager to 0. During restoration MCM must not be working
	if err := scaleMachineControllerManager(ctx, log, cl, worker, 0); err != nil {
		return fmt.Errorf("failed to scale down machine-controller-manager: %w", err)
	}

	// Deploy generated machine classes.
	if err := workerDelegate.DeployMachineClasses(ctx); err != nil {
		return fmt.Errorf("failed to deploy the machine classes: %w", err)
	}

	if err := kubernetes.WaitUntilDeploymentScaledToDesiredReplicas(ctx, cl, kubernetesutils.Key(worker.Namespace, McmDeploymentName), 0); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("deadline exceeded while scaling down machine-controller-manager: %w", err)
	}

	// Do the actual restoration
	if err := restoreMachineSetsAndMachines(ctx, log, cl, wantedMachineDeployments); err != nil {
		return fmt.Errorf("failed restoration of the machineSet and the machines: %w", err)
	}

	// Generate machine deployment configuration based on previously computed list of deployments and deploy them.
	if err := deployMachineDeployments(ctx, log, cl, cluster, worker, existingMachineDeployments, wantedMachineDeployments, workerDelegate.MachineClassKind(), true); err != nil {
		return fmt.Errorf("failed to restore the machine deployment config: %w", err)
	}

	// Scale the machine-controller-manager to 1 now that all resources have been restored.
	if !extensionscontroller.IsHibernated(cluster) {
		if err := scaleMachineControllerManager(ctx, log, cl, worker, 1); err != nil {
			return fmt.Errorf("failed to scale up machine-controller-manager: %w", err)
		}
	}

	return nil
}

// Restore uses the Worker's spec to figure out the wanted MachineDeployments. Then it parses the Worker's state.
// If there is a record in the state corresponding to a wanted deployment then the Restore function
// deploys that MachineDeployment with all related MachineSet and Machines. It finally calls the 'Reconcile' function.
func (a *genericActuator) Restore(ctx context.Context, log logr.Logger, worker *extensionsv1alpha1.Worker, cluster *extensionscontroller.Cluster) error {
	if err := RestoreWithoutReconcile(ctx, log, a.client, a.delegateFactory, worker, cluster); err != nil {
		return err
	}
	return a.Reconcile(ctx, log, worker, cluster)
}

func addStateToMachineDeployment(worker *extensionsv1alpha1.Worker, wantedMachineDeployments extensionsworkercontroller.MachineDeployments) error {
	if worker.Status.State == nil || len(worker.Status.State.Raw) <= 0 {
		return nil
	}

	// Parse the worker state to MachineDeploymentStates
	workerState := &extensionsworkercontroller.State{
		MachineDeployments: make(map[string]*extensionsworkercontroller.MachineDeploymentState),
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

func restoreMachineSetsAndMachines(ctx context.Context, log logr.Logger, cl client.Client, wantedMachineDeployments extensionsworkercontroller.MachineDeployments) error {
	log.Info("Deploying Machines and MachineSets")
	for _, wantedMachineDeployment := range wantedMachineDeployments {
		for _, machineSet := range wantedMachineDeployment.State.MachineSets {
			if err := cl.Create(ctx, &machineSet); client.IgnoreAlreadyExists(err) != nil {
				return err
			}
		}

		for _, machine := range wantedMachineDeployment.State.Machines {
			if err := cl.Create(ctx, &machine); err != nil {
				if !apierrors.IsAlreadyExists(err) {
					return err
				}
			}
		}
	}

	return nil
}

func removeWantedDeploymentWithoutState(wantedMachineDeployments extensionsworkercontroller.MachineDeployments) extensionsworkercontroller.MachineDeployments {
	if wantedMachineDeployments == nil {
		return nil
	}

	reducedMachineDeployments := make(extensionsworkercontroller.MachineDeployments, 0)
	for _, wantedMachineDeployment := range wantedMachineDeployments {
		if wantedMachineDeployment.State != nil && len(wantedMachineDeployment.State.MachineSets) > 0 {
			reducedMachineDeployments = append(reducedMachineDeployments, wantedMachineDeployment)
		}
	}
	return reducedMachineDeployments
}
