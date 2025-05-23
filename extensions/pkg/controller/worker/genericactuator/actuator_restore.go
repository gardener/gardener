// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package genericactuator

import (
	"context"
	"encoding/json"
	"fmt"

	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	extensionsworkercontroller "github.com/gardener/gardener/extensions/pkg/controller/worker"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/utils/gardener/shootstate"
)

// RestoreWithoutReconcile restores the worker state without calling 'Reconcile'.
func RestoreWithoutReconcile(
	ctx context.Context,
	log logr.Logger,
	gardenReader client.Reader,
	seedClient client.Client,
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
	if err := seedClient.List(ctx, existingMachineDeployments, client.InNamespace(worker.Namespace)); err != nil {
		return err
	}

	// Parse the worker state to a separate machineDeployment states and attach them to
	// the corresponding machineDeployments which are to be deployed later
	log.Info("Extracting machine state")
	if err := addStateToMachineDeployment(ctx, log, gardenReader, cluster.Shoot, wantedMachineDeployments); err != nil {
		return err
	}

	wantedMachineDeployments = removeWantedDeploymentWithoutState(wantedMachineDeployments)

	// Deploy generated machine classes.
	if err := workerDelegate.DeployMachineClasses(ctx); err != nil {
		return fmt.Errorf("failed to deploy the machine classes: %w", err)
	}

	// Do the actual restoration
	if err := restoreMachineSetsAndMachines(ctx, log, seedClient, wantedMachineDeployments); err != nil {
		return fmt.Errorf("failed restoration of the machineSet and the machines: %w", err)
	}

	// Generate machine deployment configuration based on previously computed list of deployments and deploy them.
	if err := deployMachineDeployments(ctx, log, seedClient, cluster, worker, existingMachineDeployments, wantedMachineDeployments, true); err != nil {
		return fmt.Errorf("failed to restore the machine deployment config: %w", err)
	}

	// Scale the machine-controller-manager to 1 now that all resources have been restored.
	if !extensionscontroller.IsHibernated(cluster) {
		if err := scaleMachineControllerManager(ctx, log, seedClient, worker, 1); err != nil {
			return fmt.Errorf("failed to scale up machine-controller-manager: %w", err)
		}
	}

	return nil
}

// Restore uses the Worker's spec to figure out the wanted MachineDeployments. Then it parses the Worker's state.
// If there is a record in the state corresponding to a wanted deployment then the Restore function
// deploys that MachineDeployment with all related MachineSet and Machines. It finally calls the 'Reconcile' function.
func (a *genericActuator) Restore(ctx context.Context, log logr.Logger, worker *extensionsv1alpha1.Worker, cluster *extensionscontroller.Cluster) error {
	if err := RestoreWithoutReconcile(ctx, log, a.gardenReader, a.seedClient, a.delegateFactory, worker, cluster); err != nil {
		return err
	}
	return a.Reconcile(ctx, log, worker, cluster)
}

func addStateToMachineDeployment(
	ctx context.Context,
	log logr.Logger,
	gardenReader client.Reader,
	shoot *gardencorev1beta1.Shoot,
	wantedMachineDeployments extensionsworkercontroller.MachineDeployments,
) error {
	var state []byte

	// We use the `gardenReader` here to prevent controller-runtime from trying to cache/list the ShootStates.
	shootState := &gardencorev1beta1.ShootState{ObjectMeta: metav1.ObjectMeta{Name: shoot.Name, Namespace: shoot.Namespace}}
	if err := gardenReader.Get(ctx, client.ObjectKeyFromObject(shootState), shootState); err != nil {
		return err
	}

	gardenerData := v1beta1helper.GardenerResourceDataList(shootState.Spec.Gardener)
	if machineState := gardenerData.Get(v1beta1constants.DataTypeMachineState); machineState != nil && machineState.Type == v1beta1constants.DataTypeMachineState {
		log.Info("Fetching machine state from ShootState succeeded", "shootState", client.ObjectKeyFromObject(shootState))
		var err error
		state, err = shootstate.DecompressMachineState(machineState.Data.Raw)
		if err != nil {
			return err
		}
	}

	if len(state) == 0 {
		log.Info("Machine state is empty, no state to add")
		return nil
	}

	// Parse the worker state to MachineDeploymentStates
	machineState := &shootstate.MachineState{MachineDeployments: make(map[string]*shootstate.MachineDeploymentState)}
	if err := json.Unmarshal(state, &machineState); err != nil {
		return err
	}

	// Attach the parsed MachineDeploymentStates to the wanted MachineDeployments
	for index, wantedMachineDeployment := range wantedMachineDeployments {
		wantedMachineDeployments[index].State = machineState.MachineDeployments[wantedMachineDeployment.Name]
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
