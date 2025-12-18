// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package maintenance

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	securityv1alpha1 "github.com/gardener/gardener/pkg/apis/security/v1alpha1"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/test"
	admissionpluginsvalidation "github.com/gardener/gardener/pkg/utils/validation/admissionplugins"
	featuresvalidation "github.com/gardener/gardener/pkg/utils/validation/features"
	"github.com/gardener/gardener/pkg/utils/version"
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

	Describe("#maintainMachineImages", func() {
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
				Version: ptr.To(shootCurrentImageVersion),
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
									CRI:           []gardencorev1beta1.CRI{{Name: gardencorev1beta1.CRINameContainerD}},
									Architectures: []string{"amd64"},
								},
								{
									ExpirableVersion: gardencorev1beta1.ExpirableVersion{
										Version:        overallLatestVersion,
										ExpirationDate: &expirationDateInTheFuture,
									},
									CRI:           []gardencorev1beta1.CRI{{Name: gardencorev1beta1.CRINameContainerD}},
									Architectures: []string{"amd64"},
								},
								{
									ExpirableVersion: gardencorev1beta1.ExpirableVersion{
										Version: shootCurrentImageVersion + "-inplace",
									},
									CRI:           []gardencorev1beta1.CRI{{Name: gardencorev1beta1.CRINameContainerD}},
									Architectures: []string{"amd64"},
									InPlaceUpdates: &gardencorev1beta1.InPlaceUpdates{
										Supported: true,
									},
								},
								{
									ExpirableVersion: gardencorev1beta1.ExpirableVersion{
										Version:        overallLatestVersion + "-inplace",
										ExpirationDate: &expirationDateInTheFuture,
									},
									CRI:           []gardencorev1beta1.CRI{{Name: gardencorev1beta1.CRINameContainerD}},
									Architectures: []string{"amd64"},
									InPlaceUpdates: &gardencorev1beta1.InPlaceUpdates{
										Supported:           true,
										MinVersionForUpdate: ptr.To(shootCurrentImageVersion + "-inplace"),
									},
								},
							},
						},
					},
					MachineTypes: []gardencorev1beta1.MachineType{
						{Name: "someMachineType"},
						{Name: ""},
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
							MachineImageVersion: ptr.To(true),
						},
					},
					Provider: gardencorev1beta1.Provider{Workers: []gardencorev1beta1.Worker{
						{
							Name: "cpu-worker",
							Machine: gardencorev1beta1.Machine{
								Image:        shootCurrentImage,
								Architecture: ptr.To("amd64"),
								Type:         "someMachineType",
							},
							UpdateStrategy: ptr.To(gardencorev1beta1.AutoRollingUpdate),
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

			It("should update machine image version to overall latest. Auto update: already on latest patch for minor, and there is an overall higher version available for in-place updates", func() {
				shoot.Spec.Provider.Workers[0].Machine.Image.Version = ptr.To(shootCurrentImageVersion + "-inplace")
				shoot.Spec.Provider.Workers[0].UpdateStrategy = ptr.To(gardencorev1beta1.AutoInPlaceUpdate)
				_, err := maintainMachineImages(log, shoot, cloudProfile)

				Expect(err).NotTo(HaveOccurred())
				assertWorkerMachineImageVersion(&shoot.Spec.Provider.Workers[0], "CoreOs", overallLatestVersion+"-inplace")
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
					CRI:           []gardencorev1beta1.CRI{{Name: gardencorev1beta1.CRINameContainerD}},
					Architectures: []string{"arm64"},
				})

				// add overall higher version with wrong architecture (should be ignored)
				cloudProfile.Spec.MachineImages[0].Versions = append(cloudProfile.Spec.MachineImages[0].Versions, gardencorev1beta1.MachineImageVersion{
					ExpirableVersion: gardencorev1beta1.ExpirableVersion{
						Version:        "1.7.1",
						ExpirationDate: &expirationDateInTheFuture,
					},
					CRI:           []gardencorev1beta1.CRI{{Name: gardencorev1beta1.CRINameContainerD}},
					Architectures: []string{"amd64"},
				})

				shoot.Spec.Provider.Workers[0].Machine.Architecture = ptr.To("arm64")

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
							CRI:           []gardencorev1beta1.CRI{{Name: gardencorev1beta1.CRINameContainerD}},
							Architectures: []string{"amd64"},
						},
						{
							ExpirableVersion: gardencorev1beta1.ExpirableVersion{
								Version: expectedVersionGLWorker,
							},
							CRI:           []gardencorev1beta1.CRI{{Name: gardencorev1beta1.CRINameContainerD}},
							Architectures: []string{"amd64"},
						},
					},
				})

				otherWorker := gardencorev1beta1.Worker{
					Name: "cpu-glinux",
					Machine: gardencorev1beta1.Machine{
						Image: &gardencorev1beta1.ShootMachineImage{
							Name:    "gardenlinux",
							Version: ptr.To("1.0.0"),
						},
						Architecture: ptr.To("amd64"),
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
					CRI:           []gardencorev1beta1.CRI{{Name: gardencorev1beta1.CRINameContainerD}},
				}

				cloudProfile.Spec.MachineImages[0].Versions = append(cloudProfile.Spec.MachineImages[0].Versions, highestForMinor)

				_, err := maintainMachineImages(log, shoot, cloudProfile)

				Expect(err).NotTo(HaveOccurred())
				assertWorkerMachineImageVersion(&shoot.Spec.Provider.Workers[0], "CoreOs", expectedVersion)
			})

			It("should update machine image version to overall latest. ForceUpdate: expiration date in the past", func() {
				shoot.Spec.Maintenance.AutoUpdate.MachineImageVersion = ptr.To(false)
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
								CRI:           []gardencorev1beta1.CRI{{Name: gardencorev1beta1.CRINameContainerD}},
								Architectures: []string{"amd64"},
							},
							{
								ExpirableVersion: gardencorev1beta1.ExpirableVersion{
									Version:        expectedVersion,
									ExpirationDate: &expirationDateInTheFuture,
								},
								CRI:           []gardencorev1beta1.CRI{{Name: gardencorev1beta1.CRINameContainerD}},
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
					CRI:           []gardencorev1beta1.CRI{{Name: gardencorev1beta1.CRINameContainerD}},
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
								CRI:           []gardencorev1beta1.CRI{{Name: gardencorev1beta1.CRINameContainerD}},
								Architectures: []string{"amd64"},
							},
							{
								ExpirableVersion: gardencorev1beta1.ExpirableVersion{
									Version:        expectedVersion,
									ExpirationDate: &expirationDateInTheFuture,
								},
								CRI:           []gardencorev1beta1.CRI{{Name: gardencorev1beta1.CRINameContainerD}},
								Architectures: []string{"amd64"},
							},
							{
								ExpirableVersion: gardencorev1beta1.ExpirableVersion{
									Version:        "1.2.0",
									ExpirationDate: &expirationDateInTheFuture,
								},
								CRI:           []gardencorev1beta1.CRI{{Name: gardencorev1beta1.CRINameContainerD}},
								Architectures: []string{"amd64"},
							},
							{
								ExpirableVersion: gardencorev1beta1.ExpirableVersion{
									Version:        shootCurrentImageVersion,
									ExpirationDate: &expirationDateInThePast,
								},
								CRI:           []gardencorev1beta1.CRI{{Name: gardencorev1beta1.CRINameContainerD}},
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
								CRI:           []gardencorev1beta1.CRI{{Name: gardencorev1beta1.CRINameContainerD}},
								Architectures: []string{"amd64"},
							},
							{
								ExpirableVersion: gardencorev1beta1.ExpirableVersion{
									// current version 1.0.0, next minor in cloudprofile: 1.2.X
									Version:        highestPatchNextMinor,
									ExpirationDate: &expirationDateInTheFuture,
								},
								CRI:           []gardencorev1beta1.CRI{{Name: gardencorev1beta1.CRINameContainerD}},
								Architectures: []string{"amd64"},
							},
							{
								ExpirableVersion: gardencorev1beta1.ExpirableVersion{
									Version:        lowerPatchNextMinor,
									ExpirationDate: &expirationDateInTheFuture,
								},
								CRI:           []gardencorev1beta1.CRI{{Name: gardencorev1beta1.CRINameContainerD}},
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
					CRI:           []gardencorev1beta1.CRI{{Name: gardencorev1beta1.CRINameContainerD}},
				}

				// update to the latest patch version of the minor after the next minor (skip next minor)
				highestNonPreviewPatchVersionNplusTwoMinor := gardencorev1beta1.MachineImageVersion{
					ExpirableVersion: gardencorev1beta1.ExpirableVersion{
						Version: "1.4.1",
					},
					Architectures: []string{"amd64"},
					CRI:           []gardencorev1beta1.CRI{{Name: gardencorev1beta1.CRINameContainerD}},
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
					CRI:           []gardencorev1beta1.CRI{{Name: gardencorev1beta1.CRINameContainerD}},
				}
				expiredPatchVersionNextMinor := gardencorev1beta1.MachineImageVersion{
					ExpirableVersion: gardencorev1beta1.ExpirableVersion{
						Version:        lowerPatchNextMinor,
						ExpirationDate: &expirationDateInThePast,
					},
					Architectures: []string{"amd64"},
					CRI:           []gardencorev1beta1.CRI{{Name: gardencorev1beta1.CRINameContainerD}},
				}

				// do not update to the latest patch version of the minor after the next minor (no not skip next minor)
				highestNonPreviewPatchVersionNplusTwoMinor := gardencorev1beta1.MachineImageVersion{
					ExpirableVersion: gardencorev1beta1.ExpirableVersion{
						Version: "1.4.1",
					},
					Architectures: []string{"amd64"},
					CRI:           []gardencorev1beta1.CRI{{Name: gardencorev1beta1.CRINameContainerD}},
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
								CRI:           []gardencorev1beta1.CRI{{Name: gardencorev1beta1.CRINameContainerD}},
								Architectures: []string{"amd64"},
							},
							{
								ExpirableVersion: gardencorev1beta1.ExpirableVersion{
									// highest version of next minor, but Shoot should not update, as current version is not expired.
									Version: "1.2.0",
								},
								CRI:           []gardencorev1beta1.CRI{{Name: gardencorev1beta1.CRINameContainerD}},
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
								CRI:           []gardencorev1beta1.CRI{{Name: gardencorev1beta1.CRINameContainerD}},
								Architectures: []string{"amd64"},
							},
							{
								ExpirableVersion: gardencorev1beta1.ExpirableVersion{
									// highest patch for Shoot's current minor
									Version: highestPatchCurrentMinor,
								},
								CRI:           []gardencorev1beta1.CRI{{Name: gardencorev1beta1.CRINameContainerD}},
								Architectures: []string{"amd64"},
							},
							{
								ExpirableVersion: gardencorev1beta1.ExpirableVersion{
									Version: highestVersionForCurrentMajor,
								},
								CRI:           []gardencorev1beta1.CRI{{Name: gardencorev1beta1.CRINameContainerD}},
								Architectures: []string{"amd64"},
							},
							{
								ExpirableVersion: gardencorev1beta1.ExpirableVersion{
									// highest version for next major. Don't update to this next major as need to update to latest version in major.
									Version: "3.2.5",
								},
								CRI:           []gardencorev1beta1.CRI{{Name: gardencorev1beta1.CRINameContainerD}},
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
								CRI:           []gardencorev1beta1.CRI{{Name: gardencorev1beta1.CRINameContainerD}},
								Architectures: []string{"amd64"},
							},
							{
								ExpirableVersion: gardencorev1beta1.ExpirableVersion{
									// intermediate minor (we skip over)
									Version: "1.3.0",
								},
								CRI:           []gardencorev1beta1.CRI{{Name: gardencorev1beta1.CRINameContainerD}},
								Architectures: []string{"amd64"},
							},
							{
								ExpirableVersion: gardencorev1beta1.ExpirableVersion{
									Version: highestVersionForCurrentMajor,
								},
								CRI:           []gardencorev1beta1.CRI{{Name: gardencorev1beta1.CRINameContainerD}},
								Architectures: []string{"amd64"},
							},
							{
								ExpirableVersion: gardencorev1beta1.ExpirableVersion{
									// highest version for next major. Don't update to this next major as need to update to latest version in major.
									Version: "3.2.5",
								},
								CRI:           []gardencorev1beta1.CRI{{Name: gardencorev1beta1.CRINameContainerD}},
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
								CRI:           []gardencorev1beta1.CRI{{Name: gardencorev1beta1.CRINameContainerD}},
								Architectures: []string{"amd64"},
							},
							{
								ExpirableVersion: gardencorev1beta1.ExpirableVersion{
									// intermediate minor (we skip over)
									Version: "1.3.0",
								},
								CRI:           []gardencorev1beta1.CRI{{Name: gardencorev1beta1.CRINameContainerD}},
								Architectures: []string{"amd64"},
							},
							{
								ExpirableVersion: gardencorev1beta1.ExpirableVersion{
									Version: highestVersionForCurrentMajor,
								},
								CRI:           []gardencorev1beta1.CRI{{Name: gardencorev1beta1.CRINameContainerD}},
								Architectures: []string{"amd64"},
							},
							{
								ExpirableVersion: gardencorev1beta1.ExpirableVersion{
									// highest version for next major. Don't update to this next major as need to update to latest version in major.
									Version: "3.2.5",
								},
								CRI:           []gardencorev1beta1.CRI{{Name: gardencorev1beta1.CRINameContainerD}},
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
								CRI:           []gardencorev1beta1.CRI{{Name: gardencorev1beta1.CRINameContainerD}},
								Architectures: []string{"amd64"},
							},
							{
								ExpirableVersion: gardencorev1beta1.ExpirableVersion{
									Version: intermediateVersionNextMajor,
								},
								CRI:           []gardencorev1beta1.CRI{{Name: gardencorev1beta1.CRINameContainerD}},
								Architectures: []string{"amd64"},
							},
							{
								ExpirableVersion: gardencorev1beta1.ExpirableVersion{
									Version: latestVersionNextMajor,
								},
								CRI:           []gardencorev1beta1.CRI{{Name: gardencorev1beta1.CRINameContainerD}},
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
					CRI:           []gardencorev1beta1.CRI{{Name: gardencorev1beta1.CRINameContainerD}},
				}

				// update to the latest patch version of the major after the next major (skip next minor)
				highestNonPreviewVersionNplusTwoMajor := gardencorev1beta1.MachineImageVersion{
					ExpirableVersion: gardencorev1beta1.ExpirableVersion{
						Version: "3.4.1",
					},
					Architectures: []string{"amd64"},
					CRI:           []gardencorev1beta1.CRI{{Name: gardencorev1beta1.CRINameContainerD}},
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
								CRI:           []gardencorev1beta1.CRI{{Name: gardencorev1beta1.CRINameContainerD}},
								Architectures: []string{"amd64"},
							},
							{
								ExpirableVersion: gardencorev1beta1.ExpirableVersion{
									// highest version for next major. Don't update to this next major.
									Version: "2.2.5",
								},
								CRI:           []gardencorev1beta1.CRI{{Name: gardencorev1beta1.CRINameContainerD}},
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

		Context("Shoot uses CloudProfile with Capabilities", func() {
			latestVersionWithSupportedCapabilities := "1.4.2"

			BeforeEach(func() {
				cloudProfile.Spec.MachineCapabilities = []gardencorev1beta1.CapabilityDefinition{
					{Name: "architecture", Values: []string{v1beta1constants.ArchitectureARM64, v1beta1constants.ArchitectureAMD64}},
					{Name: "someCapability", Values: []string{"value1", "value2", "value3"}},
					{Name: "someOtherCapability", Values: []string{"value1", "value2"}},
				}
				cloudProfile.Spec.MachineTypes = []gardencorev1beta1.MachineType{
					{
						Name: "someMachineType",
						Capabilities: gardencorev1beta1.Capabilities{
							"architecture":   []string{v1beta1constants.ArchitectureAMD64},
							"someCapability": []string{"value1"},
						},
					},
					{
						Name: "someOtherMachineType",
						Capabilities: gardencorev1beta1.Capabilities{
							"architecture":   []string{v1beta1constants.ArchitectureAMD64},
							"someCapability": []string{"value2"},
						},
					}, {
						Name: "anotherMachineType",
						Capabilities: gardencorev1beta1.Capabilities{
							"architecture":   []string{v1beta1constants.ArchitectureAMD64},
							"someCapability": []string{"value3"},
						},
					},
				}
				cloudProfile.Spec.MachineImages[0].Versions = []gardencorev1beta1.MachineImageVersion{
					{
						ExpirableVersion: gardencorev1beta1.ExpirableVersion{
							Version: shootCurrentImageVersion,
						},
						CRI: []gardencorev1beta1.CRI{{Name: gardencorev1beta1.CRINameContainerD}},
						CapabilityFlavors: []gardencorev1beta1.MachineImageFlavor{
							{Capabilities: gardencorev1beta1.Capabilities{
								"architecture": []string{v1beta1constants.ArchitectureAMD64},
							}},
							{Capabilities: gardencorev1beta1.Capabilities{
								"architecture": []string{v1beta1constants.ArchitectureARM64},
							}},
						},
					},
					{
						ExpirableVersion: gardencorev1beta1.ExpirableVersion{
							Version: latestVersionWithSupportedCapabilities,
						},
						CRI: []gardencorev1beta1.CRI{{Name: gardencorev1beta1.CRINameContainerD}},
						CapabilityFlavors: []gardencorev1beta1.MachineImageFlavor{
							{Capabilities: gardencorev1beta1.Capabilities{
								"architecture":   []string{v1beta1constants.ArchitectureAMD64},
								"someCapability": []string{"value1", "value2"},
							}},
							{Capabilities: gardencorev1beta1.Capabilities{
								"architecture":   []string{v1beta1constants.ArchitectureARM64},
								"someCapability": []string{"value1", "value2"},
							}},
						},
					},
					{
						ExpirableVersion: gardencorev1beta1.ExpirableVersion{
							Version:        overallLatestVersion,
							ExpirationDate: &expirationDateInTheFuture,
						},
						CRI: []gardencorev1beta1.CRI{{Name: gardencorev1beta1.CRINameContainerD}},
						CapabilityFlavors: []gardencorev1beta1.MachineImageFlavor{
							{Capabilities: gardencorev1beta1.Capabilities{
								"architecture":   []string{v1beta1constants.ArchitectureAMD64},
								"someCapability": []string{"value2"},
							}},
							{Capabilities: gardencorev1beta1.Capabilities{
								"architecture":   []string{v1beta1constants.ArchitectureARM64},
								"someCapability": []string{"value2"},
							}},
						},
					},
				}
			})

			It("should update to the latest version with supported capabilities", func() {
				// the latest overall version does not support the workers' capabilities, hence it should not be updated to
				_, err := maintainMachineImages(log, shoot, cloudProfile)
				Expect(err).NotTo(HaveOccurred())
				assertWorkerMachineImageVersion(&shoot.Spec.Provider.Workers[0], "CoreOs", latestVersionWithSupportedCapabilities)
			})

			It("should update to the latest version as all capabilities are supported", func() {
				shoot.Spec.Provider.Workers[0].Machine.Type = "someOtherMachineType"
				_, err := maintainMachineImages(log, shoot, cloudProfile)
				Expect(err).NotTo(HaveOccurred())
				assertWorkerMachineImageVersion(&shoot.Spec.Provider.Workers[0], "CoreOs", overallLatestVersion)
			})

			It("should not update if the current version is the latest/only that supports the machine type capabilities", func() {
				shoot.Spec.Provider.Workers[0].Machine.Type = "anotherMachineType"
				_, err := maintainMachineImages(log, shoot, cloudProfile)
				Expect(err).NotTo(HaveOccurred())
				assertWorkerMachineImageVersion(&shoot.Spec.Provider.Workers[0], "CoreOs", shootCurrentImageVersion)
			})
		})

		It("should treat workers with `cri: nil` like `cri.name: containerd` and not update if `containerd` is not explicitly supported by the machine image", func() {
			cloudProfile.Spec.MachineImages[0].Versions[1].CRI = []gardencorev1beta1.CRI{{Name: gardencorev1beta1.CRIName("other")}}
			cloudProfile.Spec.MachineImages[0].Versions[3].CRI = []gardencorev1beta1.CRI{{Name: gardencorev1beta1.CRIName("other")}}

			_, err := maintainMachineImages(log, shoot, cloudProfile)
			Expect(err).NotTo(HaveOccurred())
			assertWorkerMachineImageVersion(&shoot.Spec.Provider.Workers[0], "CoreOs", "1.0.0")
		})

		It("should determine that the shoot worker machine images must NOT to be maintained - ForceUpdate not required & MaintenanceAutoUpdate set to false", func() {
			shoot.Spec.Maintenance.AutoUpdate.MachineImageVersion = ptr.To(false)

			expected := shoot.Spec.Provider.Workers[0].Machine.Image.DeepCopy()
			_, err := maintainMachineImages(log, shoot, cloudProfile)

			Expect(err).NotTo(HaveOccurred())
			Expect(shoot.Spec.Provider.Workers[0].Machine.Image).To(Equal(expected))
		})

		It("should determine that the shoot worker machine images must NOT be maintained - found no machineImageVersion with matching CRI", func() {
			cloudProfile.Spec.MachineImages[0].Versions[1].CRI = []gardencorev1beta1.CRI{{Name: gardencorev1beta1.CRIName("other")}}
			cloudProfile.Spec.MachineImages[0].Versions[3].CRI = []gardencorev1beta1.CRI{{Name: gardencorev1beta1.CRIName("other")}}
			shoot.Spec.Provider.Workers[0].CRI = &gardencorev1beta1.CRI{Name: gardencorev1beta1.CRINameContainerD}

			expected := shoot.Spec.Provider.Workers[0].Machine.Image.DeepCopy()
			_, err := maintainMachineImages(log, shoot, cloudProfile)
			Expect(err).NotTo(HaveOccurred())
			Expect(shoot.Spec.Provider.Workers[0].Machine.Image).To(Equal(expected))
		})

		It("should determine that some shoot worker machine images must be not be maintained - MachineImageVersion doesn't support certain CRIs", func() {
			// only the shoots current os image contains the "other" CRI (none of the other versions do) -> this worker pool must not be updated
			cloudProfile.Spec.MachineImages[0].Versions[0].CRI = []gardencorev1beta1.CRI{{Name: gardencorev1beta1.CRIName("other")}}
			shoot.Spec.Provider.Workers[0].CRI = &gardencorev1beta1.CRI{Name: gardencorev1beta1.CRIName("other")}

			// add another pool without CRI constraints -> should be updated via auto-update
			shoot.Spec.Provider.Workers = append(shoot.Spec.Provider.Workers, gardencorev1beta1.Worker{Name: "worker-without-cri-config", Machine: gardencorev1beta1.Machine{Image: shootCurrentImage.DeepCopy(), Architecture: ptr.To("amd64")}})

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
			shoot.Spec.Provider.Workers = append(shoot.Spec.Provider.Workers, gardencorev1beta1.Worker{Name: "worker-without-containerruntime", CRI: &gardencorev1beta1.CRI{Name: gardencorev1beta1.CRINameContainerD}, Machine: gardencorev1beta1.Machine{Image: shootCurrentImage.DeepCopy(), Architecture: ptr.To("amd64")}})

			_, err := maintainMachineImages(log, shoot, cloudProfile)
			Expect(err).NotTo(HaveOccurred())

			assertWorkerMachineImageVersion(&shoot.Spec.Provider.Workers[0], "CoreOs", "1.0.0")
			assertWorkerMachineImageVersion(&shoot.Spec.Provider.Workers[1], "CoreOs", overallLatestVersion)
		})

		It("should determine that some shoot worker machine images must be not be maintained - MachineImageVersion does not support containerruntime", func() {
			cloudProfile.Spec.MachineImages[0].Versions[0].CRI = []gardencorev1beta1.CRI{{Name: gardencorev1beta1.CRINameContainerD, ContainerRuntimes: []gardencorev1beta1.ContainerRuntime{{Type: "gvisor"}, {Type: "kata-container"}, {Type: "some-other-cr"}}}}
			cloudProfile.Spec.MachineImages[0].Versions[1].CRI = []gardencorev1beta1.CRI{{Name: gardencorev1beta1.CRINameContainerD, ContainerRuntimes: []gardencorev1beta1.ContainerRuntime{{Type: "gvisor"}, {Type: "kata-container"}}}}

			shoot.Spec.Provider.Workers[0].CRI = &gardencorev1beta1.CRI{Name: gardencorev1beta1.CRINameContainerD, ContainerRuntimes: []gardencorev1beta1.ContainerRuntime{{Type: "gvisor"}, {Type: "kata-container"}, {Type: "some-other-cr"}}}
			shoot.Spec.Provider.Workers = append(shoot.Spec.Provider.Workers, gardencorev1beta1.Worker{Name: "worker-with-gvisor-and-kata", CRI: &gardencorev1beta1.CRI{Name: gardencorev1beta1.CRINameContainerD, ContainerRuntimes: []gardencorev1beta1.ContainerRuntime{{Type: "gvisor"}, {Type: "kata-container"}}}, Machine: gardencorev1beta1.Machine{Image: shootCurrentImage.DeepCopy(), Architecture: ptr.To("amd64")}})
			shoot.Spec.Provider.Workers = append(shoot.Spec.Provider.Workers, gardencorev1beta1.Worker{Name: "worker-with-gvisor", CRI: &gardencorev1beta1.CRI{Name: gardencorev1beta1.CRINameContainerD, ContainerRuntimes: []gardencorev1beta1.ContainerRuntime{{Type: "gvisor"}}}, Machine: gardencorev1beta1.Machine{Image: shootCurrentImage.DeepCopy(), Architecture: ptr.To("amd64")}})

			_, err := maintainMachineImages(log, shoot, cloudProfile)
			Expect(err).NotTo(HaveOccurred())

			assertWorkerMachineImageVersion(&shoot.Spec.Provider.Workers[0], "CoreOs", "1.0.0")
			assertWorkerMachineImageVersion(&shoot.Spec.Provider.Workers[1], "CoreOs", overallLatestVersion)
			assertWorkerMachineImageVersion(&shoot.Spec.Provider.Workers[2], "CoreOs", overallLatestVersion)
		})

		It("should determine that the shoot worker machine images must not be maintained - found no machineImageVersion with matching kubeletVersionConstraint (control plane K8s version)", func() {
			cloudProfile.Spec.MachineImages[0].Versions[1].KubeletVersionConstraint = ptr.To("< 1.26")
			cloudProfile.Spec.MachineImages[0].Versions[3].KubeletVersionConstraint = ptr.To("< 1.26")
			shoot.Spec.Kubernetes.Version = "1.26.0"

			expected := shoot.Spec.Provider.Workers[0].Machine.Image.DeepCopy()
			_, err := maintainMachineImages(log, shoot, cloudProfile)
			Expect(err).NotTo(HaveOccurred())
			Expect(shoot.Spec.Provider.Workers[0].Machine.Image).To(Equal(expected))
		})

		It("should determine that the shoot worker machine images must be maintained - found machineImageVersion with matching kubeletVersionConstraint (control plane K8s version)", func() {
			cloudProfile.Spec.MachineImages[0].Versions[1].KubeletVersionConstraint = ptr.To("< 1.26")
			shoot.Spec.Kubernetes.Version = "1.25.1"

			_, err := maintainMachineImages(log, shoot, cloudProfile)
			Expect(err).NotTo(HaveOccurred())
			assertWorkerMachineImageVersion(&shoot.Spec.Provider.Workers[0], "CoreOs", overallLatestVersion)
		})

		It("should determine that the shoot worker machine images must not be maintained - found no machineImageVersion with matching kubeletVersionConstraint (worker K8s version)", func() {
			cloudProfile.Spec.MachineImages[0].Versions[1].KubeletVersionConstraint = ptr.To(">= 1.26")
			cloudProfile.Spec.MachineImages[0].Versions[3].KubeletVersionConstraint = ptr.To(">= 1.26")
			shoot.Spec.Kubernetes.Version = "1.26.0"
			shoot.Spec.Provider.Workers[0].Kubernetes = &gardencorev1beta1.WorkerKubernetes{
				Version: ptr.To("1.25.0"),
			}

			expected := shoot.Spec.Provider.Workers[0].Machine.Image.DeepCopy()
			_, err := maintainMachineImages(log, shoot, cloudProfile)
			Expect(err).NotTo(HaveOccurred())
			Expect(shoot.Spec.Provider.Workers[0].Machine.Image).To(Equal(expected))
		})

		It("should determine that the shoot worker machine images must be maintained - found machineImageVersion with matching kubeletVersionConstraint (worker K8s version)", func() {
			assertWorkerMachineImageVersion(&shoot.Spec.Provider.Workers[0], "CoreOs", "1.0.0")
			cloudProfile.Spec.MachineImages[0].Versions[1].KubeletVersionConstraint = ptr.To(">= 1.26")
			shoot.Spec.Kubernetes.Version = "1.27.0"
			shoot.Spec.Provider.Workers[0].Kubernetes = &gardencorev1beta1.WorkerKubernetes{
				Version: ptr.To("1.26.0"),
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

		It("should return an error - cloud profile has no matching (machineImage.type) machine type defined", func() {
			shoot.Spec.Provider.Workers[0].Machine.Type = "non-existing-machine-type"

			_, err := maintainMachineImages(log, shoot, cloudProfile)

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("machine type \"non-existing-machine-type\" of worker \"cpu-worker\" does not exist in cloudprofile"))
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

			_, err := maintainKubernetesVersion(log, shoot.Spec.Kubernetes.Version, shoot.Spec.Maintenance.AutoUpdate.KubernetesVersion, cloudProfile, func(v string) (string, error) {
				shoot.Spec.Kubernetes.Version = v
				return v, nil
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

			_, err := maintainKubernetesVersion(log, shoot.Spec.Kubernetes.Version, shoot.Spec.Maintenance.AutoUpdate.KubernetesVersion, cloudProfile, func(v string) (string, error) {
				shoot.Spec.Kubernetes.Version = v
				return v, nil
			})

			Expect(err).To(Not(HaveOccurred()))
			Expect(shoot.Spec.Kubernetes.Version).To(Equal("1.0.1"))
		})

		It("should determine that the shoot kubernetes version must be maintained - ForceUpdate to latest qualifying patch version of next minor version", func() {
			shoot.Spec.Maintenance.AutoUpdate.KubernetesVersion = false
			cloudProfile.Spec.Kubernetes.Versions[3].ExpirationDate = &expirationDateInThePast
			shoot.Spec.Kubernetes = gardencorev1beta1.Kubernetes{Version: "1.0.2"}

			_, err := maintainKubernetesVersion(log, shoot.Spec.Kubernetes.Version, shoot.Spec.Maintenance.AutoUpdate.KubernetesVersion, cloudProfile, func(v string) (string, error) {
				shoot.Spec.Kubernetes.Version = v
				return v, nil
			})

			Expect(err).NotTo(HaveOccurred())
			Expect(shoot.Spec.Kubernetes.Version).To(Equal("1.1.2"))
		})

		It("should determine that the shoot kubernetes version must be maintained - ForceUpdate to latest qualifying patch version of next minor version", func() {
			shoot.Spec.Maintenance.AutoUpdate.KubernetesVersion = true
			cloudProfile.Spec.Kubernetes.Versions[3].ExpirationDate = &expirationDateInThePast
			shoot.Spec.Kubernetes = gardencorev1beta1.Kubernetes{Version: "1.0.2"}

			_, err := maintainKubernetesVersion(log, shoot.Spec.Kubernetes.Version, shoot.Spec.Maintenance.AutoUpdate.KubernetesVersion, cloudProfile, func(v string) (string, error) {
				shoot.Spec.Kubernetes.Version = v
				return v, nil
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

			_, err := maintainKubernetesVersion(log, shoot.Spec.Kubernetes.Version, shoot.Spec.Maintenance.AutoUpdate.KubernetesVersion, cloudProfile, func(v string) (string, error) {
				shoot.Spec.Kubernetes.Version = v
				return v, nil
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

			_, err := maintainKubernetesVersion(log, shoot.Spec.Kubernetes.Version, shoot.Spec.Maintenance.AutoUpdate.KubernetesVersion, cloudProfile, func(v string) (string, error) {
				shoot.Spec.Kubernetes.Version = v
				return v, nil
			})

			Expect(err).To(HaveOccurred())
		})

		It("should determine that the shoot kubernetes version must be maintained - MaintenanceAutoUpdate set to true", func() {
			shoot.Spec.Maintenance.AutoUpdate.KubernetesVersion = true
			shoot.Spec.Kubernetes = gardencorev1beta1.Kubernetes{Version: "1.0.1"}

			_, err := maintainKubernetesVersion(log, shoot.Spec.Kubernetes.Version, shoot.Spec.Maintenance.AutoUpdate.KubernetesVersion, cloudProfile, func(v string) (string, error) {
				shoot.Spec.Kubernetes.Version = v
				return v, nil
			})

			Expect(err).NotTo(HaveOccurred())
			Expect(shoot.Spec.Kubernetes.Version).To(Equal("1.0.2"))
		})

		It("should determine that the kubernetes version must NOT to be maintained - ForceUpdate not required & MaintenanceAutoUpdate set to false", func() {
			shoot.Spec.Maintenance.AutoUpdate.KubernetesVersion = false
			cloudProfile.Spec.Kubernetes.Versions[4].ExpirationDate = &expirationDateInTheFuture
			shoot.Spec.Kubernetes = gardencorev1beta1.Kubernetes{Version: "1.0.1"}

			_, err := maintainKubernetesVersion(log, shoot.Spec.Kubernetes.Version, shoot.Spec.Maintenance.AutoUpdate.KubernetesVersion, cloudProfile, func(v string) (string, error) {
				shoot.Spec.Kubernetes.Version = v
				return v, nil
			})

			Expect(err).NotTo(HaveOccurred())
			Expect(shoot.Spec.Kubernetes.Version).To(Equal("1.0.1"))
		})

		It("should determine that the shootKubernetes version must be maintained - cloud profile has no matching kubernetes version defined (the shoots kubernetes version has been deleted from the cloudProfile) -> update to latest kubernetes patch version with same minor", func() {
			cloudProfile.Spec.Kubernetes.Versions = kubernetesSettings.Versions[:4]
			shoot.Spec.Maintenance.AutoUpdate.KubernetesVersion = true
			shoot.Spec.Kubernetes = gardencorev1beta1.Kubernetes{Version: "1.0.0"}

			_, err := maintainKubernetesVersion(log, shoot.Spec.Kubernetes.Version, shoot.Spec.Maintenance.AutoUpdate.KubernetesVersion, cloudProfile, func(v string) (string, error) {
				shoot.Spec.Kubernetes.Version = v
				return v, nil
			})

			Expect(err).NotTo(HaveOccurred())
			Expect(shoot.Spec.Kubernetes.Version).To(Equal("1.0.2"))
		})

		It("should determine that the shootKubernetes version must be maintained - cloud profile has no matching kubernetes version defined (the shoots kubernetes version has been deleted from the cloudProfile) && isLatest patch version for minor-> update to latest kubernetes patch version for next minor", func() {
			cloudProfile.Spec.Kubernetes.Versions = kubernetesSettings.Versions[:2]
			shoot.Spec.Maintenance.AutoUpdate.KubernetesVersion = true
			shoot.Spec.Kubernetes = gardencorev1beta1.Kubernetes{Version: "1.0.2"}

			_, err := maintainKubernetesVersion(log, shoot.Spec.Kubernetes.Version, shoot.Spec.Maintenance.AutoUpdate.KubernetesVersion, cloudProfile, func(v string) (string, error) {
				shoot.Spec.Kubernetes.Version = v
				return v, nil
			})

			Expect(err).NotTo(HaveOccurred())
			Expect(shoot.Spec.Kubernetes.Version).To(Equal("1.1.2"))
		})

		It("do not update major Kubernetes version", func() {
			shoot.Spec.Maintenance.AutoUpdate.KubernetesVersion = false
			cloudProfile.Spec.Kubernetes.Versions[3].ExpirationDate = &expirationDateInThePast
			shoot.Spec.Kubernetes = gardencorev1beta1.Kubernetes{Version: "1.1.2"}

			_, err := maintainKubernetesVersion(log, shoot.Spec.Kubernetes.Version, shoot.Spec.Maintenance.AutoUpdate.KubernetesVersion, cloudProfile, func(v string) (string, error) {
				shoot.Spec.Kubernetes.Version = v
				return v, nil
			})

			Expect(err).NotTo(HaveOccurred())
			Expect(shoot.Spec.Kubernetes.Version).To(Equal("1.1.2"))
		})
	})

	Describe("#computeCredentialsToRotationResults", func() {
		var shoot *gardencorev1beta1.Shoot

		BeforeEach(func() {
			shoot = &gardencorev1beta1.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Name: "shoot",
				},
				Spec: gardencorev1beta1.ShootSpec{
					Maintenance: &gardencorev1beta1.Maintenance{
						AutoRotation: &gardencorev1beta1.MaintenanceAutoRotation{
							Credentials: &gardencorev1beta1.MaintenanceCredentialsAutoRotation{
								SSHKeypair: &gardencorev1beta1.MaintenanceRotationConfig{
									RotationPeriod: &metav1.Duration{Duration: 24 * time.Hour},
								},
								Observability: &gardencorev1beta1.MaintenanceRotationConfig{
									RotationPeriod: &metav1.Duration{Duration: 24 * time.Hour},
								},
								ETCDEncryptionKey: &gardencorev1beta1.MaintenanceRotationConfig{
									RotationPeriod: &metav1.Duration{Duration: 24 * time.Hour},
								},
							},
						},
					},
					Provider: gardencorev1beta1.Provider{
						Workers: []gardencorev1beta1.Worker{
							{
								Name: "worker",
							},
						},
					},
				},
			}
		})

		It("should return empty results if no credentials rotation is required", func() {
			shoot.Spec.Maintenance.AutoRotation.Credentials.SSHKeypair.RotationPeriod.Duration = 0
			shoot.Spec.Maintenance.AutoRotation.Credentials.Observability.RotationPeriod.Duration = 0
			shoot.Spec.Maintenance.AutoRotation.Credentials.ETCDEncryptionKey.RotationPeriod.Duration = 0

			results := computeCredentialsToRotationResults(log, shoot, metav1.Time{Time: now})

			Expect(results).To(BeEmpty())
		})

		It("should return successful results for all credentials rotation", func() {
			results := computeCredentialsToRotationResults(log, shoot, metav1.Time{Time: now})

			Expect(results).To(HaveLen(3))
			Expect(results["rotate-ssh-keypair"]).To(Equal(updateResult{
				description:  "SSH keypair rotation started",
				reason:       "Automatic rotation of SSH keypair configured",
				isSuccessful: true,
			}))
			Expect(results["rotate-observability-credentials"]).To(Equal(updateResult{
				description:  "Observability passwords rotation started",
				reason:       "Automatic rotation of observability passwords configured",
				isSuccessful: true,
			}))
			Expect(results["rotate-etcd-encryption-key"]).To(Equal(updateResult{
				description:  "ETCD Encryption key rotation started",
				reason:       "Automatic rotation of etcd encryption key configured",
				isSuccessful: true,
			}))
		})

		It("should return successful results only when the rotation period has passed", func() {
			shoot.Status.Credentials = &gardencorev1beta1.ShootCredentials{
				Rotation: &gardencorev1beta1.ShootCredentialsRotation{
					SSHKeypair: &gardencorev1beta1.ShootSSHKeypairRotation{
						LastCompletionTime: &metav1.Time{Time: now.Add(-48 * time.Hour)},
					},
					Observability: &gardencorev1beta1.ObservabilityRotation{
						LastCompletionTime: &metav1.Time{Time: now.Add(-72 * time.Hour)},
					},
					ETCDEncryptionKey: &gardencorev1beta1.ETCDEncryptionKeyRotation{
						LastCompletionTime: &metav1.Time{Time: now.Add(-96 * time.Hour)},
					},
				},
			}
			results := computeCredentialsToRotationResults(log, shoot, metav1.Time{Time: now})

			Expect(results).To(HaveLen(3))
			Expect(results["rotate-ssh-keypair"]).To(Equal(updateResult{
				description:  "SSH keypair rotation started",
				reason:       "Automatic rotation of SSH keypair configured",
				isSuccessful: true,
			}))
			Expect(results["rotate-observability-credentials"]).To(Equal(updateResult{
				description:  "Observability passwords rotation started",
				reason:       "Automatic rotation of observability passwords configured",
				isSuccessful: true,
			}))
			Expect(results["rotate-etcd-encryption-key"]).To(Equal(updateResult{
				description:  "ETCD Encryption key rotation started",
				reason:       "Automatic rotation of etcd encryption key configured",
				isSuccessful: true,
			}))
		})

		It("should return successful results only when the rotation period has passed for newly created Shoot", func() {
			shoot.CreationTimestamp = metav1.Time{Time: now.Add(-48 * time.Hour)}
			shoot.Status.Credentials = nil
			results := computeCredentialsToRotationResults(log, shoot, metav1.Time{Time: now})

			Expect(results).To(HaveLen(3))
			Expect(results["rotate-ssh-keypair"]).To(Equal(updateResult{
				description:  "SSH keypair rotation started",
				reason:       "Automatic rotation of SSH keypair configured",
				isSuccessful: true,
			}))
			Expect(results["rotate-observability-credentials"]).To(Equal(updateResult{
				description:  "Observability passwords rotation started",
				reason:       "Automatic rotation of observability passwords configured",
				isSuccessful: true,
			}))
			Expect(results["rotate-etcd-encryption-key"]).To(Equal(updateResult{
				description:  "ETCD Encryption key rotation started",
				reason:       "Automatic rotation of etcd encryption key configured",
				isSuccessful: true,
			}))
		})

		It("should fail etcd encryption key rotation when etcd key rotation is in progress", func() {
			shoot.Spec.Maintenance.AutoRotation.Credentials.SSHKeypair.RotationPeriod.Duration = 0
			shoot.Spec.Maintenance.AutoRotation.Credentials.Observability.RotationPeriod.Duration = 0
			shoot.Status.Credentials = &gardencorev1beta1.ShootCredentials{
				Rotation: &gardencorev1beta1.ShootCredentialsRotation{
					ETCDEncryptionKey: &gardencorev1beta1.ETCDEncryptionKeyRotation{
						Phase: gardencorev1beta1.RotationPrepared,
					},
				},
			}
			results := computeCredentialsToRotationResults(log, shoot, metav1.Time{Time: now})

			Expect(results).To(HaveLen(1))
			Expect(results["rotate-etcd-encryption-key"]).To(Equal(updateResult{
				description:  "Could not start ETCD encryption key rotation",
				reason:       "ETCD encryption key rotation is already in progress",
				isSuccessful: false,
			}))
		})

		It("should not return results when the rotation period has not passed", func() {
			shoot.CreationTimestamp = metav1.Time{Time: now.Add(-48 * time.Hour)}
			shoot.Status.Credentials = &gardencorev1beta1.ShootCredentials{
				Rotation: &gardencorev1beta1.ShootCredentialsRotation{
					SSHKeypair: &gardencorev1beta1.ShootSSHKeypairRotation{
						LastCompletionTime: &metav1.Time{Time: now},
					},
					Observability: &gardencorev1beta1.ObservabilityRotation{
						LastCompletionTime: &metav1.Time{Time: now.Add(-time.Hour)},
					},
					ETCDEncryptionKey: &gardencorev1beta1.ETCDEncryptionKeyRotation{
						LastCompletionTime: &metav1.Time{Time: now.Add(-4 * time.Hour)},
					},
				},
			}
			results := computeCredentialsToRotationResults(log, shoot, metav1.Time{Time: now})

			Expect(results).To(BeEmpty())
		})

		It("should not return results when Shoow is newly created", func() {
			shoot.CreationTimestamp = metav1.Time{Time: now}
			shoot.Status.Credentials = nil
			results := computeCredentialsToRotationResults(log, shoot, metav1.Time{Time: now})

			Expect(results).To(BeEmpty())
		})
	})

	Describe("#getOperation", func() {
		var shoot *gardencorev1beta1.Shoot

		BeforeEach(func() {
			shoot = &gardencorev1beta1.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Name: "shoot",
				},
			}
		})

		DescribeTable("#getOperation",
			func(maintenanceAnnotation *string, credentialsToRotationUpdate map[string]updateResult, expectedResult []string) {
				if maintenanceAnnotation != nil {
					shoot.Annotations = map[string]string{
						"maintenance.gardener.cloud/operation": *maintenanceAnnotation,
					}
				}

				result := getOperation(shoot, credentialsToRotationUpdate)
				resultOptions := utils.SplitAndTrimString(result, ";")
				Expect(resultOptions).To(ConsistOf(expectedResult))
			},
			Entry("should return reconcile operation when there is no maintenance operation", nil, nil, []string{"reconcile"}),
			Entry("should return reconcile operation when maintenance operation is empty", ptr.To(""), nil, []string{"reconcile"}),
			Entry("should return maintenance operation when it is not empty", ptr.To("foo"), nil, []string{"reconcile", "foo"}),
			Entry("should return rotate-ssh-keypair operation when it is not part of the result updates", ptr.To("rotate-ssh-keypair"),
				map[string]updateResult{
					"rotate-observability-credentials": {
						isSuccessful: true,
					},
				}, []string{"reconcile", "rotate-ssh-keypair", "rotate-observability-credentials"}),
			Entry("should return rotate-observability-credentials operation when it is not part of the result updates", ptr.To("rotate-observability-credentials"),
				map[string]updateResult{
					"rotate-etcd-encryption-key": {
						isSuccessful: true,
					},
				}, []string{"reconcile", "rotate-observability-credentials", "rotate-etcd-encryption-key"}),
			Entry("should return appended options when maintenance operation is rotate-ssh-keypair", ptr.To("rotate-ssh-keypair"),
				map[string]updateResult{
					"rotate-ssh-keypair": {
						isSuccessful: true,
					},
				}, []string{"reconcile", "rotate-ssh-keypair", "rotate-ssh-keypair"}),
			Entry("should return appended options when maintenance operation is rotate-observability-credentials", ptr.To("rotate-credentials-start"),
				map[string]updateResult{
					"rotate-observability-credentials": {
						isSuccessful: true,
					},
				}, []string{"reconcile", "rotate-credentials-start", "rotate-observability-credentials"}),
			Entry("should not append rotate-etcd-encryption-key when rotate-etcd-encryption-key-start is present in maintenance operations", ptr.To("rotate-etcd-encryption-key-start"),
				map[string]updateResult{
					"rotate-etcd-encryption-key": {
						isSuccessful: true,
					},
				}, []string{"reconcile", "rotate-etcd-encryption-key-start"}),
			Entry("should return reconcile when all operations in result updates have failed", nil,
				map[string]updateResult{
					"rotate-ssh-keypair": {
						isSuccessful: false,
					},
					"rotate-observability-credentials": {
						isSuccessful: false,
					},
					"rotate-etcd-encryption-key": {
						isSuccessful: false,
					},
				}, []string{"reconcile"}),
			Entry("should return correct operations", nil,
				map[string]updateResult{
					"rotate-ssh-keypair": {
						isSuccessful: false,
					},
					"rotate-observability-credentials": {
						isSuccessful: true,
					},
					"rotate-etcd-encryption-key": {
						isSuccessful: true,
					},
				}, []string{"reconcile", "rotate-observability-credentials", "rotate-etcd-encryption-key"}),
			Entry("should return all operations", ptr.To("rotate-credentials-start"),
				map[string]updateResult{
					"rotate-ssh-keypair": {
						isSuccessful: true,
					},
					"rotate-observability-credentials": {
						isSuccessful: true,
					},
					"rotate-etcd-encryption-key": {
						isSuccessful: true,
					},
				}, []string{"reconcile", "rotate-credentials-start", "rotate-ssh-keypair", "rotate-observability-credentials", "rotate-etcd-encryption-key"}),
		)
	})

	Describe("#maintainFeatureGatesForShoot", func() {
		var (
			shoot                   *gardencorev1beta1.Shoot
			supportedfeatureGate1   = "featureGate1"
			supportedfeatureGate2   = "featureGate2"
			unsupportedfeatureGate1 = "featureGate3"
			unsupportedfeatureGate2 = "featureGate4"
		)

		BeforeEach(func() {
			defer test.WithVar(
				&IsFeatureGateSupported, isFeatureGateSupported,
			)

			shoot = &gardencorev1beta1.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Name: "shoot",
				},
				Spec: gardencorev1beta1.ShootSpec{
					Kubernetes: gardencorev1beta1.Kubernetes{
						Version: "1.13.5",
						KubeAPIServer: &gardencorev1beta1.KubeAPIServerConfig{
							KubernetesConfig: gardencorev1beta1.KubernetesConfig{
								FeatureGates: map[string]bool{
									supportedfeatureGate1:   true,
									unsupportedfeatureGate1: true,
									supportedfeatureGate2:   false,
								},
							},
						},
						KubeControllerManager: &gardencorev1beta1.KubeControllerManagerConfig{
							KubernetesConfig: gardencorev1beta1.KubernetesConfig{
								FeatureGates: map[string]bool{
									supportedfeatureGate1:   true,
									unsupportedfeatureGate1: true,
									unsupportedfeatureGate2: false,
								},
							},
						},
						KubeScheduler: &gardencorev1beta1.KubeSchedulerConfig{
							KubernetesConfig: gardencorev1beta1.KubernetesConfig{
								FeatureGates: map[string]bool{
									supportedfeatureGate2:   true,
									unsupportedfeatureGate1: true,
								},
							},
						},
						KubeProxy: &gardencorev1beta1.KubeProxyConfig{
							KubernetesConfig: gardencorev1beta1.KubernetesConfig{
								FeatureGates: map[string]bool{
									supportedfeatureGate1:   true,
									supportedfeatureGate2:   false,
									unsupportedfeatureGate2: true,
								},
							},
						},
						Kubelet: &gardencorev1beta1.KubeletConfig{
							KubernetesConfig: gardencorev1beta1.KubernetesConfig{
								FeatureGates: map[string]bool{
									supportedfeatureGate1:   true,
									unsupportedfeatureGate2: true,
								},
							},
						},
					},
					Provider: gardencorev1beta1.Provider{
						Workers: []gardencorev1beta1.Worker{
							{
								Name: "cpu-worker-1",
								Kubernetes: &gardencorev1beta1.WorkerKubernetes{
									Kubelet: &gardencorev1beta1.KubeletConfig{
										KubernetesConfig: gardencorev1beta1.KubernetesConfig{
											FeatureGates: map[string]bool{
												supportedfeatureGate1:   true,
												unsupportedfeatureGate2: true,
											},
										},
									},
								},
							},
							{
								Name: "cpu-worker-2",
								Kubernetes: &gardencorev1beta1.WorkerKubernetes{
									Kubelet: &gardencorev1beta1.KubeletConfig{
										KubernetesConfig: gardencorev1beta1.KubernetesConfig{
											FeatureGates: map[string]bool{
												supportedfeatureGate2:   true,
												unsupportedfeatureGate1: true,
											},
										},
									},
								},
							},
							{
								Name: "cpu-worker-3",
								Kubernetes: &gardencorev1beta1.WorkerKubernetes{
									Kubelet: &gardencorev1beta1.KubeletConfig{
										KubernetesConfig: gardencorev1beta1.KubernetesConfig{
											FeatureGates: map[string]bool{
												supportedfeatureGate1: true,
											},
										},
									},
								},
							},
						},
					},
				},
			}
		})

		It("should maintain feature gates", func() {
			result := maintainFeatureGatesForShoot(shoot)
			Expect(result).To(ConsistOf(
				ContainSubstring("Removed feature gates from %q because they are not supported in Kubernetes version %q: %s", "spec.kubernetes.kubeAPIServer.featureGates", "1.13.5", unsupportedfeatureGate1),
				ContainSubstring("Removed feature gates from %q because they are not supported in Kubernetes version %q: %s", "spec.kubernetes.kubeControllerManager.featureGates", "1.13.5", fmt.Sprintf("%s, %s", unsupportedfeatureGate1, unsupportedfeatureGate2)),
				ContainSubstring("Removed feature gates from %q because they are not supported in Kubernetes version %q: %s", "spec.kubernetes.kubeScheduler.featureGates", "1.13.5", unsupportedfeatureGate1),
				ContainSubstring("Removed feature gates from %q because they are not supported in Kubernetes version %q: %s", "spec.kubernetes.kubeProxy.featureGates", "1.13.5", unsupportedfeatureGate2),
				ContainSubstring("Removed feature gates from %q because they are not supported in Kubernetes version %q: %s", "spec.kubernetes.kubelet.featureGates", "1.13.5", unsupportedfeatureGate2),
				ContainSubstring("Removed feature gates from %q because they are not supported in Kubernetes version %q: %s", "spec.provider.workers[0].kubernetes.kubelet.featureGates", "1.13.5", unsupportedfeatureGate2),
				ContainSubstring("Removed feature gates from %q because they are not supported in Kubernetes version %q: %s", "spec.provider.workers[1].kubernetes.kubelet.featureGates", "1.13.5", unsupportedfeatureGate1),
			))
			Expect(shoot.Spec.Kubernetes.KubeAPIServer.FeatureGates).To(Equal(map[string]bool{
				supportedfeatureGate1: true,
				supportedfeatureGate2: false,
			}))
			Expect(shoot.Spec.Kubernetes.KubeControllerManager.FeatureGates).To(Equal(map[string]bool{
				supportedfeatureGate1: true,
			}))
			Expect(shoot.Spec.Kubernetes.KubeScheduler.FeatureGates).To(Equal(map[string]bool{
				supportedfeatureGate2: true,
			}))
			Expect(shoot.Spec.Kubernetes.KubeProxy.FeatureGates).To(Equal(map[string]bool{
				supportedfeatureGate1: true,
				supportedfeatureGate2: false,
			}))
			Expect(shoot.Spec.Kubernetes.Kubelet.FeatureGates).To(Equal(map[string]bool{
				supportedfeatureGate1: true,
			}))
			Expect(shoot.Spec.Provider.Workers[0].Kubernetes.Kubelet.FeatureGates).To(Equal(map[string]bool{
				supportedfeatureGate1: true,
			}))
			Expect(shoot.Spec.Provider.Workers[1].Kubernetes.Kubelet.FeatureGates).To(Equal(map[string]bool{
				supportedfeatureGate2: true,
			}))
			Expect(shoot.Spec.Provider.Workers[2].Kubernetes.Kubelet.FeatureGates).To(Equal(map[string]bool{
				supportedfeatureGate1: true,
			}))
		})
	})

	Describe("#maintainAdmissionPlugins", func() {
		var (
			shoot                       *gardencorev1beta1.Shoot
			supportedAdmissionPlugin1   = "admissionPlugin1"
			supportedAdmissionPlugin2   = "admissionPlugin2"
			unsupportedAdmissionPlugin1 = "admissionPlugin3"
			unsupportedAdmissionPlugin2 = "admissionPlugin4"
		)

		BeforeEach(func() {
			defer test.WithVar(
				&IsAdmissionPluginSupported, isAdmissionPluginSupported,
			)

			shoot = &gardencorev1beta1.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Name: "shoot",
				},
				Spec: gardencorev1beta1.ShootSpec{
					Kubernetes: gardencorev1beta1.Kubernetes{
						Version: "1.13.5",
						KubeAPIServer: &gardencorev1beta1.KubeAPIServerConfig{
							AdmissionPlugins: []gardencorev1beta1.AdmissionPlugin{
								{
									Name: supportedAdmissionPlugin1,
								},
								{
									Name: supportedAdmissionPlugin2,
								},
								{
									Name: unsupportedAdmissionPlugin1,
								},
								{
									Name: unsupportedAdmissionPlugin2,
								},
							},
						},
					},
				},
			}
		})

		It("should maintain admission plugins", func() {
			result := maintainAdmissionPluginsForShoot(shoot)
			Expect(result).To(ConsistOf(
				ContainSubstring("Removed admission plugins from %q because they are not supported in Kubernetes version %q: %s", "spec.kubernetes.kubeAPIServer.admissionPlugins", "1.13.5", fmt.Sprintf("%s, %s", unsupportedAdmissionPlugin1, unsupportedAdmissionPlugin2)),
			))
			Expect(shoot.Spec.Kubernetes.KubeAPIServer.AdmissionPlugins).To(ConsistOf(
				HaveField("Name", Equal(supportedAdmissionPlugin1)),
				HaveField("Name", Equal(supportedAdmissionPlugin2)),
			))
		})

		It("should maintain admission plugins if there are no unsupported plugins", func() {
			shoot.Spec.Kubernetes.KubeAPIServer.AdmissionPlugins = []gardencorev1beta1.AdmissionPlugin{
				{
					Name: supportedAdmissionPlugin1,
				},
				{
					Name: supportedAdmissionPlugin2,
				},
			}

			result := maintainAdmissionPluginsForShoot(shoot)
			Expect(result).To(BeEmpty())
			Expect(shoot.Spec.Kubernetes.KubeAPIServer.AdmissionPlugins).To(ConsistOf(
				HaveField("Name", Equal(supportedAdmissionPlugin1)),
				HaveField("Name", Equal(supportedAdmissionPlugin2)),
			))
		})
	})

	Describe("#migrateSecretBindingToCredentialsBinding", func() {
		var (
			ctx               context.Context
			reconciler        *Reconciler
			fakeClient        client.Client
			shoot             *gardencorev1beta1.Shoot
			secretBinding     *gardencorev1beta1.SecretBinding
			namespace         = "test-namespace"
			secretBindingName = "test-secret-binding"
			secretName        = "test-secret"
			secretNamespace   = "test-secret-namespace"
			providerType      = "test-provider"
		)

		BeforeEach(func() {
			ctx = context.TODO()

			scheme := runtime.NewScheme()
			Expect(gardencorev1beta1.AddToScheme(scheme)).To(Succeed())
			Expect(securityv1alpha1.AddToScheme(scheme)).To(Succeed())

			fakeClient = fakeclient.NewClientBuilder().WithScheme(scheme).Build()

			reconciler = &Reconciler{
				Client: fakeClient,
			}

			shoot = &gardencorev1beta1.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-shoot",
					Namespace: namespace,
				},
				Spec: gardencorev1beta1.ShootSpec{
					SecretBindingName: ptr.To(secretBindingName),
				},
			}

			secretBinding = &gardencorev1beta1.SecretBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      secretBindingName,
					Namespace: namespace,
				},
				Provider: &gardencorev1beta1.SecretBindingProvider{
					Type: providerType,
				},
				SecretRef: corev1.SecretReference{
					Name:      secretName,
					Namespace: secretNamespace,
				},
			}
		})

		It("should migrate from SecretBinding to CredentialsBinding when none exists", func() {
			Expect(fakeClient.Create(ctx, secretBinding)).To(Succeed())

			err := reconciler.migrateSecretBindingToCredentialsBinding(ctx, shoot)
			Expect(err).NotTo(HaveOccurred())

			Expect(shoot.Spec.SecretBindingName).To(BeNil())
			Expect(shoot.Spec.CredentialsBindingName).NotTo(BeNil())
			Expect(*shoot.Spec.CredentialsBindingName).To(Equal("force-migrated-" + secretBindingName))

			createdCredentialsBinding := &securityv1alpha1.CredentialsBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "force-migrated-" + secretBindingName,
					Namespace: namespace,
				},
			}
			err = fakeClient.Get(ctx, client.ObjectKeyFromObject(createdCredentialsBinding), createdCredentialsBinding)
			Expect(err).NotTo(HaveOccurred())

			Expect(createdCredentialsBinding).To(Equal(&securityv1alpha1.CredentialsBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "force-migrated-" + secretBindingName,
					Namespace: namespace,
					Labels: map[string]string{
						"credentialsbinding.gardener.cloud/status": "force-migrated",
					},
					ResourceVersion: "1",
				},
				Provider: securityv1alpha1.CredentialsBindingProvider{
					Type: providerType,
				},
				CredentialsRef: corev1.ObjectReference{
					APIVersion: "v1",
					Kind:       "Secret",
					Name:       secretName,
					Namespace:  secretNamespace,
				},
			}))
		})

		It("should use existing user-created CredentialsBinding when it references the same Secret and Quotas match", func() {
			Expect(fakeClient.Create(ctx, secretBinding)).To(Succeed())

			existingCredentialsBinding := &securityv1alpha1.CredentialsBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      secretBindingName,
					Namespace: namespace,
				},
				Provider: securityv1alpha1.CredentialsBindingProvider{
					Type: providerType,
				},
				CredentialsRef: corev1.ObjectReference{
					APIVersion: "v1",
					Kind:       "Secret",
					Name:       secretName,
					Namespace:  secretNamespace,
				},
			}
			Expect(fakeClient.Create(ctx, existingCredentialsBinding)).To(Succeed())

			err := reconciler.migrateSecretBindingToCredentialsBinding(ctx, shoot)
			Expect(err).NotTo(HaveOccurred())

			Expect(shoot.Spec.SecretBindingName).To(BeNil())
			Expect(shoot.Spec.CredentialsBindingName).NotTo(BeNil())
			Expect(*shoot.Spec.CredentialsBindingName).To(Equal(secretBindingName))
		})

		It("should fail when existing CredentialsBinding references a different Secret", func() {
			Expect(fakeClient.Create(ctx, secretBinding)).To(Succeed())

			existingCredentialsBinding := &securityv1alpha1.CredentialsBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "force-migrated-" + secretBindingName,
					Namespace: namespace,
				},
				Provider: securityv1alpha1.CredentialsBindingProvider{
					Type: providerType,
				},
				CredentialsRef: corev1.ObjectReference{
					APIVersion: "v1",
					Kind:       "Secret",
					Name:       "different-secret",
					Namespace:  secretNamespace,
				},
			}
			Expect(fakeClient.Create(ctx, existingCredentialsBinding)).To(Succeed())

			err := reconciler.migrateSecretBindingToCredentialsBinding(ctx, shoot)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("does not reference the same Secret"))
		})

		It("should use existing CredentialsBinding when quotas match as sets (order doesn't matter)", func() {
			quota1 := corev1.ObjectReference{APIVersion: "v1", Kind: "Quota", Name: "quota1", Namespace: "ns1"}
			quota2 := corev1.ObjectReference{APIVersion: "v1", Kind: "Quota", Name: "quota2", Namespace: "ns2"}
			secretBinding.Quotas = []corev1.ObjectReference{quota1, quota2}
			Expect(fakeClient.Create(ctx, secretBinding)).To(Succeed())

			existingCredentialsBinding := &securityv1alpha1.CredentialsBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "force-migrated-" + secretBindingName,
					Namespace: namespace,
				},
				Provider: securityv1alpha1.CredentialsBindingProvider{
					Type: providerType,
				},
				CredentialsRef: corev1.ObjectReference{
					APIVersion: "v1",
					Kind:       "Secret",
					Name:       secretName,
					Namespace:  secretNamespace,
				},
				Quotas: []corev1.ObjectReference{quota2, quota1},
			}
			Expect(fakeClient.Create(ctx, existingCredentialsBinding)).To(Succeed())

			err := reconciler.migrateSecretBindingToCredentialsBinding(ctx, shoot)
			Expect(err).NotTo(HaveOccurred())

			Expect(shoot.Spec.SecretBindingName).To(BeNil())
			Expect(shoot.Spec.CredentialsBindingName).NotTo(BeNil())
			Expect(*shoot.Spec.CredentialsBindingName).To(Equal("force-migrated-" + secretBindingName))
		})

		It("should fail when existing CredentialsBinding has different quotas", func() {
			quota1 := corev1.ObjectReference{APIVersion: "v1", Kind: "Quota", Name: "quota1", Namespace: "ns1"}
			quota2 := corev1.ObjectReference{APIVersion: "v1", Kind: "Quota", Name: "quota2", Namespace: "ns2"}
			secretBinding.Quotas = []corev1.ObjectReference{quota1, quota2}
			Expect(fakeClient.Create(ctx, secretBinding)).To(Succeed())

			quota3 := corev1.ObjectReference{APIVersion: "v1", Kind: "Quota", Name: "quota3", Namespace: "ns3"}
			existingCredentialsBinding := &securityv1alpha1.CredentialsBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "force-migrated-" + secretBindingName,
					Namespace: namespace,
				},
				Provider: securityv1alpha1.CredentialsBindingProvider{
					Type: providerType,
				},
				CredentialsRef: corev1.ObjectReference{
					APIVersion: "v1",
					Kind:       "Secret",
					Name:       secretName,
					Namespace:  secretNamespace,
				},
				Quotas: []corev1.ObjectReference{quota1, quota3}, // Different quota
			}
			Expect(fakeClient.Create(ctx, existingCredentialsBinding)).To(Succeed())

			err := reconciler.migrateSecretBindingToCredentialsBinding(ctx, shoot)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("does not have the same Quotas"))
		})
	})

	Describe("#quotasEqual", func() {
		It("should return true for empty slices", func() {
			Expect(quotasEqual(nil, nil)).To(BeTrue())
			Expect(quotasEqual([]corev1.ObjectReference{}, []corev1.ObjectReference{})).To(BeTrue())
		})

		It("should return false for different lengths", func() {
			quota1 := corev1.ObjectReference{APIVersion: "v1", Kind: "Quota", Name: "quota1"}
			Expect(quotasEqual([]corev1.ObjectReference{quota1}, []corev1.ObjectReference{})).To(BeFalse())
		})

		It("should return true for same quotas in different order", func() {
			quota1 := corev1.ObjectReference{APIVersion: "v1", Kind: "Quota", Name: "quota1", Namespace: "ns1"}
			quota2 := corev1.ObjectReference{APIVersion: "v1", Kind: "Quota", Name: "quota2", Namespace: "ns2"}

			slice1 := []corev1.ObjectReference{quota1, quota2}
			slice2 := []corev1.ObjectReference{quota2, quota1}

			Expect(quotasEqual(slice1, slice2)).To(BeTrue())
		})

		It("should return false for different quotas", func() {
			quota1 := corev1.ObjectReference{APIVersion: "v1", Kind: "Quota", Name: "quota1", Namespace: "ns1"}
			quota2 := corev1.ObjectReference{APIVersion: "v1", Kind: "Quota", Name: "quota2", Namespace: "ns2"}
			quota3 := corev1.ObjectReference{APIVersion: "v1", Kind: "Quota", Name: "quota3", Namespace: "ns3"}

			slice1 := []corev1.ObjectReference{quota1, quota2}
			slice2 := []corev1.ObjectReference{quota1, quota3}

			Expect(quotasEqual(slice1, slice2)).To(BeFalse())
		})

		It("should handle quotas without namespace", func() {
			quota1 := corev1.ObjectReference{APIVersion: "v1", Kind: "Quota", Name: "quota1"}
			quota2 := corev1.ObjectReference{APIVersion: "v1", Kind: "Quota", Name: "quota2"}

			slice1 := []corev1.ObjectReference{quota1, quota2}
			slice2 := []corev1.ObjectReference{quota2, quota1}

			Expect(quotasEqual(slice1, slice2)).To(BeTrue())
		})
	})
})

