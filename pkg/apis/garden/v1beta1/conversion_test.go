// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package v1beta1

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"time"

	"github.com/gardener/gardener/pkg/apis/garden"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Conversion", func() {
	var (
		expirationDate     = &metav1.Time{Time: time.Now().Add(time.Second * 20)}
		v1betaMachineImage *MachineImage
		gardenMachineImage *garden.MachineImage
	)

	BeforeEach(func() {
		v1betaMachineImage = &MachineImage{
			Name: "coreos",
			Versions: []MachineImageVersion{
				{
					Version:        "0.0.9",
					ExpirationDate: expirationDate,
				},
				{
					Version:        "0.0.7",
					ExpirationDate: expirationDate,
				},
				{
					Version:        "0.0.8",
					ExpirationDate: expirationDate,
				},
			},
		}

		gardenMachineImage = &garden.MachineImage{
			Name: "coreos",
			Versions: []garden.MachineImageVersion{
				{
					Version: "1.0.0",
				},
				{
					Version:        "0.0.9",
					ExpirationDate: expirationDate,
				},
				{
					Version:        "0.0.7",
					ExpirationDate: expirationDate,
				},
				{
					Version:        "0.0.8",
					ExpirationDate: expirationDate,
				},
			},
		}
	})

	Describe("#V1Beta1MachineImageToGardenMachineImage", func() {
		It("external machine image should be properly converted to internal machine image", func() {
			v1betaMachineImage.Version = "1.0.0"
			internalMachineImage := &garden.MachineImage{}

			Convert_v1beta1_MachineImage_To_garden_MachineImage(v1betaMachineImage, internalMachineImage, nil)

			Expect(internalMachineImage.Name).To(Equal("coreos"))
			Expect(internalMachineImage.Versions).To(HaveLen(4))
			Expect(internalMachineImage.Versions[0].Version).To(Equal("1.0.0"))
			Expect(internalMachineImage.Versions[0].ExpirationDate).To(BeNil())
		})
		It("external machine image (no version set) should be properly converted to internal machine image", func() {
			internalMachineImage := &garden.MachineImage{}

			Convert_v1beta1_MachineImage_To_garden_MachineImage(v1betaMachineImage, internalMachineImage, nil)

			Expect(internalMachineImage.Name).To(Equal("coreos"))
			Expect(internalMachineImage.Versions).To(HaveLen(3))
			Expect(internalMachineImage.Versions[0].Version).To(Equal("0.0.9"))
		})
	},
	)
	Describe("#GardenMachineImageToV1Beta1MachineImage", func() {
		It("internal machine image should be properly converted to external machine image", func() {
			v1betaMachineImage := &MachineImage{}

			Convert_garden_MachineImage_To_v1beta1_MachineImage(gardenMachineImage, v1betaMachineImage, nil)

			Expect(v1betaMachineImage.Name).To(Equal("coreos"))
			Expect(v1betaMachineImage.Version).To(HaveLen(0))
			Expect(v1betaMachineImage.Versions).To(HaveLen(4))
			Expect(v1betaMachineImage.Versions[0].Version).To(Equal("1.0.0"))
			Expect(v1betaMachineImage.Versions[0].ExpirationDate).To(BeNil())
		})
	})

	Describe("#GardenMachineImageBackAndForth", func() {
		It("assure no structural change in resulting external version after back and forth conversion", func() {
			internalMachineImage := &garden.MachineImage{}

			Convert_v1beta1_MachineImage_To_garden_MachineImage(v1betaMachineImage, internalMachineImage, nil)

			v1betaMachineImageResult := &MachineImage{}

			Convert_garden_MachineImage_To_v1beta1_MachineImage(internalMachineImage, v1betaMachineImageResult, nil)

			Expect(v1betaMachineImageResult).To(Equal(v1betaMachineImage))
		})
		It("assure expected structural change (when image.Version is set in v1beta1) in resulting external version after back and forth conversion", func() {
			v1betaMachineImage.Version = "1.0.0"

			internalMachineImage := &garden.MachineImage{}

			Convert_v1beta1_MachineImage_To_garden_MachineImage(v1betaMachineImage, internalMachineImage, nil)

			v1betaMachineImageResult := &MachineImage{}

			Convert_garden_MachineImage_To_v1beta1_MachineImage(internalMachineImage, v1betaMachineImageResult, nil)

			Expect(v1betaMachineImageResult).ToNot(Equal(v1betaMachineImage))
			Expect(v1betaMachineImageResult.Version).To(HaveLen(0))
			Expect(v1betaMachineImageResult.Versions).To(HaveLen(4))
			Expect(v1betaMachineImageResult.Versions[0].Version).To(Equal("1.0.0"))
			Expect(v1betaMachineImageResult.Versions[0].ExpirationDate).To(BeNil())
		})
	})
},
)
