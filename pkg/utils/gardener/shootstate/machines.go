// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package shootstate

import (
	"bytes"
	"cmp"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"slices"

	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

// MachineDeploymentState stores the last versions of the machine sets and machines to which the machine deployment
// corresponds.
type MachineDeploymentState struct {
	Replicas    int32                        `json:"replicas,omitempty"`
	MachineSets []machinev1alpha1.MachineSet `json:"machineSets,omitempty"`
	Machines    []machinev1alpha1.Machine    `json:"machines,omitempty"`
}

// MachineState represent the last known state of the machines.
type MachineState struct {
	MachineDeployments map[string]*MachineDeploymentState `json:"machineDeployments,omitempty"`
}

func computeMachineState(ctx context.Context, seedClient client.Client, namespace, shootName string) (*MachineState, error) {
	state := &MachineState{MachineDeployments: make(map[string]*MachineDeploymentState)}

	machineDeployments := &machinev1alpha1.MachineDeploymentList{}
	if err := seedClient.List(ctx, machineDeployments, client.InNamespace(namespace)); err != nil {
		return nil, err
	}

	// If a provider extensions uses a version < v1.82 of the extensions library, then the Worker controller will have
	// persisted the machine state to the `Worker`'s `.status.state` field and already shallow-deleted all machine
	// objects before below code gets executed (i.e., before gardenlet can compute the machine state itself).
	// In this case, let's get the data from the `Worker`'s `.status.state` field instead of trying to (re-)compute it
	// (which will fail anyways).
	// TODO(rfranzke): Remove this block after Gardener v1.86 has been released.
	if len(machineDeployments.Items) == 0 {
		worker := &extensionsv1alpha1.Worker{ObjectMeta: metav1.ObjectMeta{Name: shootName, Namespace: namespace}}
		if err := seedClient.Get(ctx, client.ObjectKeyFromObject(worker), worker); client.IgnoreNotFound(err) != nil {
			return nil, fmt.Errorf("failed reading Worker object %s: %w", client.ObjectKeyFromObject(worker), err)
		}

		if worker.Status.State != nil && len(worker.Status.State.Raw) > 0 {
			if err := json.Unmarshal(worker.Status.State.Raw, &state); err != nil {
				return nil, fmt.Errorf("failed unmarshaling machine state from Worker's '.status.state' field: %w", err)
			}
			return state, nil
		}
	}

	machineDeploymentToMachineSets, err := getMachineDeploymentToMachineSetsMap(ctx, seedClient, namespace)
	if err != nil {
		return nil, err
	}

	machineSetToMachines, err := getMachineSetToMachinesMap(ctx, seedClient, namespace)
	if err != nil {
		return nil, err
	}

	var allMachines []machinev1alpha1.Machine
	for _, machineDeployment := range machineDeployments.Items {
		machineSets, ok := machineDeploymentToMachineSets[machineDeployment.Name]
		if !ok {
			continue
		}

		for i, machineSet := range machineSets {
			// remove irrelevant data from the machine set
			machineSets[i].ObjectMeta = metav1.ObjectMeta{
				Name:        machineSet.Name,
				Namespace:   machineSet.Namespace,
				Annotations: machineSet.Annotations,
				Labels:      machineSet.Labels,
			}
			machineSets[i].Status = machinev1alpha1.MachineSetStatus{}

			// fetch machines related to the machine set/deployment
			machines := append(machineSetToMachines[machineSet.Name], machineSetToMachines[machineDeployment.Name]...)
			if len(machines) == 0 {
				continue
			}

			for j, machine := range machines {
				// remove irrelevant data from the machine
				machines[j].ObjectMeta = metav1.ObjectMeta{
					Name:        machine.Name,
					Namespace:   machine.Namespace,
					Annotations: machine.Annotations,
					Labels:      machine.Labels,
				}
				machines[j].Status = machinev1alpha1.MachineStatus{}
			}

			allMachines = append(allMachines, machines...)
		}

		state.MachineDeployments[machineDeployment.Name] = &MachineDeploymentState{
			Replicas:    machineDeployment.Spec.Replicas,
			MachineSets: machineSets,
			Machines:    allMachines,
		}
	}

	return state, nil
}

func getMachineDeploymentToMachineSetsMap(ctx context.Context, c client.Client, namespace string) (map[string][]machinev1alpha1.MachineSet, error) {
	existingMachineSets := &machinev1alpha1.MachineSetList{}
	if err := c.List(ctx, existingMachineSets, client.InNamespace(namespace)); err != nil {
		return nil, err
	}

	// When we read from the cache we get unsorted results, hence, we sort to prevent unnecessary state updates from
	// happening.
	slices.SortFunc(existingMachineSets.Items, func(a, b machinev1alpha1.MachineSet) int {
		return cmp.Compare(a.Name, b.Name)
	})

	return gardenerutils.BuildOwnerToMachineSetsMap(existingMachineSets.Items), nil
}

func getMachineSetToMachinesMap(ctx context.Context, seedClient client.Client, namespace string) (map[string][]machinev1alpha1.Machine, error) {
	existingMachines := &machinev1alpha1.MachineList{}
	if err := seedClient.List(ctx, existingMachines, client.InNamespace(namespace)); err != nil {
		return nil, err
	}

	// We temporarily filter out machines without provider ID or node label (VMs which got created but not yet joined
	// the cluster) to prevent unnecessarily persisting them in the Worker state.
	// TODO: Remove this again once machine-controller-manager supports backing off creation/deletion of failed
	//  machines, see https://github.com/gardener/machine-controller-manager/issues/483.
	var filteredMachines []machinev1alpha1.Machine
	for _, machine := range existingMachines.Items {
		if _, ok := machine.Labels["node"]; ok || machine.Spec.ProviderID != "" {
			filteredMachines = append(filteredMachines, machine)
		}
	}

	// When we read from the cache we get unsorted results, hence, we sort to prevent unnecessary state updates from
	// happening.
	slices.SortFunc(filteredMachines, func(a, b machinev1alpha1.Machine) int {
		return cmp.Compare(a.Name, b.Name)
	})

	return gardenerutils.BuildOwnerToMachinesMap(filteredMachines), nil
}

type compressedMachineState struct {
	State []byte `json:"state"`
}

func compressMachineState(state []byte) ([]byte, error) {
	if len(state) == 0 || string(state) == "{}" {
		return nil, nil
	}

	var stateCompressed bytes.Buffer
	gzipWriter, err := gzip.NewWriterLevel(&stateCompressed, gzip.BestCompression)
	if err != nil {
		return nil, fmt.Errorf("failed creating gzip writer for compressing machine state data: %w", err)
	}
	defer gzipWriter.Close()

	if _, err := gzipWriter.Write(state); err != nil {
		return nil, fmt.Errorf("failed writing machine state data for compression: %w", err)
	}

	// Close ensures any unwritten data is flushed and the gzip footer is written. Without this, the `stateCompressed`
	// buffer would not contain any data. Hence, we have to call it explicitly here after writing, in addition to the
	// 'defer' call above.
	if err := gzipWriter.Close(); err != nil {
		return nil, fmt.Errorf("failed closing the gzip writer after compressing the machine state data: %w", err)
	}

	return json.Marshal(&compressedMachineState{State: stateCompressed.Bytes()})
}

// DecompressMachineState decompresses the machine state data.
func DecompressMachineState(stateCompressed []byte) ([]byte, error) {
	if len(stateCompressed) == 0 {
		return nil, nil
	}

	var machineState compressedMachineState
	if err := json.Unmarshal(stateCompressed, &machineState); err != nil {
		return nil, fmt.Errorf("failed unmarshalling JSON to compressed machine state structure: %w", err)
	}

	gzipReader, err := gzip.NewReader(bytes.NewReader(machineState.State))
	if err != nil {
		return nil, fmt.Errorf("failed creating gzip reader for decompressing machine state data: %w", err)
	}
	defer gzipReader.Close()

	var state bytes.Buffer
	if _, err := state.ReadFrom(gzipReader); err != nil {
		return nil, fmt.Errorf("failed reading machine state data for decompression: %w", err)
	}

	return state.Bytes(), nil
}