func assertWorkerMachineImageVersion(worker *gardencorev1beta1.Worker, imageName string, imageVersion string) {
	ExpectWithOffset(1, worker.Machine.Image.Name).To(Equal(imageName))
	ExpectWithOffset(1, *worker.Machine.Image.Version).To(Equal(imageVersion))
}

func isFeatureGateSupported(featureGate, v string) (bool, error) {
	featureGateVersionRanges := map[string]*featuresvalidation.FeatureGateVersionRange{
		"featureGate1": {VersionRange: version.VersionRange{}},
		"featureGate2": {VersionRange: version.VersionRange{AddedInVersion: "1.12", RemovedInVersion: "1.14"}},
		"featureGate3": {VersionRange: version.VersionRange{RemovedInVersion: "1.13"}},
		"featureGate4": {VersionRange: version.VersionRange{AddedInVersion: "1.9", RemovedInVersion: "1.13"}},
	}

	vr := featureGateVersionRanges[featureGate]
	if vr == nil {
		return false, fmt.Errorf("unknown feature gate %s", featureGate)
	}

	return vr.Contains(v)
}

func isAdmissionPluginSupported(plugin, v string) (bool, error) {
	admissionPluginsVersionRanges := map[string]*admissionpluginsvalidation.AdmissionPluginVersionRange{
		"admissionPlugin1": {VersionRange: version.VersionRange{AddedInVersion: "1.12", RemovedInVersion: "1.14"}},
		"admissionPlugin2": {VersionRange: version.VersionRange{}},
		"admissionPlugin3": {VersionRange: version.VersionRange{RemovedInVersion: "1.13"}},
		"admissionPlugin4": {VersionRange: version.VersionRange{AddedInVersion: "1.8", RemovedInVersion: "1.13"}},
	}

	vr := admissionPluginsVersionRanges[plugin]
	if vr == nil {
		return false, fmt.Errorf("unknown admission plugin %q", plugin)
	}
	return vr.Contains(v)
}
