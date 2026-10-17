// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package bastion

import (
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
)

// BastionMachineToCloudProfileBastion maps the machine override from an extensions.gardener.cloud Bastion to the
// CloudProfile Bastion type consumed by GetMachineSpecFromCloudProfile. It returns nil when the override
// does not specify any machine configuration, so that machine selection falls back to the CloudProfile defaults.
func BastionMachineToCloudProfileBastion(machine *extensionsv1alpha1.BastionMachine) *gardencorev1beta1.Bastion {
	if machine == nil {
		return nil
	}

	bastion := &gardencorev1beta1.Bastion{}

	if machine.Type != nil {
		bastion.MachineType = &gardencorev1beta1.BastionMachineType{Name: *machine.Type}
	}

	if machine.Image != nil {
		bastion.MachineImage = &gardencorev1beta1.BastionMachineImage{
			Name:    machine.Image.Name,
			Version: machine.Image.Version,
		}
	}

	return bastion
}
