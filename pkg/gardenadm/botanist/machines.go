// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"
	"fmt"

	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ListControlPlaneMachines stores all control plane machines in controlPlaneMachines for later retrieval.
// Listing the machines only once ensures consistent ordering when accessing them by index.
func (b *GardenadmBotanist) ListControlPlaneMachines(ctx context.Context) error {
	machineList := &machinev1alpha1.MachineList{}
	if err := b.SeedClientSet.Client().List(ctx, machineList, client.InNamespace(b.Shoot.ControlPlaneNamespace)); err != nil {
		return fmt.Errorf("failed to list machines: %w", err)
	}
	b.controlPlaneMachines = machineList.Items
	return nil
}

// GetMachineByIndex returns the control plane machine with the given index or an error if the index is out of bounds.
func (b *GardenadmBotanist) GetMachineByIndex(index int) (*machinev1alpha1.Machine, error) {
	if index < 0 {
		return nil, fmt.Errorf("machine index must be non-negative, got %d", index)
	}
	if index >= len(b.controlPlaneMachines) {
		return nil, fmt.Errorf("only %d machines found, but wanted machine with index %d", len(b.controlPlaneMachines), index)
	}
	return &b.controlPlaneMachines[index], nil
}
