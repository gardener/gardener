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
package shoot_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"time"

	. "github.com/gardener/gardener/pkg/controllermanager/controller/shoot"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
)

var _ = Describe("Shoot Maintenance", func() {
	now := time.Now()
	expirationDateInTheFuture := metav1.Time{Time: now.Add(time.Minute * 10)}
	expirationDateInThePast := metav1.Time{Time: now.AddDate(0, 0, -1)}
	trueVar := true
	falseVar := false

	Context("Shoot Maintenance", func() {
		Describe("ExpirationDateExpired", func() {
			It("should determine that expirationDate applies", func() {
				applies := ExpirationDateExpired(&expirationDateInThePast)
				Expect(applies).To(Equal(trueVar))
			})

			It("should determine that expirationDate not applies", func() {
				applies := ExpirationDateExpired(&expirationDateInTheFuture)
				Expect(applies).To(Equal(falseVar))
			})
		})
		Describe("ForceUpdateRequired", func() {
			var (
				shootCurrentImage = &gardenv1beta1.ShootMachineImage{
					Name:    "CoreOs",
					Version: "1.0.0",
				}
			)
			It("should determine that forceUpdate is required", func() {
				imageCloudProvider := gardenv1beta1.MachineImage{
					Name: "CoreOs",
					Versions: []gardenv1beta1.MachineImageVersion{
						{
							Version: "1.0.1",
						},
						{
							Version:        "1.0.0",
							ExpirationDate: &expirationDateInThePast,
						},
					},
				}

				required := ForceUpdateRequired(shootCurrentImage, imageCloudProvider)
				Expect(required).To(Equal(trueVar))
			})

			It("should determine that forceUpdate is not required", func() {
				imageCloudProvider := gardenv1beta1.MachineImage{
					Name: "CoreOs",
					Versions: []gardenv1beta1.MachineImageVersion{
						{
							Version: "1.0.1",
						},
						{
							Version:        "1.0.0",
							ExpirationDate: &expirationDateInTheFuture,
						},
					},
				}

				required := ForceUpdateRequired(shootCurrentImage, imageCloudProvider)
				Expect(required).To(Equal(falseVar))
			})
		})
	})
	Describe("MaintainMachineImage", func() {
		var (
			shootCurrentImage *gardenv1beta1.ShootMachineImage
			cloudProfile      *gardenv1beta1.CloudProfile
			shoot             *gardenv1beta1.Shoot
		)

		BeforeEach(func() {
			shootCurrentImage = &gardenv1beta1.ShootMachineImage{
				Name:    "CoreOs",
				Version: "1.0.0",
			}

			cloudProfile = &gardenv1beta1.CloudProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name: "profile",
				},
				Spec: gardenv1beta1.CloudProfileSpec{
					GCP: &gardenv1beta1.GCPProfile{
						Constraints: gardenv1beta1.GCPConstraints{
							MachineImages: []gardenv1beta1.MachineImage{
								{
									Name: "CoreOs",

									Versions: []gardenv1beta1.MachineImageVersion{
										{Version: "1.0.0"},
										{Version: "1.1.1",
											ExpirationDate: &expirationDateInTheFuture},
									},
								},
							},
						},
					},
				},
			}

			shoot = &gardenv1beta1.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Name: "shoot",
				},
				Spec: gardenv1beta1.ShootSpec{
					Maintenance: &gardenv1beta1.Maintenance{
						AutoUpdate: &gardenv1beta1.MaintenanceAutoUpdate{
							MachineImageVersion: &trueVar,
						},
					},
				},
			}
		})
		It("should determine that the shoots machine image must be maintained - ForceUpdate", func() {
			shoot.Spec.Maintenance.AutoUpdate.MachineImageVersion = &falseVar
			cloudProfile.Spec.GCP.Constraints.MachineImages[0].Versions[0].ExpirationDate = &expirationDateInThePast
			shoot.Spec.Cloud.GCP = &gardenv1beta1.GCPCloud{MachineImage: shootCurrentImage}
			hasToBeMaintained, image, err := MaintainMachineImage(shoot, cloudProfile, shootCurrentImage)
			Expect(err).To(BeNil())
			Expect(hasToBeMaintained).To(Equal(trueVar))
			Expect(image.Name).To(Equal(cloudProfile.Spec.GCP.Constraints.MachineImages[0].Name))
			Expect(image.Version).To(Equal(cloudProfile.Spec.GCP.Constraints.MachineImages[0].Versions[1].Version))
		})

		It("should determine that the shoots machine image must be maintained - MaintenanceAutoUpdate set to true (nil is also is being defaulted to true in the apiserver)", func() {
			shoot.Spec.Cloud.GCP = &gardenv1beta1.GCPCloud{MachineImage: shootCurrentImage}
			hasToBeMaintained, image, err := MaintainMachineImage(shoot, cloudProfile, shootCurrentImage)
			Expect(err).To(BeNil())
			Expect(hasToBeMaintained).To(Equal(trueVar))
			Expect(image.Name).To(Equal(cloudProfile.Spec.GCP.Constraints.MachineImages[0].Name))
			Expect(image.Version).To(Equal(cloudProfile.Spec.GCP.Constraints.MachineImages[0].Versions[1].Version))
		})

		It("should determine that the shoots machine image must NOT to be maintained - ForceUpdate not required & MaintenanceAutoUpdate set to false", func() {
			shoot.Spec.Maintenance.AutoUpdate.MachineImageVersion = &falseVar
			shoot.Spec.Cloud.GCP = &gardenv1beta1.GCPCloud{MachineImage: shootCurrentImage}
			hasToBeMaintained, _, err := MaintainMachineImage(shoot, cloudProfile, shootCurrentImage)
			Expect(err).To(BeNil())
			Expect(hasToBeMaintained).To(Equal(falseVar))
		})

		It("should determine that the shoots machine image must be maintained - cloud profile has no matching (machineImage.name & machineImage.version) machine image defined (the shoots image has been deleted from the cloudProfile) -> update to latest machineImage with same name", func() {
			cloudProfile.Spec.GCP.Constraints.MachineImages = []gardenv1beta1.MachineImage{
				{
					Name: "CoreOs",
					Versions: []gardenv1beta1.MachineImageVersion{
						{
							Version:        "1.1.1",
							ExpirationDate: &expirationDateInTheFuture,
						},
					},
				},
			}
			shoot.Spec.Cloud.GCP = &gardenv1beta1.GCPCloud{MachineImage: shootCurrentImage}

			hasToBeMaintained, image, err := MaintainMachineImage(shoot, cloudProfile, shootCurrentImage)
			Expect(err).To(BeNil())
			Expect(hasToBeMaintained).To(Equal(trueVar))
			Expect(image.Name).To(Equal(cloudProfile.Spec.GCP.Constraints.MachineImages[0].Name))
			Expect(image.Version).To(Equal(cloudProfile.Spec.GCP.Constraints.MachineImages[0].Versions[0].Version))
		})

		It("should return an error - cloud profile has no matching (machineImage.name) machine image defined", func() {
			cloudProfile.Spec.GCP.Constraints.MachineImages = cloudProfile.Spec.GCP.Constraints.MachineImages[1:]
			shoot.Spec.Cloud.GCP = &gardenv1beta1.GCPCloud{MachineImage: shootCurrentImage}
			_, _, err := MaintainMachineImage(shoot, cloudProfile, shootCurrentImage)
			Expect(err).NotTo(BeNil())
		})
	})
})
