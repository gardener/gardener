// Copyright (c) 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

	. "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

var _ = Describe("CloudProfile defaulting", func() {
	Describe("#SetDefaults_MachineImageVersion", func() {
		var obj *MachineImageVersion

		BeforeEach(func() {
			obj = &MachineImageVersion{}
		})

		It("should correctly set the default MachineImageVersion", func() {
			SetDefaults_MachineImageVersion(obj)

			Expect(len(obj.CRI)).To(Equal(1))
			Expect(obj.CRI[0].Name).To(Equal(CRINameDocker))
			Expect(obj.Architectures).To(Equal([]string{"amd64"}))
		})
	})

	Describe("#SetDefaults_MachineType", func() {
		var obj *MachineType

		BeforeEach(func() {
			obj = &MachineType{}
		})

		It("should correctly set the default MachineType", func() {
			SetDefaults_MachineType(obj)

			Expect(*obj.Architecture).To(Equal("amd64"))
			Expect(*obj.Usable).To(BeTrue())
		})
	})
})
