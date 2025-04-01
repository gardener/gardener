// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package v1beta1_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"

	. "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
)

var _ = Describe("CloudProfile defaulting", func() {
	var obj *CloudProfile

	BeforeEach(func() {
		obj = &CloudProfile{
			Spec: CloudProfileSpec{
				MachineImages: []MachineImage{{
					Versions: []MachineImageVersion{{}},
				}},
				MachineTypes: []MachineType{{}},
				VolumeTypes:  []VolumeType{{}},
			},
		}
	})

	Describe("MachineImage defaulting", func() {
		It("should correctly default MachineImage", func() {
			SetObjectDefaults_CloudProfile(obj)

			machineImage := obj.Spec.MachineImages[0]
			Expect(machineImage.UpdateStrategy).To(PointTo(Equal(UpdateStrategyMajor)))
		})
	})

	Describe("MachineImageVersion defaulting", func() {
		It("should correctly default MachineImageVersion", func() {
			SetObjectDefaults_CloudProfile(obj)
			machineImageVersion := obj.Spec.MachineImages[0].Versions[0]
			Expect(machineImageVersion.CRI).To(ConsistOf(
				CRI{Name: "containerd"},
			))
			Expect(machineImageVersion.Architectures).To(ConsistOf("amd64"))
			Expect(machineImageVersion.CapabilitySets).To(BeNil())
		})
	})

	Describe("VolumeType defaulting", func() {
		It("should correctly default VolumeType", func() {
			SetObjectDefaults_CloudProfile(obj)

			volumeType := obj.Spec.VolumeTypes[0]
			Expect(volumeType.Usable).To(PointTo(BeTrue()))
		})
	})

	Describe("MachineType defaulting", func() {
		It("should correctly default MachineType", func() {
			SetObjectDefaults_CloudProfile(obj)

			machineType := obj.Spec.MachineTypes[0]
			Expect(machineType.Architecture).To(PointTo(Equal("amd64")))
			Expect(machineType.Capabilities).To(BeNil())
			Expect(machineType.Usable).To(PointTo(BeTrue()))
		})
	})

	Describe("Architecture defaulting", func() {
		Context("Without Capabilities", func() {
			It("should default the architecture for MachineImageVersion and MachineType", func() {
				SetObjectDefaults_CloudProfile(obj)

				Expect(obj.Spec.MachineImages[0].Versions[0].Architectures).To(ConsistOf("amd64"))
				Expect(obj.Spec.MachineTypes[0].Architecture).To(PointTo(Equal("amd64")))

				Expect(obj.Spec.MachineImages[0].Versions[0].CapabilitySets).To(BeEmpty())
				Expect(obj.Spec.MachineTypes[0].Capabilities).To(BeNil())
			})
		})

		Context("With Capabilities", func() {
			It("should not default the architecture for MachineImageVersion and MachineType", func() {
				obj.Spec.Capabilities = Capabilities{
					v1beta1constants.ArchitectureKey: []string{"arm64"},
				}
				SetObjectDefaults_CloudProfile(obj)

				Expect(obj.Spec.MachineImages[0].Versions[0].Architectures).To(BeEmpty())
				Expect(obj.Spec.MachineTypes[0].Architecture).To(BeNil())

				Expect(obj.Spec.MachineImages[0].Versions[0].CapabilitySets).To(BeEmpty())
				Expect(obj.Spec.MachineTypes[0].Capabilities).To(BeNil())
			})
		})
	})
})
