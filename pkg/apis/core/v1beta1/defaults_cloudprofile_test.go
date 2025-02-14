// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package v1beta1_test

import (
	"fmt"
	. "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/apiserver/features"
	coreFeatures "github.com/gardener/gardener/pkg/features"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

var _ = Describe("CloudProfile defaulting", func() {
	var obj *CloudProfile
	features.RegisterFeatureGates()

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

	Describe("VolumeType defaulting", func() {
		It("should correctly default VolumeType", func() {
			SetObjectDefaults_CloudProfile(obj)

			volumeType := obj.Spec.VolumeTypes[0]
			Expect(volumeType.Usable).To(PointTo(BeTrue()))
		})
	})

	Describe("MachineImageVersion defaulting with CloudProfileCapabilities == false ", func() {
		BeforeEach(func() {
			DeferCleanup(test.WithFeatureGate(coreFeatures.DefaultFeatureGate, coreFeatures.CloudProfileCapabilities, false))
		})
		var (
			expectedArchitecture string = v1beta1constants.ArchitectureAMD64
		)
		It("should correctly default MachineImageVersion", func() {
			SetObjectDefaults_CloudProfile(obj)
			machineImageVersion := obj.Spec.MachineImages[0].Versions[0]
			Expect(machineImageVersion.CRI).To(ConsistOf(
				CRI{Name: "containerd"},
			))
			Expect(machineImageVersion.Architectures).To(ConsistOf(expectedArchitecture))
			Expect(machineImageVersion.CapabilitiesSet).To(BeNil())
		})

		Describe("MachineType defaulting", func() {
			It("should correctly default MachineType", func() {
				SetObjectDefaults_CloudProfile(obj)

				machineType := obj.Spec.MachineTypes[0]
				Expect(machineType.Architecture).To(PointTo(Equal(expectedArchitecture)))
				Expect(machineType.Capabilities).To(BeNil())
				Expect(machineType.Usable).To(PointTo(BeTrue()))
			})
		})
	})

	Describe("MachineImageVersion defaulting with CloudProfileCapabilities == true ", func() {
		BeforeEach(func() {
			DeferCleanup(test.WithFeatureGate(coreFeatures.DefaultFeatureGate, coreFeatures.CloudProfileCapabilities, true))
		})
		var (
			expectedCapabilities    = Capabilities{"architecture": v1beta1constants.ArchitectureAMD64}
			expectedCapabilitiesSet = []v1.JSON{{Raw: []byte(`{"architecture":"` + v1beta1constants.ArchitectureAMD64 + `"}`)}}
		)

		It("should correctly default MachineImageVersion", func() {
			SetObjectDefaults_CloudProfile(obj)
			fmt.Print(expectedCapabilitiesSet)
			machineImageVersion := obj.Spec.MachineImages[0].Versions[0]
			Expect(machineImageVersion.CRI).To(ConsistOf(
				CRI{Name: "containerd"},
			))
			Expect(machineImageVersion.Architectures).To(BeNil())
			Expect(machineImageVersion.CapabilitiesSet).To(Equal(expectedCapabilitiesSet))
		})

		Describe("MachineType defaulting", func() {
			It("should correctly default MachineType", func() {
				SetObjectDefaults_CloudProfile(obj)

				machineType := obj.Spec.MachineTypes[0]
				Expect(machineType.Architecture).To(BeNil())
				Expect(machineType.Capabilities).To(Equal(expectedCapabilities))
				Expect(machineType.Usable).To(PointTo(BeTrue()))
			})
		})
	})

})
