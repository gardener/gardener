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

package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sort"

	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	extensionsworkerhelper "github.com/gardener/gardener/extensions/pkg/controller/worker/helper"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
)

// TODO(rfranzke): Drop this stateReconciler after a few releases as soon as the shoot migrate flow persists the Shoot
//  state only after all extension resources have been migrated.

type stateReconciler struct {
	client client.Client
}

// NewStateReconciler creates a new reconcile.Reconciler that reconciles
// Worker's State resources of Gardener's `extensions.gardener.cloud` API group.
func NewStateReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &stateReconciler{client: mgr.GetClient()}
}

func (r *stateReconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	worker := &extensionsv1alpha1.Worker{}
	if err := r.client.Get(ctx, request.NamespacedName, worker); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	if worker.DeletionTimestamp != nil {
		return reconcile.Result{}, nil
	}

	if v1beta1helper.ComputeOperationType(worker.ObjectMeta, worker.Status.LastOperation) != gardencorev1beta1.LastOperationTypeReconcile {
		return reconcile.Result{Requeue: true}, nil
	}

	if isWorkerMigrated(worker) {
		return reconcile.Result{}, nil
	}

	return reconcile.Result{}, PersistState(ctx, log, r.client, worker)
}

// PersistState persists the worker state into the .status.state field.
func PersistState(ctx context.Context, log logr.Logger, c client.Client, worker *extensionsv1alpha1.Worker) error {
	state, err := computeState(ctx, c, worker.Namespace)
	if err != nil {
		return err
	}

	rawState, err := json.Marshal(state)
	if err != nil {
		return err
	}

	// If the state did not change, do not even try to send an empty PATCH request.
	if worker.Status.State != nil && bytes.Equal(rawState, worker.Status.State.Raw) {
		return nil
	}

	patch := client.MergeFromWithOptions(worker.DeepCopy(), client.MergeFromWithOptimisticLock{})
	worker.Status.State = &runtime.RawExtension{Raw: rawState}
	if err := c.Status().Patch(ctx, worker, patch); err != nil {
		return fmt.Errorf("error updating Worker state: %w", err)
	}

	log.Info("Successfully updated Worker state")
	return nil
}

func computeState(ctx context.Context, c client.Client, namespace string) (*State, error) {
	existingMachineDeployments := &machinev1alpha1.MachineDeploymentList{}
	if err := c.List(ctx, existingMachineDeployments, client.InNamespace(namespace)); err != nil {
		return nil, err
	}

	machineSets, err := getExistingMachineSetsMap(ctx, c, namespace)
	if err != nil {
		return nil, err
	}

	machines, err := getExistingMachinesMap(ctx, c, namespace)
	if err != nil {
		return nil, err
	}

	workerState := &State{MachineDeployments: make(map[string]*MachineDeploymentState)}

	for _, deployment := range existingMachineDeployments.Items {
		machineDeploymentState := &MachineDeploymentState{}
		machineDeploymentState.Replicas = deployment.Spec.Replicas

		machineDeploymentMachineSets, ok := machineSets[deployment.Name]
		if !ok {
			continue
		}

		addMachineSetToMachineDeploymentState(machineDeploymentMachineSets, machineDeploymentState)

		for _, machineSet := range machineDeploymentMachineSets {
			currentMachines := append(machines[machineSet.Name], machines[deployment.Name]...)
			if len(currentMachines) <= 0 {
				continue
			}

			for index := range currentMachines {
				addMachineToMachineDeploymentState(&currentMachines[index], machineDeploymentState)
			}
		}

		workerState.MachineDeployments[deployment.Name] = machineDeploymentState
	}

	return workerState, nil
}

