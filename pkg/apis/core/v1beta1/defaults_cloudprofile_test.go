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

package v1beta1_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"

	. "github.com/gardener/gardener/pkg/apis/core/v1beta1"
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

	Describe("MachineImageVersion defaulting", func() {
		It("should correctly default MachineImageVersion", func() {
			SetObjectDefaults_CloudProfile(obj)

			machineImageVersion := obj.Spec.MachineImages[0].Versions[0]
			Expect(machineImageVersion.CRI).To(ConsistOf(
				CRI{Name: "docker"},
			))
			Expect(machineImageVersion.Architectures).To(ConsistOf(
				"amd64",
			))
		})
	})

	Describe("MachineType defaulting", func() {
		It("should correctly default MachineType", func() {
			SetObjectDefaults_CloudProfile(obj)

			machineType := obj.Spec.MachineTypes[0]
			Expect(machineType.Architecture).To(PointTo(Equal("amd64")))
			Expect(machineType.Usable).To(PointTo(BeTrue()))
		})
	})

	Describe("VolumeType defaulting", func() {
		It("should correctly default VolumeType", func() {
			SetObjectDefaults_CloudProfile(obj)

			volumeType := obj.Spec.VolumeTypes[0]
			Expect(volumeType.Usable).To(PointTo(BeTrue()))
		})
	})
})
