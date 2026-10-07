// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package local

import (
	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("CreateMachine", func() {
	Describe("#nodeTemplateLabelsForMachine", func() {
		It("should return no labels if the machine class has no node template", func() {
			Expect(nodeTemplateLabelsForMachine(nil)).To(BeEmpty())
			Expect(nodeTemplateLabelsForMachine(&machinev1alpha1.MachineClass{})).To(BeEmpty())
		})

		It("should return the labels from the node template", func() {
			Expect(nodeTemplateLabelsForMachine(&machinev1alpha1.MachineClass{
				NodeTemplate: &machinev1alpha1.NodeTemplate{
					InstanceType: "local",
					Region:       "local",
					Zone:         "local-a",
				},
			})).To(Equal(map[string]string{
				"local.provider.extensions.gardener.cloud/instance-type": "local",
				"local.provider.extensions.gardener.cloud/region":        "local",
				"local.provider.extensions.gardener.cloud/zone":          "local-a",
			}))
		})

		It("should only return labels for non-empty fields", func() {
			Expect(nodeTemplateLabelsForMachine(&machinev1alpha1.MachineClass{
				NodeTemplate: &machinev1alpha1.NodeTemplate{Zone: "local-a"},
			})).To(Equal(map[string]string{
				"local.provider.extensions.gardener.cloud/zone": "local-a",
			}))
		})
	})
})
