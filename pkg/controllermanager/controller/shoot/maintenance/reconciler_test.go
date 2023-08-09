// Copyright 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package maintenance

import (
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

var _ = Describe("Shoot Maintenance", func() {
	var (
		log logr.Logger
		now time.Time

		expirationDateInTheFuture, expirationDateInThePast metav1.Time
	)

	BeforeEach(func() {
		log = logr.Discard()
		now = time.Now()
		expirationDateInTheFuture = metav1.Time{Time: now.Add(time.Minute * 10)}
		expirationDateInThePast = metav1.Time{Time: now.AddDate(0, 0, -1)}
	})

	Context("Shoot Maintenance", func() {
		Describe("#ExpirationDateExpired", func() {
			It("should determine that expirationDate applies", func() {
				applies := ExpirationDateExpired(&expirationDateInThePast)
				Expect(applies).To(Equal(true))
			})

			It("should determine that expirationDate not applies", func() {
				applies := ExpirationDateExpired(&expirationDateInTheFuture)
				Expect(applies).To(Equal(false))
			})
		})
	})

	Describe("#MaintainMachineImages", func() {
		var (
			shootCurrentImage        *gardencorev1beta1.ShootMachineImage
			cloudProfile             *gardencorev1beta1.CloudProfile
			shoot                    *gardencorev1beta1.Shoot
			previewClassification    = gardencorev1beta1.ClassificationPreview
			deprecatedClassification = gardencorev1beta1.ClassificationDeprecated
			shootCurrentImageVersion = "1.0.0"
			overallLatestVersion     = "1.5.3"
		)

		BeforeEach(func() {
			shootCurrentImage = &gardencorev1beta1.ShootMachineImage{
				Name:    "CoreOs",
				Version: pointer.String(shootCurrentImageVersion),
			}

			strategyMajor := gardencorev1beta1.UpdateStrategyMajor
			cloudProfile = &gardencorev1beta1.CloudProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name: "profile",
				},
				Spec: gardencorev1beta1.CloudProfileSpec{
					MachineImages: []gardencorev1beta1.MachineImage{
						{
							Name: "CoreOs",
							// set default strategy as set by the APIServer for tests that are not strategy specific
							UpdateStrategy: &strategyMajor,
							Versions: []gardencorev1beta1.MachineImageVersion{
								{
									ExpirableVersion: gardencorev1beta1.ExpirableVersion{
										Version: shootCurrentImageVersion,
									},
									CRI:           []gardencorev1beta1.CRI{{Name: gardencorev1beta1.CRINameDocker}},
									Architectures: []string{"amd64"},
								},
								{
									ExpirableVersion: gardencorev1beta1.ExpirableVersion{
										Version:        overallLatestVersion,
										ExpirationDate: &expirationDateInTheFuture,
									},
									CRI:           []gardencorev1beta1.CRI{{Name: gardencorev1beta1.CRINameDocker}},
									Architectures: []string{"amd64"},
								},
							},
						},
					},
				},
			}

			shoot = &gardencorev1beta1.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Name: "shoot",
				},
				Spec: gardencorev1beta1.ShootSpec{
					Kubernetes: gardencorev1beta1.Kubernetes{
						Version: "1.25.1",
					},
					Maintenance: &gardencorev1beta1.Maintenance{
						AutoUpdate: &gardencorev1beta1.MaintenanceAutoUpdate{
							MachineImageVersion: pointer.Bool(true),
						},
					},
					Provider: gardencorev1beta1.Provider{Workers: []gardencorev1beta1.Worker{
						{
							Name: "cpu-worker",
							Machine: gardencorev1beta1.Machine{
								Image:        shootCurrentImage,
								Architecture: pointer.String("amd64"),
							},
						},
					},
					},
				},
			}
		})

		Describe("UpdateStrategy: Major", func() {
			BeforeEach(func() {
				updateStrategyMajor := gardencorev1beta1.UpdateStrategyMajor
				cloudProfile.Spec.MachineImages[0].UpdateStrategy = &updateStrategyMajor
			})

			It("should update machine image version to overall latest. Auto update: already on latest patch for minor, and there is an overall higher version available", func() {
				_, err := maintainMachineImages(log, shoot, cloudProfile)

				Expect(err).NotTo(HaveOccurred())
				assertWorkerMachineImageVersion(&shoot.Spec.Provider.Workers[0], "CoreOs", overallLatestVersion)
			})

			It("should update machine image version to overall latest for correct architecture. Auto update: already on latest patch for minor, and there is an overall higher version available", func() {
				expectedVersion := "1.1.2"

				// set all version for image only compatible with amd64
				for i := 0; i < len(cloudProfile.Spec.MachineImages[0].Versions); i++ {
					cloudProfile.Spec.MachineImages[0].Versions[i].Architectures = []string{"amd64"}
				}

				// add relevant arm64 images to be updated to
				cloudProfile.Spec.MachineImages[0].Versions = append(cloudProfile.Spec.MachineImages[0].Versions, gardencorev1beta1.MachineImageVersion{
					ExpirableVersion: gardencorev1beta1.ExpirableVersion{
						Version:        expectedVersion,
						ExpirationDate: &expirationDateInTheFuture,
					},
					CRI:           []gardencorev1beta1.CRI{{Name: gardencorev1beta1.CRINameDocker}},
					Architectures: []string{"arm64"},
				})

				// add overall higher version with wrong architecture (should be ignored)
				cloudProfile.Spec.MachineImages[0].Versions = append(cloudProfile.Spec.MachineImages[0].Versions, gardencorev1beta1.MachineImageVersion{
					ExpirableVersion: gardencorev1beta1.ExpirableVersion{
						Version:        "1.7.1",
						ExpirationDate: &expirationDateInTheFuture,
					},
					CRI:           []gardencorev1beta1.CRI{{Name: gardencorev1beta1.CRINameDocker}},
					Architectures: []string{"amd64"},
				})

				shoot.Spec.Provider.Workers[0].Machine.Architecture = pointer.String("arm64")

				_, err := maintainMachineImages(log, shoot, cloudProfile)
				Expect(err).NotTo(HaveOccurred())
				assertWorkerMachineImageVersion(&shoot.Spec.Provider.Workers[0], "CoreOs", expectedVersion)
			})

			It("should update version of multiple worker pools to the overall latest of the respective images. Auto update: multiple worker pools", func() {
				expectedVersionGLWorker := "1.0.1"

				// add a gardenlinux machine image to the cloudprofile to be used by the second worker
				autoUpdateStrategyMajor := gardencorev1beta1.UpdateStrategyMajor
				cloudProfile.Spec.MachineImages = append(cloudProfile.Spec.MachineImages, gardencorev1beta1.MachineImage{
					Name:           "gardenlinux",
					UpdateStrategy: &autoUpdateStrategyMajor,
					Versions: []gardencorev1beta1.MachineImageVersion{
						{
							ExpirableVersion: gardencorev1beta1.ExpirableVersion{
								Version: "1.0.0",
							},
							CRI:           []gardencorev1beta1.CRI{{Name: gardencorev1beta1.CRINameDocker}},
							Architectures: []string{"amd64"},
						},
						{
							ExpirableVersion: gardencorev1beta1.ExpirableVersion{
								Version: expectedVersionGLWorker,
							},
							CRI:           []gardencorev1beta1.CRI{{Name: gardencorev1beta1.CRINameDocker}},
							Architectures: []string{"amd64"},
						},
					},
				})

				otherWorker := gardencorev1beta1.Worker{
					Name: "cpu-glinux",
					Machine: gardencorev1beta1.Machine{
						Image: &gardencorev1beta1.ShootMachineImage{
							Name:    "gardenlinux",
							Version: pointer.String("1.0.0"),
						},
						Architecture: pointer.String("amd64"),
					},
				}

				shoot.Spec.Provider.Workers = append(shoot.Spec.Provider.Workers, otherWorker)
				_, err := maintainMachineImages(log, shoot, cloudProfile)

				Expect(err).NotTo(HaveOccurred())

				assertWorkerMachineImageVersion(&shoot.Spec.Provider.Workers[0], "CoreOs", overallLatestVersion)
				assertWorkerMachineImageVersion(&shoot.Spec.Provider.Workers[1], "gardenlinux", expectedVersionGLWorker)
			})

			It("should auto update to latest non-preview version of the same minor version. Auto update: version is not the latest patch version of the current minor. Instead of updating to overall latest right away, first update to latest patch of current minor.", func() {
				expectedVersion := "1.0.2"
				highestForMinor := gardencorev1beta1.MachineImageVersion{
					ExpirableVersion: gardencorev1beta1.ExpirableVersion{
						Version: expectedVersion,
					},
					Architectures: []string{"amd64"},
					CRI:           []gardencorev1beta1.CRI{{Name: gardencorev1beta1.CRINameDocker}},
				}

				cloudProfile.Spec.MachineImages[0].Versions = append(cloudProfile.Spec.MachineImages[0].Versions, highestForMinor)

				_, err := maintainMachineImages(log, shoot, cloudProfile)

				Expect(err).NotTo(HaveOccurred())
				assertWorkerMachineImageVersion(&shoot.Spec.Provider.Workers[0], "CoreOs", expectedVersion)
			})

			It("should update machine image version to overall latest. ForceUpdate: expiration date in the past", func() {
				shoot.Spec.Maintenance.AutoUpdate.MachineImageVersion = pointer.Bool(false)
				cloudProfile.Spec.MachineImages[0].Versions[0].ExpirationDate = &expirationDateInThePast

				_, err := maintainMachineImages(log, shoot, cloudProfile)

				Expect(err).NotTo(HaveOccurred())
				assertWorkerMachineImageVersion(&shoot.Spec.Provider.Workers[0], "CoreOs", overallLatestVersion)
			})

			It("should update machine image version to overall latest. Force update: cloud profile has no matching machine image defined -> update to latest machineImage of same patch version or to overall latest", func() {
				// there is a higher patch version compared to the current shoot image version -> update to that first
				expectedVersion := "1.0.1"
				autoUpdateStrategy := gardencorev1beta1.UpdateStrategyMajor
				cloudProfile.Spec.MachineImages = []gardencorev1beta1.MachineImage{
					{
						Name:           "CoreOs",
						UpdateStrategy: &autoUpdateStrategy,
						Versions: []gardencorev1beta1.MachineImageVersion{
							{
								ExpirableVersion: gardencorev1beta1.ExpirableVersion{
									Version:        "1.1.1",
									ExpirationDate: &expirationDateInTheFuture,
								},
								CRI:           []gardencorev1beta1.CRI{{Name: gardencorev1beta1.CRINameDocker}},
								Architectures: []string{"amd64"},
							},
							{
								ExpirableVersion: gardencorev1beta1.ExpirableVersion{
									Version:        expectedVersion,
									ExpirationDate: &expirationDateInTheFuture,
								},
								CRI:           []gardencorev1beta1.CRI{{Name: gardencorev1beta1.CRINameDocker}},
								Architectures: []string{"amd64"},
							},
						},
					},
				}

				_, err := maintainMachineImages(log, shoot, cloudProfile)

				Expect(err).NotTo(HaveOccurred())
				assertWorkerMachineImageVersion(&shoot.Spec.Provider.Workers[0], "CoreOs", expectedVersion)
			})

			It("should return a maintenance failure - edge case: all qualifying versions from the CloudProfile for machine image are lower than the Shoot's version (Shoot has the highest version and it is expired). Should not downgrade shoot machine image version.", func() {
				highestExpiredVersion := gardencorev1beta1.MachineImageVersion{
					ExpirableVersion: gardencorev1beta1.ExpirableVersion{
						Version:        "1.7.2",
						Classification: &deprecatedClassification,
						ExpirationDate: &expirationDateInThePast,
					},
				}
				cloudProfile.Spec.MachineImages[0].Versions = append(cloudProfile.Spec.MachineImages[0].Versions, highestExpiredVersion)
				shoot.Spec.Provider.Workers[0].Machine.Image.Version = &highestExpiredVersion.Version
				results, err := maintainMachineImages(log, shoot, cloudProfile)

				Expect(results[shoot.Spec.Provider.Workers[0].Name].isSuccessful).To(BeFalse())
				Expect(err).ToNot(HaveOccurred())
			})

			It("should not change version: already on highest version.", func() {
				shoot.Spec.Provider.Workers[0].Machine.Image.Version = &overallLatestVersion
				_, err := maintainMachineImages(log, shoot, cloudProfile)

				Expect(err).NotTo(HaveOccurred())
				assertWorkerMachineImageVersion(&shoot.Spec.Provider.Workers[0], "CoreOs", overallLatestVersion)
			})
		})

		Describe("UpdateStrategy: patch", func() {
			var (
				// Note: this is not a consecutive minor for the current shoot version 1.0.0
				highestPatchNextMinor = "1.2.5"
				lowerPatchNextMinor   = "1.2.4"
			)

			BeforeEach(func() {
				updateStrategyPatch := gardencorev1beta1.UpdateStrategyPatch
				cloudProfile.Spec.MachineImages[0].UpdateStrategy = &updateStrategyPatch
			})

			It("should auto-update to the latest patch version for this minor. Auto update: not on latest patch version", func() {
				expectedVersion := "1.0.2"
				latestPathThisMinor := gardencorev1beta1.MachineImageVersion{
					ExpirableVersion: gardencorev1beta1.ExpirableVersion{
						Version: expectedVersion,
					},
					Architectures: []string{"amd64"},
					CRI:           []gardencorev1beta1.CRI{{Name: gardencorev1beta1.CRINameDocker}},
				}

				versions := []gardencorev1beta1.MachineImageVersion{cloudProfile.Spec.MachineImages[0].Versions[0]}
				versions = append(versions, latestPathThisMinor)

				cloudProfile.Spec.MachineImages[0].Versions = versions

				// the shoots patch version is expired and there is no higher non-expired & non-preview patch version of the same minor -> force update
				_, err := maintainMachineImages(log, shoot, cloudProfile)

				Expect(err).NotTo(HaveOccurred())
				assertWorkerMachineImageVersion(&shoot.Spec.Provider.Workers[0], "CoreOs", expectedVersion)
			})

			It("should update to latest non-preview version of the next higher (not necessarily consecutive) minor version. Force update: Version is expired and there is no higher patch version", func() {
				strategyPatch := gardencorev1beta1.UpdateStrategyPatch

				// cause force update: expire Shoot's OS version
				cloudProfile.Spec.MachineImages[0].Versions[0].ExpirationDate = &expirationDateInThePast

				expectedVersion := "1.2.1"
				cloudProfile.Spec.MachineImages = []gardencorev1beta1.MachineImage{
					{
						Name:           "CoreOs",
						UpdateStrategy: &strategyPatch,
						Versions: []gardencorev1beta1.MachineImageVersion{
							{
								ExpirableVersion: gardencorev1beta1.ExpirableVersion{
									// must not update to this version, as is next major (never update to next major for the patch strategy)
									Version:        "2.1.0",
									ExpirationDate: &expirationDateInTheFuture,
								},
								CRI:           []gardencorev1beta1.CRI{{Name: gardencorev1beta1.CRINameDocker}},
								Architectures: []string{"amd64"},
							},
							{
								ExpirableVersion: gardencorev1beta1.ExpirableVersion{
									Version:        expectedVersion,
									ExpirationDate: &expirationDateInTheFuture,
								},
								CRI:           []gardencorev1beta1.CRI{{Name: gardencorev1beta1.CRINameDocker}},
								Architectures: []string{"amd64"},
							},
							{
								ExpirableVersion: gardencorev1beta1.ExpirableVersion{
									Version:        "1.2.0",
									ExpirationDate: &expirationDateInTheFuture,
								},
								CRI:           []gardencorev1beta1.CRI{{Name: gardencorev1beta1.CRINameDocker}},
								Architectures: []string{"amd64"},
							},
							{
								ExpirableVersion: gardencorev1beta1.ExpirableVersion{
									Version:        shootCurrentImageVersion,
									ExpirationDate: &expirationDateInThePast,
								},
								CRI:           []gardencorev1beta1.CRI{{Name: gardencorev1beta1.CRINameDocker}},
								Architectures: []string{"amd64"},
							},
						},
					},
				}

				// the shoots patch version is expired and there is no higher non-expired & non-preview patch version of the same minor -> force update
				_, err := maintainMachineImages(log, shoot, cloudProfile)

				Expect(err).NotTo(HaveOccurred())
				assertWorkerMachineImageVersion(&shoot.Spec.Provider.Workers[0], "CoreOs", expectedVersion)
			})

			It("should update to latest non-preview version of the next higher (not necessarily consecutive) minor version: Force update: Image does not exist in the cloud profile", func() {
				strategyPatch := gardencorev1beta1.UpdateStrategyPatch
				cloudProfile.Spec.MachineImages = []gardencorev1beta1.MachineImage{
					{
						Name:           "CoreOs",
						UpdateStrategy: &strategyPatch,
						Versions: []gardencorev1beta1.MachineImageVersion{
							{
								ExpirableVersion: gardencorev1beta1.ExpirableVersion{
									// must not update to this version, as not the next minor
									Version:        "1.3.1",
									ExpirationDate: &expirationDateInTheFuture,
								},
								CRI:           []gardencorev1beta1.CRI{{Name: gardencorev1beta1.CRINameDocker}},
								Architectures: []string{"amd64"},
							},
							{
								ExpirableVersion: gardencorev1beta1.ExpirableVersion{
									// current version 1.0.0, next minor in cloudprofile: 1.2.X
									Version:        highestPatchNextMinor,
									ExpirationDate: &expirationDateInTheFuture,
								},
								CRI:           []gardencorev1beta1.CRI{{Name: gardencorev1beta1.CRINameDocker}},
								Architectures: []string{"amd64"},
							},
							{
								ExpirableVersion: gardencorev1beta1.ExpirableVersion{
									Version:        lowerPatchNextMinor,
									ExpirationDate: &expirationDateInTheFuture,
								},
								CRI:           []gardencorev1beta1.CRI{{Name: gardencorev1beta1.CRINameDocker}},
								Architectures: []string{"amd64"},
							},
						},
					},
				}

				_, err := maintainMachineImages(log, shoot, cloudProfile)

				Expect(err).NotTo(HaveOccurred())
				assertWorkerMachineImageVersion(&shoot.Spec.Provider.Workers[0], "CoreOs", highestPatchNextMinor)
			})

			It("should update to latest patch in next minor, but next minor has no qualifying version (all preview, would update to expired). Hence, skip the next minor and force update to minor after that", func() {
				// cause force update: expire Shoot's OS version
				cloudProfile.Spec.MachineImages[0].Versions[0].ExpirationDate = &expirationDateInThePast

				// all versions of the next minor are in preview, hence do not qualify for an update (only supported, deprecated and expired versions qualify)
				previewPatchVersionNextMinor := gardencorev1beta1.MachineImageVersion{
					ExpirableVersion: gardencorev1beta1.ExpirableVersion{
						Version:        lowerPatchNextMinor,
						Classification: &previewClassification,
					},
					Architectures: []string{"amd64"},
					CRI:           []gardencorev1beta1.CRI{{Name: gardencorev1beta1.CRINameDocker}},
				}

				// update to the latest patch version of the minor after the next minor (skip next minor)
				highestNonPreviewPatchVersionNplusTwoMinor := gardencorev1beta1.MachineImageVersion{
					ExpirableVersion: gardencorev1beta1.ExpirableVersion{
						Version: "1.4.1",
					},
					Architectures: []string{"amd64"},
					CRI:           []gardencorev1beta1.CRI{{Name: gardencorev1beta1.CRINameDocker}},
				}

				versions := []gardencorev1beta1.MachineImageVersion{cloudProfile.Spec.MachineImages[0].Versions[0]}
				versions = append(versions, previewPatchVersionNextMinor)
				versions = append(versions, highestNonPreviewPatchVersionNplusTwoMinor)

				cloudProfile.Spec.MachineImages[0].Versions = versions

				// the shoots patch version is expired and there is no higher non-expired & non-preview patch version of the same minor -> force update
				_, err := maintainMachineImages(log, shoot, cloudProfile)

				Expect(err).NotTo(HaveOccurred())
				assertWorkerMachineImageVersion(&shoot.Spec.Provider.Workers[0], "CoreOs", highestNonPreviewPatchVersionNplusTwoMinor.Version)
			})

			It("should not skip next minor if has qualifying expired versions. Update to expired latest patch in next minor because next minor has only expired and preview versions", func() {
				// cause force update: expire Shoot's OS version
				cloudProfile.Spec.MachineImages[0].Versions[0].ExpirationDate = &expirationDateInThePast

				// versions of the next minor are in {preview, expired}, hence allow update to expired version
				previewPatchVersionNextMinor := gardencorev1beta1.MachineImageVersion{
					ExpirableVersion: gardencorev1beta1.ExpirableVersion{
						Version:        highestPatchNextMinor,
						Classification: &previewClassification,
					},
					Architectures: []string{"amd64"},
					CRI:           []gardencorev1beta1.CRI{{Name: gardencorev1beta1.CRINameDocker}},
				}
				expiredPatchVersionNextMinor := gardencorev1beta1.MachineImageVersion{
					ExpirableVersion: gardencorev1beta1.ExpirableVersion{
						Version:        lowerPatchNextMinor,
						ExpirationDate: &expirationDateInThePast,
					},
					Architectures: []string{"amd64"},
					CRI:           []gardencorev1beta1.CRI{{Name: gardencorev1beta1.CRINameDocker}},
				}

				// do not update to the latest patch version of the minor after the next minor (no not skip next minor)
				highestNonPreviewPatchVersionNplusTwoMinor := gardencorev1beta1.MachineImageVersion{
					ExpirableVersion: gardencorev1beta1.ExpirableVersion{
						Version: "1.4.1",
					},
					Architectures: []string{"amd64"},
					CRI:           []gardencorev1beta1.CRI{{Name: gardencorev1beta1.CRINameDocker}},
				}

				versions := []gardencorev1beta1.MachineImageVersion{cloudProfile.Spec.MachineImages[0].Versions[0]}
				versions = append(versions, previewPatchVersionNextMinor)
				versions = append(versions, expiredPatchVersionNextMinor)
				versions = append(versions, highestNonPreviewPatchVersionNplusTwoMinor)

				cloudProfile.Spec.MachineImages[0].Versions = versions

				// the shoots patch version is expired and there is no higher non-expired & non-preview patch version of the same minor -> force update
				_, err := maintainMachineImages(log, shoot, cloudProfile)

				Expect(err).NotTo(HaveOccurred())
				assertWorkerMachineImageVersion(&shoot.Spec.Provider.Workers[0], "CoreOs", expiredPatchVersionNextMinor.Version)
			})

			It("should not change version. Auto update: no force update and already on latest qualifying machine image version for minor.", func() {
				highestVersionForMinor := "1.1.1"
				autoUpdateStrategyPatch := gardencorev1beta1.UpdateStrategyPatch
				cloudProfile.Spec.MachineImages = []gardencorev1beta1.MachineImage{
					{
						Name:           "CoreOs",
						UpdateStrategy: &autoUpdateStrategyPatch,
						Versions: []gardencorev1beta1.MachineImageVersion{
							{
								ExpirableVersion: gardencorev1beta1.ExpirableVersion{
									Version: highestVersionForMinor,
								},
								CRI:           []gardencorev1beta1.CRI{{Name: gardencorev1beta1.CRINameDocker}},
								Architectures: []string{"amd64"},
							},
							{
								ExpirableVersion: gardencorev1beta1.ExpirableVersion{
									// highest version of next minor, but Shoot should not update, as current version is not expired.
									Version: "1.2.0",
								},
								CRI:           []gardencorev1beta1.CRI{{Name: gardencorev1beta1.CRINameDocker}},
								Architectures: []string{"amd64"},
							},
						},
					},
				}
				shoot.Spec.Provider.Workers[0].Machine.Image.Version = &highestVersionForMinor
				expected := shoot.Spec.Provider.Workers[0].Machine.Image.DeepCopy()
				_, err := maintainMachineImages(log, shoot, cloudProfile)
				Expect(err).NotTo(HaveOccurred())
				Expect(shoot.Spec.Provider.Workers[0].Machine.Image).To(Equal(expected))
			})

			It("should report a maintenance failure - edge case: all qualifying versions from the CloudProfile for machine image are lower than the Shoot's version (Shoot has the highest version and it is expired). Should not downgrade shoot machine image version.", func() {
				highestExpiredVersion := gardencorev1beta1.MachineImageVersion{
					ExpirableVersion: gardencorev1beta1.ExpirableVersion{
						Version:        "1.7.2",
						Classification: &deprecatedClassification,
						ExpirationDate: &expirationDateInThePast,
					},
				}
				cloudProfile.Spec.MachineImages[0].Versions = append(cloudProfile.Spec.MachineImages[0].Versions, highestExpiredVersion)
				shoot.Spec.Provider.Workers[0].Machine.Image.Version = &highestExpiredVersion.Version
				results, err := maintainMachineImages(log, shoot, cloudProfile)

				Expect(results[shoot.Spec.Provider.Workers[0].Name].isSuccessful).To(BeFalse())
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Describe("UpdateStrategy: minor", func() {
			It("should auto-update to the latest patch version for this minor before considering an update to the latest version in the major. Auto update: not on latest minor.patch version in major", func() {
				autoUpdateStrategyMinor := gardencorev1beta1.UpdateStrategyMinor
				highestVersionForCurrentMajor := "1.5.1"
				highestPatchCurrentMinor := "1.0.6"
				cloudProfile.Spec.MachineImages = []gardencorev1beta1.MachineImage{
					{
						Name:           "CoreOs",
						UpdateStrategy: &autoUpdateStrategyMinor,
						Versions: []gardencorev1beta1.MachineImageVersion{
							{
								ExpirableVersion: gardencorev1beta1.ExpirableVersion{
									// Shoot's current version
									Version: shootCurrentImageVersion,
								},
								CRI:           []gardencorev1beta1.CRI{{Name: gardencorev1beta1.CRINameDocker}},
								Architectures: []string{"amd64"},
							},
							{
								ExpirableVersion: gardencorev1beta1.ExpirableVersion{
									// highest patch for Shoot's current minor
									Version: highestPatchCurrentMinor,
								},
								CRI:           []gardencorev1beta1.CRI{{Name: gardencorev1beta1.CRINameDocker}},
								Architectures: []string{"amd64"},
							},
							{
								ExpirableVersion: gardencorev1beta1.ExpirableVersion{
									Version: highestVersionForCurrentMajor,
								},
								CRI:           []gardencorev1beta1.CRI{{Name: gardencorev1beta1.CRINameDocker}},
								Architectures: []string{"amd64"},
							},
							{
								ExpirableVersion: gardencorev1beta1.ExpirableVersion{
									// highest version for next major. Don't update to this next major as need to update to latest version in major.
									Version: "3.2.5",
								},
								CRI:           []gardencorev1beta1.CRI{{Name: gardencorev1beta1.CRINameDocker}},
								Architectures: []string{"amd64"},
							},
						},
					},
				}

				_, err := maintainMachineImages(log, shoot, cloudProfile)

				Expect(err).NotTo(HaveOccurred())
				assertWorkerMachineImageVersion(&shoot.Spec.Provider.Workers[0], "CoreOs", highestPatchCurrentMinor)
			})

			It("should auto-update to the latest version for this major. Auto update: not on latest minor.patch version in major", func() {
				autoUpdateStrategyMinor := gardencorev1beta1.UpdateStrategyMinor
				highestVersionForCurrentMajor := "1.5.1"
				cloudProfile.Spec.MachineImages = []gardencorev1beta1.MachineImage{
					{
						Name:           "CoreOs",
						UpdateStrategy: &autoUpdateStrategyMinor,
						Versions: []gardencorev1beta1.MachineImageVersion{
							{
								ExpirableVersion: gardencorev1beta1.ExpirableVersion{
									// Shoot's current version
									Version: shootCurrentImageVersion,
								},
								CRI:           []gardencorev1beta1.CRI{{Name: gardencorev1beta1.CRINameDocker}},
								Architectures: []string{"amd64"},
							},
							{
								ExpirableVersion: gardencorev1beta1.ExpirableVersion{
									// intermediate minor (we skip over)
									Version: "1.3.0",
								},
								CRI:           []gardencorev1beta1.CRI{{Name: gardencorev1beta1.CRINameDocker}},
								Architectures: []string{"amd64"},
							},
							{
								ExpirableVersion: gardencorev1beta1.ExpirableVersion{
									Version: highestVersionForCurrentMajor,
								},
								CRI:           []gardencorev1beta1.CRI{{Name: gardencorev1beta1.CRINameDocker}},
								Architectures: []string{"amd64"},
							},
							{
								ExpirableVersion: gardencorev1beta1.ExpirableVersion{
									// highest version for next major. Don't update to this next major as need to update to latest version in major.
									Version: "3.2.5",
								},
								CRI:           []gardencorev1beta1.CRI{{Name: gardencorev1beta1.CRINameDocker}},
								Architectures: []string{"amd64"},
							},
						},
					},
				}

				_, err := maintainMachineImages(log, shoot, cloudProfile)

				Expect(err).NotTo(HaveOccurred())
				assertWorkerMachineImageVersion(&shoot.Spec.Provider.Workers[0], "CoreOs", highestVersionForCurrentMajor)
			})

			It("should force update to the latest version for this major. Force update: expired version, but not yet not on latest minor.patch version in major", func() {
				autoUpdateStrategyMinor := gardencorev1beta1.UpdateStrategyMinor
				highestVersionForCurrentMajor := "1.5.1"
				cloudProfile.Spec.MachineImages = []gardencorev1beta1.MachineImage{
					{
						Name:           "CoreOs",
						UpdateStrategy: &autoUpdateStrategyMinor,
						Versions: []gardencorev1beta1.MachineImageVersion{
							{
								ExpirableVersion: gardencorev1beta1.ExpirableVersion{
									// Shoot's current version
									Version:        shootCurrentImageVersion,
									ExpirationDate: &expirationDateInThePast,
								},
								CRI:           []gardencorev1beta1.CRI{{Name: gardencorev1beta1.CRINameDocker}},
								Architectures: []string{"amd64"},
							},
							{
								ExpirableVersion: gardencorev1beta1.ExpirableVersion{
									// intermediate minor (we skip over)
									Version: "1.3.0",
								},
								CRI:           []gardencorev1beta1.CRI{{Name: gardencorev1beta1.CRINameDocker}},
								Architectures: []string{"amd64"},
							},
							{
								ExpirableVersion: gardencorev1beta1.ExpirableVersion{
									Version: highestVersionForCurrentMajor,
								},
								CRI:           []gardencorev1beta1.CRI{{Name: gardencorev1beta1.CRINameDocker}},
								Architectures: []string{"amd64"},
							},
							{
								ExpirableVersion: gardencorev1beta1.ExpirableVersion{
									// highest version for next major. Don't update to this next major as need to update to latest version in major.
									Version: "3.2.5",
								},
								CRI:           []gardencorev1beta1.CRI{{Name: gardencorev1beta1.CRINameDocker}},
								Architectures: []string{"amd64"},
							},
						},
					},
				}

				_, err := maintainMachineImages(log, shoot, cloudProfile)

				Expect(err).NotTo(HaveOccurred())
				assertWorkerMachineImageVersion(&shoot.Spec.Provider.Workers[0], "CoreOs", highestVersionForCurrentMajor)
			})

			It("should update to latest minor.patch in next major. Force update: already on latest version for major, but version is expired", func() {
				autoUpdateStrategyMinor := gardencorev1beta1.UpdateStrategyMinor
				latestVersionForCurrentMajor := "1.5.9"
				latestVersionNextMajor := "3.5.9"
				intermediateVersionNextMajor := "3.4.9"
				cloudProfile.Spec.MachineImages = []gardencorev1beta1.MachineImage{
					{
						Name:           "CoreOs",
						UpdateStrategy: &autoUpdateStrategyMinor,
						Versions: []gardencorev1beta1.MachineImageVersion{
							{
								ExpirableVersion: gardencorev1beta1.ExpirableVersion{
									// Shoot's current version
									Version:        latestVersionForCurrentMajor,
									ExpirationDate: &expirationDateInThePast,
								},
								CRI:           []gardencorev1beta1.CRI{{Name: gardencorev1beta1.CRINameDocker}},
								Architectures: []string{"amd64"},
							},
							{
								ExpirableVersion: gardencorev1beta1.ExpirableVersion{
									Version: intermediateVersionNextMajor,
								},
								CRI:           []gardencorev1beta1.CRI{{Name: gardencorev1beta1.CRINameDocker}},
								Architectures: []string{"amd64"},
							},
							{
								ExpirableVersion: gardencorev1beta1.ExpirableVersion{
									Version: latestVersionNextMajor,
								},
								CRI:           []gardencorev1beta1.CRI{{Name: gardencorev1beta1.CRINameDocker}},
								Architectures: []string{"amd64"},
							},
						},
					},
				}

				shoot.Spec.Provider.Workers[0].Machine.Image.Version = &latestVersionForCurrentMajor

				// the shoots patch version is expired and there is no higher non-expired & non-preview patch version of the same minor -> force update
				_, err := maintainMachineImages(log, shoot, cloudProfile)

				Expect(err).NotTo(HaveOccurred())
				assertWorkerMachineImageVersion(&shoot.Spec.Provider.Workers[0], "CoreOs", latestVersionNextMajor)
			})

			It("should force update to latest minor.patch in next major, but next major has no qualifying version (all preview, would update to expired). Hence, skip the next major and force update to major after that", func() {
				// cause force update: expire Shoot's OS version
				cloudProfile.Spec.MachineImages[0].Versions[0].ExpirationDate = &expirationDateInThePast

				// all versions of the next minor are in preview, hence do not qualify for an update (only supported, deprecated and expired versions qualify)
				previewVersionNextMajor := gardencorev1beta1.MachineImageVersion{
					ExpirableVersion: gardencorev1beta1.ExpirableVersion{
						Version:        "2.1.1",
						Classification: &previewClassification,
					},
					Architectures: []string{"amd64"},
					CRI:           []gardencorev1beta1.CRI{{Name: gardencorev1beta1.CRINameDocker}},
				}

				// update to the latest patch version of the major after the next major (skip next minor)
				highestNonPreviewVersionNplusTwoMajor := gardencorev1beta1.MachineImageVersion{
					ExpirableVersion: gardencorev1beta1.ExpirableVersion{
						Version: "3.4.1",
					},
					Architectures: []string{"amd64"},
					CRI:           []gardencorev1beta1.CRI{{Name: gardencorev1beta1.CRINameDocker}},
				}

				versions := []gardencorev1beta1.MachineImageVersion{cloudProfile.Spec.MachineImages[0].Versions[0]}
				versions = append(versions, previewVersionNextMajor)
				versions = append(versions, highestNonPreviewVersionNplusTwoMajor)

				cloudProfile.Spec.MachineImages[0].Versions = versions

				// the shoots patch version is expired and there is no higher non-expired & non-preview patch version of the same minor -> force update
				_, err := maintainMachineImages(log, shoot, cloudProfile)

				Expect(err).NotTo(HaveOccurred())
				assertWorkerMachineImageVersion(&shoot.Spec.Provider.Workers[0], "CoreOs", highestNonPreviewVersionNplusTwoMajor.Version)
			})

			It("Should not downgrade shoot machine image version. All qualifying versions from the CloudProfile for machine image are lower than the Shoot's version (Shoot has the highest version and it is expired).", func() {
				highestExpiredVersion := gardencorev1beta1.MachineImageVersion{
					ExpirableVersion: gardencorev1beta1.ExpirableVersion{
						Version:        "1.7.2",
						Classification: &deprecatedClassification,
						ExpirationDate: &expirationDateInThePast,
					},
				}
				cloudProfile.Spec.MachineImages[0].Versions = append(cloudProfile.Spec.MachineImages[0].Versions, highestExpiredVersion)
				shoot.Spec.Provider.Workers[0].Machine.Image.Version = &highestExpiredVersion.Version
				results, err := maintainMachineImages(log, shoot, cloudProfile)

				Expect(results[shoot.Spec.Provider.Workers[0].Name].isSuccessful).To(BeFalse())
				Expect(err).ToNot(HaveOccurred())
				// make sure still has same version
				assertWorkerMachineImageVersion(&shoot.Spec.Provider.Workers[0], "CoreOs", highestExpiredVersion.Version)
			})

			It("should not change version: already on highest minor.patch for major.", func() {
				autoUpdateStrategyPatch := gardencorev1beta1.UpdateStrategyMinor
				highestVersionForMajor := "1.2.4"
				cloudProfile.Spec.MachineImages = []gardencorev1beta1.MachineImage{
					{
						Name:           "CoreOs",
						UpdateStrategy: &autoUpdateStrategyPatch,
						Versions: []gardencorev1beta1.MachineImageVersion{
							{
								ExpirableVersion: gardencorev1beta1.ExpirableVersion{
									Version: highestVersionForMajor,
								},
								CRI:           []gardencorev1beta1.CRI{{Name: gardencorev1beta1.CRINameDocker}},
								Architectures: []string{"amd64"},
							},
							{
								ExpirableVersion: gardencorev1beta1.ExpirableVersion{
									// highest version for next major. Don't update to this next major.
									Version: "2.2.5",
								},
								CRI:           []gardencorev1beta1.CRI{{Name: gardencorev1beta1.CRINameDocker}},
								Architectures: []string{"amd64"},
							},
						},
					},
				}

				shoot.Spec.Provider.Workers[0].Machine.Image.Version = &highestVersionForMajor
				_, err := maintainMachineImages(log, shoot, cloudProfile)

				Expect(err).NotTo(HaveOccurred())
				assertWorkerMachineImageVersion(&shoot.Spec.Provider.Workers[0], "CoreOs", highestVersionForMajor)
			})
		})

		It("should treat workers with `cri: nil` like `cri.name: docker` and not update if `docker` is not explicitly supported by the machine image", func() {
			cloudProfile.Spec.MachineImages[0].Versions[1].CRI = []gardencorev1beta1.CRI{{Name: gardencorev1beta1.CRINameContainerD}}

			_, err := maintainMachineImages(log, shoot, cloudProfile)
			Expect(err).NotTo(HaveOccurred())
			assertWorkerMachineImageVersion(&shoot.Spec.Provider.Workers[0], "CoreOs", "1.0.0")
		})

		It("should determine that the shoot worker machine images must NOT to be maintained - ForceUpdate not required & MaintenanceAutoUpdate set to false", func() {
			shoot.Spec.Maintenance.AutoUpdate.MachineImageVersion = pointer.Bool(false)

			expected := shoot.Spec.Provider.Workers[0].Machine.Image.DeepCopy()
			_, err := maintainMachineImages(log, shoot, cloudProfile)

			Expect(err).NotTo(HaveOccurred())
			Expect(shoot.Spec.Provider.Workers[0].Machine.Image).To(Equal(expected))
		})

		It("should determine that the shoot worker machine images must NOT be maintained - found no machineImageVersion with matching CRI", func() {
			cloudProfile.Spec.MachineImages[0].Versions[0].CRI = []gardencorev1beta1.CRI{{Name: gardencorev1beta1.CRINameContainerD}}
			shoot.Spec.Provider.Workers[0].CRI = &gardencorev1beta1.CRI{Name: gardencorev1beta1.CRINameContainerD}

			expected := shoot.Spec.Provider.Workers[0].Machine.Image.DeepCopy()
			_, err := maintainMachineImages(log, shoot, cloudProfile)
			Expect(err).NotTo(HaveOccurred())
			Expect(shoot.Spec.Provider.Workers[0].Machine.Image).To(Equal(expected))
		})

		It("should determine that some shoot worker machine images must be not be maintained - MachineImageVersion doesn't support certain CRIs", func() {
			// only the shoots current os image contains the containerd CRI (none of the other versions do) -> this worker pool must not be updated
			cloudProfile.Spec.MachineImages[0].Versions[0].CRI = []gardencorev1beta1.CRI{{Name: gardencorev1beta1.CRINameContainerD}}
			shoot.Spec.Provider.Workers[0].CRI = &gardencorev1beta1.CRI{Name: gardencorev1beta1.CRINameContainerD}

			// add another pool without CRI constraints -> should be updated via auto-update
			shoot.Spec.Provider.Workers = append(shoot.Spec.Provider.Workers, gardencorev1beta1.Worker{Name: "worker-without-cri-config", Machine: gardencorev1beta1.Machine{Image: shootCurrentImage.DeepCopy(), Architecture: pointer.String("amd64")}})

			_, err := maintainMachineImages(log, shoot, cloudProfile)
			Expect(err).NotTo(HaveOccurred())

			assertWorkerMachineImageVersion(&shoot.Spec.Provider.Workers[0], "CoreOs", "1.0.0")
			assertWorkerMachineImageVersion(&shoot.Spec.Provider.Workers[1], "CoreOs", overallLatestVersion)
		})

		It("should determine that some shoot worker machine images must be not be maintained - MachineImageVersion does support CRI.Name but does not support certain containerruntime", func() {
			cloudProfile.Spec.MachineImages[0].Versions[0].CRI = []gardencorev1beta1.CRI{{Name: gardencorev1beta1.CRINameContainerD, ContainerRuntimes: []gardencorev1beta1.ContainerRuntime{{Type: "gvisor"}}}}
			cloudProfile.Spec.MachineImages[0].Versions[1].CRI = []gardencorev1beta1.CRI{{Name: gardencorev1beta1.CRINameContainerD}}
			// shoot's pool 1 image version requires gvisor. Only the current os image supports gvisor, hence the pool must not be updated.
			shoot.Spec.Provider.Workers[0].CRI = &gardencorev1beta1.CRI{Name: gardencorev1beta1.CRINameContainerD, ContainerRuntimes: []gardencorev1beta1.ContainerRuntime{{Type: "gvisor"}}}

			// add another pool without CRI constraints -> should be updated via auto-update to the highest patch version of the same minor
			shoot.Spec.Provider.Workers = append(shoot.Spec.Provider.Workers, gardencorev1beta1.Worker{Name: "worker-without-containerruntime", CRI: &gardencorev1beta1.CRI{Name: gardencorev1beta1.CRINameContainerD}, Machine: gardencorev1beta1.Machine{Image: shootCurrentImage.DeepCopy(), Architecture: pointer.String("amd64")}})

			_, err := maintainMachineImages(log, shoot, cloudProfile)
			Expect(err).NotTo(HaveOccurred())

			assertWorkerMachineImageVersion(&shoot.Spec.Provider.Workers[0], "CoreOs", "1.0.0")
			assertWorkerMachineImageVersion(&shoot.Spec.Provider.Workers[1], "CoreOs", overallLatestVersion)
		})

		It("should determine that some shoot worker machine images must be not be maintained - MachineImageVersion does not support containerruntime", func() {
			cloudProfile.Spec.MachineImages[0].Versions[0].CRI = []gardencorev1beta1.CRI{{Name: gardencorev1beta1.CRINameContainerD, ContainerRuntimes: []gardencorev1beta1.ContainerRuntime{{Type: "gvisor"}, {Type: "kata-container"}, {Type: "some-other-cr"}}}}
			cloudProfile.Spec.MachineImages[0].Versions[1].CRI = []gardencorev1beta1.CRI{{Name: gardencorev1beta1.CRINameContainerD, ContainerRuntimes: []gardencorev1beta1.ContainerRuntime{{Type: "gvisor"}, {Type: "kata-container"}}}}

			shoot.Spec.Provider.Workers[0].CRI = &gardencorev1beta1.CRI{Name: gardencorev1beta1.CRINameContainerD, ContainerRuntimes: []gardencorev1beta1.ContainerRuntime{{Type: "gvisor"}, {Type: "kata-container"}, {Type: "some-other-cr"}}}
			shoot.Spec.Provider.Workers = append(shoot.Spec.Provider.Workers, gardencorev1beta1.Worker{Name: "worker-with-gvisor-and-kata", CRI: &gardencorev1beta1.CRI{Name: gardencorev1beta1.CRINameContainerD, ContainerRuntimes: []gardencorev1beta1.ContainerRuntime{{Type: "gvisor"}, {Type: "kata-container"}}}, Machine: gardencorev1beta1.Machine{Image: shootCurrentImage.DeepCopy(), Architecture: pointer.String("amd64")}})
			shoot.Spec.Provider.Workers = append(shoot.Spec.Provider.Workers, gardencorev1beta1.Worker{Name: "worker-with-gvisor", CRI: &gardencorev1beta1.CRI{Name: gardencorev1beta1.CRINameContainerD, ContainerRuntimes: []gardencorev1beta1.ContainerRuntime{{Type: "gvisor"}}}, Machine: gardencorev1beta1.Machine{Image: shootCurrentImage.DeepCopy(), Architecture: pointer.String("amd64")}})

			_, err := maintainMachineImages(log, shoot, cloudProfile)
			Expect(err).NotTo(HaveOccurred())

			assertWorkerMachineImageVersion(&shoot.Spec.Provider.Workers[0], "CoreOs", "1.0.0")
			assertWorkerMachineImageVersion(&shoot.Spec.Provider.Workers[1], "CoreOs", overallLatestVersion)
			assertWorkerMachineImageVersion(&shoot.Spec.Provider.Workers[2], "CoreOs", overallLatestVersion)
		})

		It("should determine that the shoot worker machine images must not be maintained - found no machineImageVersion with matching kubeletVersionConstraint (control plane K8s version)", func() {
			cloudProfile.Spec.MachineImages[0].Versions[1].KubeletVersionConstraint = pointer.String("< 1.26")
			shoot.Spec.Kubernetes.Version = "1.26.0"

			expected := shoot.Spec.Provider.Workers[0].Machine.Image.DeepCopy()
			_, err := maintainMachineImages(log, shoot, cloudProfile)
			Expect(err).NotTo(HaveOccurred())
			Expect(shoot.Spec.Provider.Workers[0].Machine.Image).To(Equal(expected))
		})

		It("should determine that the shoot worker machine images must be maintained - found machineImageVersion with matching kubeletVersionConstraint (control plane K8s version)", func() {
			cloudProfile.Spec.MachineImages[0].Versions[1].KubeletVersionConstraint = pointer.String("< 1.26")
			shoot.Spec.Kubernetes.Version = "1.25.1"

			_, err := maintainMachineImages(log, shoot, cloudProfile)
			Expect(err).NotTo(HaveOccurred())
			assertWorkerMachineImageVersion(&shoot.Spec.Provider.Workers[0], "CoreOs", overallLatestVersion)
		})

		It("should determine that the shoot worker machine images must not be maintained - found no machineImageVersion with matching kubeletVersionConstraint (worker K8s version)", func() {
			cloudProfile.Spec.MachineImages[0].Versions[1].KubeletVersionConstraint = pointer.String(">= 1.26")
			shoot.Spec.Kubernetes.Version = "1.26.0"
			shoot.Spec.Provider.Workers[0].Kubernetes = &gardencorev1beta1.WorkerKubernetes{
				Version: pointer.String("1.25.0"),
			}

			expected := shoot.Spec.Provider.Workers[0].Machine.Image.DeepCopy()
			_, err := maintainMachineImages(log, shoot, cloudProfile)
			Expect(err).NotTo(HaveOccurred())
			Expect(shoot.Spec.Provider.Workers[0].Machine.Image).To(Equal(expected))
		})

		It("should determine that the shoot worker machine images must be maintained - found machineImageVersion with matching kubeletVersionConstraint (worker K8s version)", func() {
			assertWorkerMachineImageVersion(&shoot.Spec.Provider.Workers[0], "CoreOs", "1.0.0")
			cloudProfile.Spec.MachineImages[0].Versions[1].KubeletVersionConstraint = pointer.String(">= 1.26")
			shoot.Spec.Kubernetes.Version = "1.27.0"
			shoot.Spec.Provider.Workers[0].Kubernetes = &gardencorev1beta1.WorkerKubernetes{
				Version: pointer.String("1.26.0"),
			}

			_, err := maintainMachineImages(log, shoot, cloudProfile)
			Expect(err).NotTo(HaveOccurred())
			assertWorkerMachineImageVersion(&shoot.Spec.Provider.Workers[0], "CoreOs", cloudProfile.Spec.MachineImages[0].Versions[1].Version)
		})

		It("should return an error - cloud profile has no matching (machineImage.name) machine image defined", func() {
			cloudProfile.Spec.MachineImages = cloudProfile.Spec.MachineImages[1:]

			_, err := maintainMachineImages(log, shoot, cloudProfile)

			Expect(err).To(HaveOccurred())
		})
	})

	Describe("#maintainKubernetesVersion", func() {
		var (
			cloudProfile          *gardencorev1beta1.CloudProfile
			shoot                 *gardencorev1beta1.Shoot
			kubernetesSettings    gardencorev1beta1.KubernetesSettings
			previewClassification = gardencorev1beta1.ClassificationPreview
		)

		BeforeEach(func() {
			kubernetesSettings = gardencorev1beta1.KubernetesSettings{
				Versions: []gardencorev1beta1.ExpirableVersion{
					{
						Version: "1.1.2",
					},
					{
						Version: "1.1.1",
					},
					{
						Version: "1.1.0",
					},
					{
						Version: "1.0.2",
					},
					{
						Version: "1.0.1",
					},
					{
						Version:        "1.0.0",
						ExpirationDate: &expirationDateInTheFuture,
					},
					{
						Version: "2.0.0",
					},
				},
			}
			cloudProfile = &gardencorev1beta1.CloudProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name: "profile",
				},
				Spec: gardencorev1beta1.CloudProfileSpec{
					Kubernetes: kubernetesSettings,
				},
			}

			shoot = &gardencorev1beta1.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Name: "shoot",
				},
				Spec: gardencorev1beta1.ShootSpec{
					Maintenance: &gardencorev1beta1.Maintenance{
						AutoUpdate: &gardencorev1beta1.MaintenanceAutoUpdate{
							KubernetesVersion: true,
						},
					},
					Kubernetes: gardencorev1beta1.Kubernetes{Version: "1.0.0"},
				},
			}
		})
		It("should determine that the shoot kubernetes version must be maintained - ForceUpdate to latest patch version", func() {
			shoot.Spec.Maintenance.AutoUpdate.KubernetesVersion = false
			cloudProfile.Spec.Kubernetes.Versions[4].ExpirationDate = &expirationDateInThePast
			shoot.Spec.Kubernetes = gardencorev1beta1.Kubernetes{Version: "1.0.1"}

			_, err := maintainKubernetesVersion(log, shoot.Spec.Kubernetes.Version, shoot.Spec.Maintenance.AutoUpdate.KubernetesVersion, cloudProfile, func(v string) error {
				shoot.Spec.Kubernetes.Version = v
				return nil
			})

			Expect(err).NotTo(HaveOccurred())
			Expect(shoot.Spec.Kubernetes.Version).To(Equal("1.0.2"))
		})

		It("should determine that the shoot kubernetes version must be maintained - ForceUpdate to latest non-preview patch version", func() {
			shoot.Spec.Maintenance.AutoUpdate.KubernetesVersion = false
			// expire shoots kubernetes version 1.0.0
			cloudProfile.Spec.Kubernetes.Versions[5].ExpirationDate = &expirationDateInThePast
			shoot.Spec.Kubernetes = gardencorev1beta1.Kubernetes{Version: "1.0.0"}

			// mark latest version 1.02 as preview
			cloudProfile.Spec.Kubernetes.Versions[3].Classification = &previewClassification

			_, err := maintainKubernetesVersion(log, shoot.Spec.Kubernetes.Version, shoot.Spec.Maintenance.AutoUpdate.KubernetesVersion, cloudProfile, func(v string) error {
				shoot.Spec.Kubernetes.Version = v
				return nil
			})

			Expect(err).To(Not(HaveOccurred()))
			Expect(shoot.Spec.Kubernetes.Version).To(Equal("1.0.1"))
		})

		It("should determine that the shoot kubernetes version must be maintained - ForceUpdate to latest qualifying patch version of next minor version", func() {
			shoot.Spec.Maintenance.AutoUpdate.KubernetesVersion = false
			cloudProfile.Spec.Kubernetes.Versions[3].ExpirationDate = &expirationDateInThePast
			shoot.Spec.Kubernetes = gardencorev1beta1.Kubernetes{Version: "1.0.2"}

			_, err := maintainKubernetesVersion(log, shoot.Spec.Kubernetes.Version, shoot.Spec.Maintenance.AutoUpdate.KubernetesVersion, cloudProfile, func(v string) error {
				shoot.Spec.Kubernetes.Version = v
				return nil
			})

			Expect(err).NotTo(HaveOccurred())
			Expect(shoot.Spec.Kubernetes.Version).To(Equal("1.1.2"))
		})

		It("should determine that the shoot kubernetes version must be maintained - ForceUpdate to latest qualifying patch version of next minor version", func() {
			shoot.Spec.Maintenance.AutoUpdate.KubernetesVersion = true
			cloudProfile.Spec.Kubernetes.Versions[3].ExpirationDate = &expirationDateInThePast
			shoot.Spec.Kubernetes = gardencorev1beta1.Kubernetes{Version: "1.0.2"}

			_, err := maintainKubernetesVersion(log, shoot.Spec.Kubernetes.Version, shoot.Spec.Maintenance.AutoUpdate.KubernetesVersion, cloudProfile, func(v string) error {
				shoot.Spec.Kubernetes.Version = v
				return nil
			})

			Expect(err).NotTo(HaveOccurred())
			Expect(shoot.Spec.Kubernetes.Version).To(Equal("1.1.2"))
		})

		// special case when all the patch versions of the consecutive minor versions are expired
		It("should determine that the shoot kubernetes version must be maintained - ForceUpdate to latest qualifying patch version (is expired) of next minor version.", func() {
			shoot.Spec.Maintenance.AutoUpdate.KubernetesVersion = false
			// expire version 1.0.2
			cloudProfile.Spec.Kubernetes.Versions[3].ExpirationDate = &expirationDateInThePast
			shoot.Spec.Kubernetes = gardencorev1beta1.Kubernetes{Version: "1.0.2"}

			// expire all the version of the consecutive minor version
			cloudProfile.Spec.Kubernetes.Versions[0].ExpirationDate = &expirationDateInThePast
			cloudProfile.Spec.Kubernetes.Versions[1].ExpirationDate = &expirationDateInThePast
			cloudProfile.Spec.Kubernetes.Versions[2].ExpirationDate = &expirationDateInThePast

			_, err := maintainKubernetesVersion(log, shoot.Spec.Kubernetes.Version, shoot.Spec.Maintenance.AutoUpdate.KubernetesVersion, cloudProfile, func(v string) error {
				shoot.Spec.Kubernetes.Version = v
				return nil
			})

			Expect(err).NotTo(HaveOccurred())
			Expect(shoot.Spec.Kubernetes.Version).To(Equal("1.1.2"))
		})

		It("should determine that the shoot kubernetes version must be maintained - however the ForceUpdate is impossible (only preview version available)", func() {
			shoot.Spec.Maintenance.AutoUpdate.KubernetesVersion = false
			cloudProfile.Spec.Kubernetes.Versions[0].Classification = &previewClassification
			cloudProfile.Spec.Kubernetes.Versions[1].Classification = &previewClassification
			cloudProfile.Spec.Kubernetes.Versions[2].Classification = &previewClassification

			cloudProfile.Spec.Kubernetes.Versions[3].ExpirationDate = &expirationDateInThePast
			shoot.Spec.Kubernetes = gardencorev1beta1.Kubernetes{Version: "1.0.2"}

			_, err := maintainKubernetesVersion(log, shoot.Spec.Kubernetes.Version, shoot.Spec.Maintenance.AutoUpdate.KubernetesVersion, cloudProfile, func(v string) error {
				shoot.Spec.Kubernetes.Version = v
				return nil
			})

			Expect(err).To(HaveOccurred())
		})

		It("should determine that the shoot kubernetes version must be maintained - MaintenanceAutoUpdate set to true", func() {
			shoot.Spec.Maintenance.AutoUpdate.KubernetesVersion = true
			shoot.Spec.Kubernetes = gardencorev1beta1.Kubernetes{Version: "1.0.1"}

			_, err := maintainKubernetesVersion(log, shoot.Spec.Kubernetes.Version, shoot.Spec.Maintenance.AutoUpdate.KubernetesVersion, cloudProfile, func(v string) error {
				shoot.Spec.Kubernetes.Version = v
				return nil
			})

			Expect(err).NotTo(HaveOccurred())
			Expect(shoot.Spec.Kubernetes.Version).To(Equal("1.0.2"))
		})

		It("should determine that the kubernetes version must NOT to be maintained - ForceUpdate not required & MaintenanceAutoUpdate set to false", func() {
			shoot.Spec.Maintenance.AutoUpdate.KubernetesVersion = false
			cloudProfile.Spec.Kubernetes.Versions[4].ExpirationDate = &expirationDateInTheFuture
			shoot.Spec.Kubernetes = gardencorev1beta1.Kubernetes{Version: "1.0.1"}

			_, err := maintainKubernetesVersion(log, shoot.Spec.Kubernetes.Version, shoot.Spec.Maintenance.AutoUpdate.KubernetesVersion, cloudProfile, func(v string) error {
				shoot.Spec.Kubernetes.Version = v
				return nil
			})

			Expect(err).NotTo(HaveOccurred())
			Expect(shoot.Spec.Kubernetes.Version).To(Equal("1.0.1"))
		})

		It("should determine that the shootKubernetes version must be maintained - cloud profile has no matching kubernetes version defined (the shoots kubernetes version has been deleted from the cloudProfile) -> update to latest kubernetes patch version with same minor", func() {
			cloudProfile.Spec.Kubernetes.Versions = kubernetesSettings.Versions[:4]
			shoot.Spec.Maintenance.AutoUpdate.KubernetesVersion = true
			shoot.Spec.Kubernetes = gardencorev1beta1.Kubernetes{Version: "1.0.0"}

			_, err := maintainKubernetesVersion(log, shoot.Spec.Kubernetes.Version, shoot.Spec.Maintenance.AutoUpdate.KubernetesVersion, cloudProfile, func(v string) error {
				shoot.Spec.Kubernetes.Version = v
				return nil
			})

			Expect(err).NotTo(HaveOccurred())
			Expect(shoot.Spec.Kubernetes.Version).To(Equal("1.0.2"))
		})

		It("should determine that the shootKubernetes version must be maintained - cloud profile has no matching kubernetes version defined (the shoots kubernetes version has been deleted from the cloudProfile) && isLatest patch version for minor-> update to latest kubernetes patch version for next minor", func() {
			cloudProfile.Spec.Kubernetes.Versions = kubernetesSettings.Versions[:2]
			shoot.Spec.Maintenance.AutoUpdate.KubernetesVersion = true
			shoot.Spec.Kubernetes = gardencorev1beta1.Kubernetes{Version: "1.0.2"}

			_, err := maintainKubernetesVersion(log, shoot.Spec.Kubernetes.Version, shoot.Spec.Maintenance.AutoUpdate.KubernetesVersion, cloudProfile, func(v string) error {
				shoot.Spec.Kubernetes.Version = v
				return nil
			})

			Expect(err).NotTo(HaveOccurred())
			Expect(shoot.Spec.Kubernetes.Version).To(Equal("1.1.2"))
		})

		It("do not update major Kubernetes version", func() {
			shoot.Spec.Maintenance.AutoUpdate.KubernetesVersion = false
			cloudProfile.Spec.Kubernetes.Versions[3].ExpirationDate = &expirationDateInThePast
			shoot.Spec.Kubernetes = gardencorev1beta1.Kubernetes{Version: "1.1.2"}

			_, err := maintainKubernetesVersion(log, shoot.Spec.Kubernetes.Version, shoot.Spec.Maintenance.AutoUpdate.KubernetesVersion, cloudProfile, func(v string) error {
				shoot.Spec.Kubernetes.Version = v
				return nil
			})

			Expect(err).NotTo(HaveOccurred())
			Expect(shoot.Spec.Kubernetes.Version).To(Equal("1.1.2"))
		})
	})

	Describe("#DisablePodSecurityPolicyAdmissionController", func() {
		var (
			shoot                             *gardencorev1beta1.Shoot
			policyAdmissionControllerDisabled gardencorev1beta1.AdmissionPlugin
			foobarAdmissionPlugin             gardencorev1beta1.AdmissionPlugin
		)

		BeforeEach(func() {
			policyAdmissionControllerDisabled = gardencorev1beta1.AdmissionPlugin{
				Name:     "PodSecurityPolicy",
				Disabled: pointer.Bool(true),
			}
			foobarAdmissionPlugin = gardencorev1beta1.AdmissionPlugin{
				Name:     "foobar",
				Disabled: pointer.Bool(true),
			}

			shoot = &gardencorev1beta1.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Name: "shoot",
				},
				Spec: gardencorev1beta1.ShootSpec{},
			}
		})

		It("should not change anything if PodSecurityPolicy admission controller is already disabled", func() {
			shoot.Spec.Kubernetes.KubeAPIServer = &gardencorev1beta1.KubeAPIServerConfig{AdmissionPlugins: []gardencorev1beta1.AdmissionPlugin{policyAdmissionControllerDisabled}}
			result := disablePodSecurityPolicyAdmissionController(shoot, "foobar")
			Expect(result).To(HaveLen(0))
			Expect(shoot.Spec.Kubernetes.KubeAPIServer.AdmissionPlugins).To(ConsistOf(policyAdmissionControllerDisabled))
		})

		It("should disable PodSecurityPolicy admission controller if there is no KubeAPIServer configuration", func() {
			result := disablePodSecurityPolicyAdmissionController(shoot, "foobar")
			Expect(result).To(HaveLen(1))
			Expect(shoot.Spec.Kubernetes.KubeAPIServer.AdmissionPlugins).To(ConsistOf(policyAdmissionControllerDisabled))
		})

		It("should disable PodSecurityPolicy admission controller if there are admission plugins in KubeAPIServer configuration", func() {
			shoot.Spec.Kubernetes.KubeAPIServer = &gardencorev1beta1.KubeAPIServerConfig{AdmissionPlugins: []gardencorev1beta1.AdmissionPlugin{foobarAdmissionPlugin}}
			result := disablePodSecurityPolicyAdmissionController(shoot, "foobar")
			Expect(result).To(HaveLen(1))
			Expect(shoot.Spec.Kubernetes.KubeAPIServer.AdmissionPlugins).To(ConsistOf(policyAdmissionControllerDisabled, foobarAdmissionPlugin))
		})

		It("should disable PodSecurityPolicy admission controller if it was enabled before", func() {
			shoot.Spec.Kubernetes.KubeAPIServer = &gardencorev1beta1.KubeAPIServerConfig{AdmissionPlugins: []gardencorev1beta1.AdmissionPlugin{foobarAdmissionPlugin, {Name: "PodSecurityPolicy", Disabled: pointer.Bool(false)}}}
			result := disablePodSecurityPolicyAdmissionController(shoot, "foobar")
			Expect(result).To(HaveLen(1))
			Expect(shoot.Spec.Kubernetes.KubeAPIServer.AdmissionPlugins).To(ConsistOf(policyAdmissionControllerDisabled, foobarAdmissionPlugin))
		})
	})

	Describe("#EnsureSufficientMaxWorkers", func() {
		var (
			shoot *gardencorev1beta1.Shoot
		)

		BeforeEach(func() {
			shoot = &gardencorev1beta1.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Name: "shoot",
				},
				Spec: gardencorev1beta1.ShootSpec{
					Provider: gardencorev1beta1.Provider{Workers: []gardencorev1beta1.Worker{
						{
							Name:    "cpu-worker",
							Maximum: 3,
							Zones:   []string{"fooZone", "barZone"},
						},
						{
							Name:    "cpu-worker2",
							Maximum: 1,
							Zones:   []string{"fooZone"},
						},
					}},
				},
			}
		})

		It("should not change worker groups which do not allow system components", func() {
			shoot.Spec.Provider.Workers[1].SystemComponents = &gardencorev1beta1.WorkerSystemComponents{Allow: false}
			shoot.Spec.Provider.Workers[1].Zones = append(shoot.Spec.Provider.Workers[1].Zones, "barZone")
			result := ensureSufficientMaxWorkers(shoot, "foobar")
			Expect(result).To(HaveLen(0))
			Expect(shoot.Spec.Provider.Workers[0].Maximum).To(Equal(int32(3)))
			Expect(shoot.Spec.Provider.Workers[1].Maximum).To(Equal(int32(1)))
		})

		It("should not change anything if the maximum workers are high enough", func() {
			result := ensureSufficientMaxWorkers(shoot, "foobar")
			Expect(result).To(HaveLen(0))
			Expect(shoot.Spec.Provider.Workers[0].Maximum).To(Equal(int32(3)))
			Expect(shoot.Spec.Provider.Workers[1].Maximum).To(Equal(int32(1)))
		})

		It("should increase values if there are more zones than maximum workers", func() {
			shoot.Spec.Provider.Workers[1].Zones = append(shoot.Spec.Provider.Workers[1].Zones, "barZone")
			result := ensureSufficientMaxWorkers(shoot, "foobar")
			Expect(result).To(HaveLen(1))
			Expect(shoot.Spec.Provider.Workers[0].Maximum).To(Equal(int32(3)))
			Expect(shoot.Spec.Provider.Workers[1].Maximum).To(Equal(int32(2)))
		})
	})
})

func assertWorkerMachineImageVersion(worker *gardencorev1beta1.Worker, imageName string, imageVersion string) {
	ExpectWithOffset(1, worker.Machine.Image.Name).To(Equal(imageName))
	ExpectWithOffset(1, *worker.Machine.Image.Version).To(Equal(imageVersion))
}
