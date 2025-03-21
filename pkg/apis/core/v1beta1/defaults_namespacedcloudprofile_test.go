// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package v1beta1_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"

	. "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

var _ = Describe("NamespacedCloudProfile defaulting", func() {
	var obj *NamespacedCloudProfile

	BeforeEach(func() {
		obj = &NamespacedCloudProfile{
			Spec: NamespacedCloudProfileSpec{
				MachineImages: []MachineImage{{
					Versions: []MachineImageVersion{{}},
				}},
				MachineTypes: []MachineType{{}},
				VolumeTypes:  []VolumeType{{}},
			},
		}
	})

	Describe("MachineImage defaulting", func() {
		It("should not set any additional field defaults", func() {
			originalObj := obj.DeepCopy()
			SetObjectDefaults_NamespacedCloudProfile(obj)
			Expect(obj.Spec.MachineImages).To(Equal(originalObj.Spec.MachineImages))
		})
	})

	Describe("MachineType defaulting", func() {
		It("should correctly default MachineType", func() {
			SetObjectDefaults_NamespacedCloudProfile(obj)

			machineType := obj.Spec.MachineTypes[0]
			Expect(machineType.Usable).To(PointTo(BeTrue()))
		})
	})

	Describe("VolumeType defaulting", func() {
		It("should correctly default VolumeType", func() {
			SetObjectDefaults_NamespacedCloudProfile(obj)

			volumeType := obj.Spec.VolumeTypes[0]
			Expect(volumeType.Usable).To(PointTo(BeTrue()))
		})
	})
})
