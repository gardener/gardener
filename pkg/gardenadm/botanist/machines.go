// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"
	"fmt"
	"slices"

	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	corev1 "k8s.io/api/core/v1"
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

// addressTypePreference when picking a node/machine address. Higher value means higher priority.
// Unknown address types have the lowest priority (0).
var addressTypePreference = map[corev1.NodeAddressType]int{
	// internal names have priority: they are reachable from within the cluster network even when nodes have no
	// externally routable addresses (and for SSH we jump via a bastion host)
	corev1.NodeInternalIP:  5,
	corev1.NodeInternalDNS: 4,
	corev1.NodeExternalIP:  3,
	corev1.NodeExternalDNS: 2,
	// this should be returned by all providers, and is actually locally resolvable in many infrastructures
	corev1.NodeHostName: 1,
}

// PreferredNodeAddress returns the preferred address of the given node/machine addresses based on
// addressTypePreference (prefer internal and IP to external and DNS). Returns an error if no addresses are given.
func PreferredNodeAddress(addresses []corev1.NodeAddress) (string, error) {
	if len(addresses) == 0 {
		return "", fmt.Errorf("no addresses found in status")
	}

	address := slices.MaxFunc(addresses, func(a, b corev1.NodeAddress) int {
		return addressTypePreference[a.Type] - addressTypePreference[b.Type]
	})

	return address.Address, nil
}