// getExistingMachineSetsMap returns a map of existing MachineSets as values and their owners as keys
func getExistingMachineSetsMap(ctx context.Context, c client.Client, namespace string) (map[string][]machinev1alpha1.MachineSet, error) {
	existingMachineSets := &machinev1alpha1.MachineSetList{}
	if err := c.List(ctx, existingMachineSets, client.InNamespace(namespace)); err != nil {
		return nil, err
	}

	// When we read from the cache we get unsorted results, hence, we sort to prevent unnecessary state updates from happening.
	sort.Slice(existingMachineSets.Items, func(i, j int) bool { return existingMachineSets.Items[i].Name < existingMachineSets.Items[j].Name })

	return extensionsworkerhelper.BuildOwnerToMachineSetsMap(existingMachineSets.Items), nil
}

// getExistingMachinesMap returns a map of the existing Machines as values and the name of their owner
// no matter of being machineSet or MachineDeployment. If a Machine has a ownerReference the key(owner)
// will be the MachineSet if not the key will be the name of the MachineDeployment which is stored as
// a label. We assume that there is no MachineDeployment and MachineSet with the same names.
func getExistingMachinesMap(ctx context.Context, c client.Client, namespace string) (map[string][]machinev1alpha1.Machine, error) {
	existingMachines := &machinev1alpha1.MachineList{}
	if err := c.List(ctx, existingMachines, client.InNamespace(namespace)); err != nil {
		return nil, err
	}

	// We temporarily filter out machines without provider ID or node label (VMs which got created but not yet joined the cluster)
	// to prevent unnecessarily persisting them in the Worker state.
	// TODO: Remove this again once machine-controller-manager supports backing off creation/deletion of failed machines, see
	// https://github.com/gardener/machine-controller-manager/issues/483.
	var filteredMachines []machinev1alpha1.Machine
	for _, machine := range existingMachines.Items {
		if _, ok := machine.Labels["node"]; ok || machine.Spec.ProviderID != "" {
			filteredMachines = append(filteredMachines, machine)
		}
	}

	// When we read from the cache we get unsorted results, hence, we sort to prevent unnecessary state updates from happening.
	sort.Slice(filteredMachines, func(i, j int) bool { return filteredMachines[i].Name < filteredMachines[j].Name })

	return extensionsworkerhelper.BuildOwnerToMachinesMap(filteredMachines), nil
}

func addMachineSetToMachineDeploymentState(machineSets []machinev1alpha1.MachineSet, machineDeploymentState *MachineDeploymentState) {
	if len(machineSets) < 1 || machineDeploymentState == nil {
		return
	}

	// remove redundant data from the machine set
	for index := range machineSets {
		machineSet := &machineSets[index]
		machineSet.ObjectMeta = metav1.ObjectMeta{
			Name:        machineSet.Name,
			Namespace:   machineSet.Namespace,
			Annotations: machineSet.Annotations,
			Labels:      machineSet.Labels,
		}
		machineSet.OwnerReferences = nil
		machineSet.Status = machinev1alpha1.MachineSetStatus{}
	}

	machineDeploymentState.MachineSets = machineSets
}

func addMachineToMachineDeploymentState(machine *machinev1alpha1.Machine, machineDeploymentState *MachineDeploymentState) {
	if machine == nil || machineDeploymentState == nil {
		return
	}

	// remove redundant data from the machine
	machine.ObjectMeta = metav1.ObjectMeta{
		Name:        machine.Name,
		Namespace:   machine.Namespace,
		Annotations: machine.Annotations,
		Labels:      machine.Labels,
	}
	machine.OwnerReferences = nil
	machine.Status = machinev1alpha1.MachineStatus{}

	machineDeploymentState.Machines = append(machineDeploymentState.Machines, *machine)
}

func isWorkerMigrated(worker *extensionsv1alpha1.Worker) bool {
	return worker.Status.LastOperation != nil &&
		worker.Status.LastOperation.Type == gardencorev1beta1.LastOperationTypeMigrate &&
		worker.Status.LastOperation.State == gardencorev1beta1.LastOperationStateSucceeded
}
