// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package bastion_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/gardener/gardener/extensions/pkg/bastion"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
)

var _ = Describe("#BastionMachineToCloudProfileBastion", func() {
	DescribeTable("should map the machine override to the CloudProfile Bastion type",
		func(machine *extensionsv1alpha1.BastionMachine, expected *gardencorev1beta1.Bastion) {
			Expect(BastionMachineToCloudProfileBastion(machine)).To(Equal(expected))
		},
		Entry("nil machine override", nil, nil),
		Entry("fully specified machine override",
			&extensionsv1alpha1.BastionMachine{
				Type:  "large",
				Image: extensionsv1alpha1.BastionMachineImage{Name: "gardenlinux", Version: "1.2.3"},
			},
			&gardencorev1beta1.Bastion{
				MachineType:  &gardencorev1beta1.BastionMachineType{Name: "large"},
				MachineImage: &gardencorev1beta1.BastionMachineImage{Name: "gardenlinux", Version: new("1.2.3")},
			},
		),
	)
})
