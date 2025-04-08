// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation_test

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	gomegatypes "github.com/onsi/gomega/types"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener/pkg/apis/core"
	. "github.com/gardener/gardener/pkg/apis/core/validation"
	"github.com/gardener/gardener/pkg/features"
	"github.com/gardener/gardener/pkg/utils/test"
)

var (
	machineImageName = "some-machine-image"
	metadata         = metav1.ObjectMeta{
		Name: "profile",
	}
	machineType = core.MachineType{
		Name:         "machine-type-1",
		CPU:          resource.MustParse("2"),
		GPU:          resource.MustParse("0"),
		Memory:       resource.MustParse("100Gi"),
		Architecture: ptr.To("amd64"),
	}
	machineTypesConstraint = []core.MachineType{
		machineType,
	}
	volumeTypesConstraint = []core.VolumeType{
		{
			Name:  "volume-type-1",
			Class: "super-premium",
		},
	}

	negativeQuantity = resource.MustParse("-1")
	validQuantity    = resource.MustParse("100Gi")

	invalidMachineType = core.MachineType{
		Name:   "",
		CPU:    negativeQuantity,
		GPU:    negativeQuantity,
		Memory: resource.MustParse("-100Gi"),
		Storage: &core.MachineTypeStorage{
			MinSize: &negativeQuantity,
		},
		Architecture: ptr.To("amd64"),
	}
	invalidMachineType2 = core.MachineType{
		Name:   "negative-storage-size",
		CPU:    resource.MustParse("2"),
		GPU:    resource.MustParse("0"),
		Memory: resource.MustParse("100Gi"),
		Storage: &core.MachineTypeStorage{
			StorageSize: &negativeQuantity,
		},
		Architecture: ptr.To("amd64"),
	}
	invalidMachineType3 = core.MachineType{
		Name:   "min-size-and-storage-size",
		CPU:    resource.MustParse("2"),
		GPU:    resource.MustParse("0"),
		Memory: resource.MustParse("100Gi"),
		Storage: &core.MachineTypeStorage{
			MinSize:     &validQuantity,
			StorageSize: &validQuantity,
		},
		Architecture: ptr.To("arm64"),
	}
	invalidMachineType4 = core.MachineType{
		Name:         "empty-storage-config",
		CPU:          resource.MustParse("2"),
		GPU:          resource.MustParse("0"),
		Memory:       resource.MustParse("100Gi"),
		Storage:      &core.MachineTypeStorage{},
		Architecture: ptr.To("foo"),
	}
	invalidMachineTypes = []core.MachineType{
		invalidMachineType,
		invalidMachineType2,
		invalidMachineType3,
		invalidMachineType4,
	}
	invalidVolumeTypes = []core.VolumeType{
		{
			Name:    "",
			Class:   "",
			MinSize: &negativeQuantity,
		},
	}

	regionName = "region1"
	zoneName   = "zone1"

	supportedClassification  = core.ClassificationSupported
	previewClassification    = core.ClassificationPreview
	deprecatedClassification = core.ClassificationDeprecated

	updateStrategyMajor = core.MachineImageUpdateStrategy("major")
)

var _ = Describe("CloudProfile Validation Tests ", func() {
	Describe("#ValidateCloudProfile", func() {
		It("should forbid empty CloudProfile resources", func() {
			cloudProfile := &core.CloudProfile{
				ObjectMeta: metav1.ObjectMeta{},
				Spec:       core.CloudProfileSpec{},
			}

			errorList := ValidateCloudProfile(cloudProfile)

			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("metadata.name"),
				})), PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.type"),
				})), PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.kubernetes.versions"),
				})), PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.kubernetes.versions"),
				})), PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.machineImages"),
				})), PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.machineTypes"),
				})), PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.regions"),
				}))))
		})

		Context("tests for unknown cloud profiles", func() {
			var (
				cloudProfile *core.CloudProfile

				duplicatedKubernetes = core.KubernetesSettings{
					Versions: []core.ExpirableVersion{{Version: "1.11.4"}, {Version: "1.11.4", Classification: &previewClassification}},
				}
				duplicatedRegions = []core.Region{
					{
						Name: regionName,
						Zones: []core.AvailabilityZone{
							{Name: zoneName},
						},
					},
					{
						Name: regionName,
						Zones: []core.AvailabilityZone{
							{Name: zoneName},
						},
					},
				}
				duplicatedZones = []core.Region{
					{
						Name: regionName,
						Zones: []core.AvailabilityZone{
							{Name: zoneName},
							{Name: zoneName},
						},
					},
				}
			)

			BeforeEach(func() {
				cloudProfile = &core.CloudProfile{
					ObjectMeta: metadata,
					Spec: core.CloudProfileSpec{
						Type: "unknown",
						SeedSelector: &core.SeedSelector{
							LabelSelector: metav1.LabelSelector{
								MatchLabels: map[string]string{"foo": "bar"},
							},
						},
						Kubernetes: core.KubernetesSettings{
							Versions: []core.ExpirableVersion{{
								Version: "1.11.4",
							}},
						},
						MachineImages: []core.MachineImage{
							{
								Name: machineImageName,
								Versions: []core.MachineImageVersion{
									{
										ExpirableVersion: core.ExpirableVersion{
											Version: "1.2.3",
										},
										CRI:           []core.CRI{{Name: "containerd"}},
										Architectures: []string{"amd64"},
									},
								},
								UpdateStrategy: &updateStrategyMajor,
							},
						},
						Regions: []core.Region{
							{
								Name: regionName,
								Zones: []core.AvailabilityZone{
									{Name: zoneName},
								},
							},
						},
						MachineTypes: machineTypesConstraint,
						VolumeTypes:  volumeTypesConstraint,
					},
				}
			})

			It("should not return any errors", func() {
				errorList := ValidateCloudProfile(cloudProfile)

				Expect(errorList).To(BeEmpty())
			})

			DescribeTable("CloudProfile metadata",
				func(objectMeta metav1.ObjectMeta, matcher gomegatypes.GomegaMatcher) {
					cloudProfile.ObjectMeta = objectMeta

					errorList := ValidateCloudProfile(cloudProfile)

					Expect(errorList).To(matcher)
				},

				Entry("should forbid CloudProfile with empty metadata",
					metav1.ObjectMeta{},
					ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal("metadata.name"),
					}))),
				),
				Entry("should forbid CloudProfile with empty name",
					metav1.ObjectMeta{Name: ""},
					ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal("metadata.name"),
					}))),
				),
				Entry("should allow CloudProfile with '.' in the name",
					metav1.ObjectMeta{Name: "profile.test"},
					BeEmpty(),
				),
				Entry("should forbid CloudProfile with '_' in the name (not a DNS-1123 subdomain)",
					metav1.ObjectMeta{Name: "profile_test"},
					ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("metadata.name"),
					}))),
				),
			)

			It("should forbid not specifying a type", func() {
				cloudProfile.Spec.Type = ""

				errorList := ValidateCloudProfile(cloudProfile)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.type"),
				}))))
			})

			It("should forbid ca bundles with unsupported format", func() {
				cloudProfile.Spec.CABundle = ptr.To("unsupported")

				errorList := ValidateCloudProfile(cloudProfile)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.caBundle"),
				}))))
			})

			Context("kubernetes version constraints", func() {
				It("should enforce that at least one version has been defined", func() {
					cloudProfile.Spec.Kubernetes.Versions = []core.ExpirableVersion{}

					errorList := ValidateCloudProfile(cloudProfile)

					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal("spec.kubernetes.versions"),
					})), PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("spec.kubernetes.versions"),
					}))))
				})

				It("should forbid versions of a not allowed pattern", func() {
					cloudProfile.Spec.Kubernetes.Versions = []core.ExpirableVersion{{Version: "1.11"}}

					errorList := ValidateCloudProfile(cloudProfile)

					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("spec.kubernetes.versions[0]"),
					}))))
				})

				It("should forbid expiration date on latest kubernetes version", func() {
					expirationDate := &metav1.Time{Time: time.Now().AddDate(0, 0, 1)}
					cloudProfile.Spec.Kubernetes.Versions = []core.ExpirableVersion{
						{
							Version:        "1.1.0",
							Classification: &supportedClassification,
						},
						{
							Version:        "1.2.0",
							Classification: &deprecatedClassification,
							ExpirationDate: expirationDate,
						},
					}

					errorList := ValidateCloudProfile(cloudProfile)

					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("spec.kubernetes.versions[].expirationDate"),
					}))))
				})

				It("should forbid duplicated kubernetes versions", func() {
					cloudProfile.Spec.Kubernetes = duplicatedKubernetes

					errorList := ValidateCloudProfile(cloudProfile)

					Expect(errorList).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeDuplicate),
							"Field": Equal(fmt.Sprintf("spec.kubernetes.versions[%d].version", len(duplicatedKubernetes.Versions)-1)),
						}))))
				})

				It("should forbid invalid classification for kubernetes versions", func() {
					classification := core.VersionClassification("dummy")
					cloudProfile.Spec.Kubernetes.Versions = []core.ExpirableVersion{
						{
							Version:        "1.1.0",
							Classification: &classification,
						},
					}

					errorList := ValidateCloudProfile(cloudProfile)

					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":     Equal(field.ErrorTypeNotSupported),
						"Field":    Equal("spec.kubernetes.versions[0].classification"),
						"BadValue": Equal(classification),
					}))))
				})

				It("only allow one supported version per minor version", func() {
					cloudProfile.Spec.Kubernetes.Versions = []core.ExpirableVersion{
						{
							Version:        "1.1.0",
							Classification: &supportedClassification,
						},
						{
							Version:        "1.1.1",
							Classification: &supportedClassification,
						},
					}
					errorList := ValidateCloudProfile(cloudProfile)

					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeForbidden),
						"Field": Equal("spec.kubernetes.versions[1]"),
					})), PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeForbidden),
						"Field": Equal("spec.kubernetes.versions[0]"),
					}))))
				})
			})

			Context("machine image validation", func() {
				It("should forbid an empty list of machine images", func() {
					cloudProfile.Spec.MachineImages = []core.MachineImage{}

					errorList := ValidateCloudProfile(cloudProfile)

					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal("spec.machineImages"),
					}))))
				})

				It("should forbid duplicate names in list of machine images", func() {
					cloudProfile.Spec.MachineImages = []core.MachineImage{
						{
							Name: machineImageName,
							Versions: []core.MachineImageVersion{
								{
									ExpirableVersion: core.ExpirableVersion{
										Version:        "3.4.6",
										Classification: &supportedClassification,
									},
									CRI:           []core.CRI{{Name: "containerd"}},
									Architectures: []string{"amd64"},
								},
							},
							UpdateStrategy: &updateStrategyMajor,
						},
						{
							Name: machineImageName,
							Versions: []core.MachineImageVersion{
								{
									ExpirableVersion: core.ExpirableVersion{
										Version:        "3.4.5",
										Classification: &previewClassification,
									},
									CRI:           []core.CRI{{Name: "containerd"}},
									Architectures: []string{"amd64"},
								},
							},
							UpdateStrategy: &updateStrategyMajor,
						},
					}

					errorList := ValidateCloudProfile(cloudProfile)

					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeDuplicate),
						"Field": Equal("spec.machineImages[1]"),
					}))))
				})

				It("should forbid machine images with no version", func() {
					cloudProfile.Spec.MachineImages = []core.MachineImage{
						{Name: machineImageName},
					}

					errorList := ValidateCloudProfile(cloudProfile)

					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeRequired),
						"Field":  Equal("spec.machineImages[0].versions"),
						"Detail": ContainSubstring("must provide at least one version for the machine image 'some-machine-image'"),
					})), PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeRequired),
						"Field":  Equal("spec.machineImages[0].updateStrategy"),
						"Detail": ContainSubstring("must provide an update strategy"),
					}))))
				})

				It("should forbid machine images with an invalid machine image update strategy", func() {
					updateStrategy := core.MachineImageUpdateStrategy("dummy")
					cloudProfile.Spec.MachineImages = []core.MachineImage{
						{
							Name: machineImageName,
							Versions: []core.MachineImageVersion{
								{
									ExpirableVersion: core.ExpirableVersion{
										Version:        "3.4.6",
										Classification: &supportedClassification,
									},
									CRI:           []core.CRI{{Name: "containerd"}},
									Architectures: []string{"amd64"},
								},
							},
							UpdateStrategy: &updateStrategy,
						},
					}

					errorList := ValidateCloudProfile(cloudProfile)

					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeNotSupported),
						"Field": Equal("spec.machineImages[0].updateStrategy"),
					}))))
				})

				It("should allow machine images with a valid machine image update strategy", func() {
					updateStrategy := core.UpdateStrategyMinor
					cloudProfile.Spec.MachineImages = []core.MachineImage{
						{
							Name: machineImageName,
							Versions: []core.MachineImageVersion{
								{
									ExpirableVersion: core.ExpirableVersion{
										Version:        "3.4.6",
										Classification: &supportedClassification,
									},
									CRI:           []core.CRI{{Name: "containerd"}},
									Architectures: []string{"amd64"},
								},
							},
							UpdateStrategy: &updateStrategy,
						},
					}

					errorList := ValidateCloudProfile(cloudProfile)

					Expect(errorList).To(BeEmpty())
				})

				It("should forbid nonSemVer machine image versions", func() {
					cloudProfile.Spec.MachineImages = []core.MachineImage{
						{
							Name: machineImageName,
							Versions: []core.MachineImageVersion{
								{
									ExpirableVersion: core.ExpirableVersion{
										Version:        "0.1.2",
										Classification: &supportedClassification,
									},
									CRI:           []core.CRI{{Name: "containerd"}},
									Architectures: []string{"amd64"},
								},
							},
							UpdateStrategy: &updateStrategyMajor,
						},
						{
							Name: "xy",
							Versions: []core.MachineImageVersion{
								{
									ExpirableVersion: core.ExpirableVersion{
										Version:        "a.b.c",
										Classification: &supportedClassification,
									},
									CRI:           []core.CRI{{Name: "containerd"}},
									Architectures: []string{"amd64"},
								},
							},
							UpdateStrategy: &updateStrategyMajor,
						},
					}

					errorList := ValidateCloudProfile(cloudProfile)

					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("spec.machineImages"),
					})), PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("spec.machineImages[1].versions[0].version"),
					}))))
				})

				It("should forbid non semver min supported version for in-place update", func() {
					cloudProfile.Spec.MachineImages = []core.MachineImage{
						{
							Name: machineImageName,
							Versions: []core.MachineImageVersion{
								{
									ExpirableVersion: core.ExpirableVersion{
										Version:        "0.1.2",
										Classification: &supportedClassification,
									},
									CRI:           []core.CRI{{Name: "containerd"}},
									Architectures: []string{"amd64"},
								},
							},
							UpdateStrategy: &updateStrategyMajor,
						},
						{
							Name: "xy",
							Versions: []core.MachineImageVersion{
								{
									ExpirableVersion: core.ExpirableVersion{
										Version:        "1.1.2",
										Classification: &supportedClassification,
									},
									CRI:           []core.CRI{{Name: "containerd"}},
									Architectures: []string{"amd64"},
									InPlaceUpdates: &core.InPlaceUpdates{
										MinVersionForUpdate: ptr.To("a.b.c"),
									},
								},
							},
							UpdateStrategy: &updateStrategyMajor,
						},
					}

					errorList := ValidateCloudProfile(cloudProfile)

					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.machineImages[1].versions[0].minVersionForInPlaceUpdate"),
						"Detail": Equal("could not parse version. Use a semantic version."),
					}))))
				})

				It("should allow expiration date on latest machine image version", func() {
					expirationDate := &metav1.Time{Time: time.Now().AddDate(0, 0, 1)}
					cloudProfile.Spec.MachineImages = []core.MachineImage{
						{
							Name: machineImageName,
							Versions: []core.MachineImageVersion{
								{
									ExpirableVersion: core.ExpirableVersion{
										Version:        "0.1.2",
										ExpirationDate: expirationDate,
										Classification: &previewClassification,
									},
									CRI:           []core.CRI{{Name: "containerd"}},
									Architectures: []string{"amd64"},
								},
								{
									ExpirableVersion: core.ExpirableVersion{
										Version:        "0.1.1",
										Classification: &supportedClassification,
									},
									CRI:           []core.CRI{{Name: "containerd"}},
									Architectures: []string{"amd64"},
								},
							},
							UpdateStrategy: &updateStrategyMajor,
						},
						{
							Name: "xy",
							Versions: []core.MachineImageVersion{
								{
									ExpirableVersion: core.ExpirableVersion{
										Version:        "0.1.1",
										ExpirationDate: expirationDate,
										Classification: &supportedClassification,
									},
									CRI:           []core.CRI{{Name: "containerd"}},
									Architectures: []string{"amd64"},
								},
							},
							UpdateStrategy: &updateStrategyMajor,
						},
					}

					errorList := ValidateCloudProfile(cloudProfile)
					Expect(errorList).To(BeEmpty())
				})

				It("should forbid invalid classification for machine image versions", func() {
					classification := core.VersionClassification("dummy")
					cloudProfile.Spec.MachineImages = []core.MachineImage{
						{
							Name: machineImageName,
							Versions: []core.MachineImageVersion{
								{
									ExpirableVersion: core.ExpirableVersion{
										Version:        "0.1.2",
										Classification: &classification,
									},
									CRI:           []core.CRI{{Name: "containerd"}},
									Architectures: []string{"amd64"},
								},
							},
							UpdateStrategy: &updateStrategyMajor,
						},
					}

					errorList := ValidateCloudProfile(cloudProfile)
					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":     Equal(field.ErrorTypeNotSupported),
						"Field":    Equal("spec.machineImages[0].versions[0].classification"),
						"BadValue": Equal(classification),
					}))))
				})

				It("should allow valid CPU architecture for machine image versions", func() {
					cloudProfile.Spec.MachineImages = []core.MachineImage{
						{
							Name: machineImageName,
							Versions: []core.MachineImageVersion{
								{
									ExpirableVersion: core.ExpirableVersion{
										Version: "0.1.2",
									},
									CRI:           []core.CRI{{Name: "containerd"}},
									Architectures: []string{"amd64", "arm64"},
								},
								{
									ExpirableVersion: core.ExpirableVersion{
										Version: "0.1.3",
									},
									CRI:           []core.CRI{{Name: "containerd"}},
									Architectures: []string{"amd64"},
								},
								{
									ExpirableVersion: core.ExpirableVersion{
										Version: "0.1.4",
									},
									CRI:           []core.CRI{{Name: "containerd"}},
									Architectures: []string{"arm64"},
								},
							},
							UpdateStrategy: &updateStrategyMajor,
						},
					}

					errorList := ValidateCloudProfile(cloudProfile)
					Expect(errorList).To(BeEmpty())
				})

				It("should forbid invalid CPU architecture for machine image versions", func() {
					cloudProfile.Spec.MachineImages = []core.MachineImage{
						{
							Name: machineImageName,
							Versions: []core.MachineImageVersion{
								{
									ExpirableVersion: core.ExpirableVersion{
										Version: "0.1.2",
									},
									CRI:           []core.CRI{{Name: "containerd"}},
									Architectures: []string{"foo"},
								},
							},
							UpdateStrategy: &updateStrategyMajor,
						},
					}

					errorList := ValidateCloudProfile(cloudProfile)
					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeNotSupported),
						"Field": Equal("spec.machineImages[0].versions[0].architectures[0]"),
					}))))
				})

				It("should allow valid kubeletVersionConstraint for machine image versions", func() {
					cloudProfile.Spec.MachineImages = []core.MachineImage{
						{
							Name: machineImageName,
							Versions: []core.MachineImageVersion{
								{
									ExpirableVersion: core.ExpirableVersion{
										Version: "0.1.2",
									},
									CRI:                      []core.CRI{{Name: "containerd"}},
									KubeletVersionConstraint: ptr.To("< 1.26"),
									Architectures:            []string{"amd64"},
								},
								{
									ExpirableVersion: core.ExpirableVersion{
										Version: "0.1.3",
									},
									CRI:                      []core.CRI{{Name: "containerd"}},
									KubeletVersionConstraint: ptr.To(">= 1.26"),
									Architectures:            []string{"amd64"},
								},
							},
							UpdateStrategy: &updateStrategyMajor,
						},
					}

					errorList := ValidateCloudProfile(cloudProfile)
					Expect(errorList).To(BeEmpty())
				})

				It("should forbid invalid kubeletVersionConstraint for machine image versions", func() {
					cloudProfile.Spec.MachineImages = []core.MachineImage{
						{
							Name: machineImageName,
							Versions: []core.MachineImageVersion{
								{
									ExpirableVersion: core.ExpirableVersion{
										Version: "0.1.2",
									},
									CRI:                      []core.CRI{{Name: "containerd"}},
									KubeletVersionConstraint: ptr.To(""),
									Architectures:            []string{"amd64"},
								},
								{
									ExpirableVersion: core.ExpirableVersion{
										Version: "0.1.3",
									},
									CRI:                      []core.CRI{{Name: "containerd"}},
									KubeletVersionConstraint: ptr.To("invalid-version"),
									Architectures:            []string{"amd64"},
								},
							},
							UpdateStrategy: &updateStrategyMajor,
						},
					}

					errorList := ValidateCloudProfile(cloudProfile)
					Expect(errorList).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeInvalid),
							"Field": Equal("spec.machineImages[0].versions[0].kubeletVersionConstraint"),
						})),
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeInvalid),
							"Field": Equal("spec.machineImages[0].versions[1].kubeletVersionConstraint"),
						})),
					))
				})
			})

			It("should forbid if no cri is present", func() {
				cloudProfile.Spec.MachineImages[0].Versions[0].CRI = nil

				errorList := ValidateCloudProfile(cloudProfile)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.machineImages[0].versions[0].cri"),
				}))))
			})

			It("should forbid non-supported container runtime interface names", func() {
				cloudProfile.Spec.MachineImages = []core.MachineImage{
					{
						Name: "invalid-cri-name",
						Versions: []core.MachineImageVersion{
							{
								ExpirableVersion: core.ExpirableVersion{
									Version: "0.1.2",
								},
								CRI: []core.CRI{
									{
										Name: "invalid-cri-name",
									},
									{
										Name: "docker",
									},
								},
								Architectures: []string{"amd64"},
							},
						},
						UpdateStrategy: &updateStrategyMajor,
					},
					{
						Name: "valid-cri-name",
						Versions: []core.MachineImageVersion{
							{
								ExpirableVersion: core.ExpirableVersion{
									Version: "0.1.2",
								},
								CRI: []core.CRI{
									{
										Name: core.CRINameContainerD,
									},
								},
								Architectures: []string{"amd64"},
							},
						},
						UpdateStrategy: &updateStrategyMajor,
					},
				}

				errorList := ValidateCloudProfile(cloudProfile)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeNotSupported),
					"Field":  Equal("spec.machineImages[0].versions[0].cri[0].name"),
					"Detail": Equal("supported values: \"containerd\""),
				})), PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeNotSupported),
					"Field":  Equal("spec.machineImages[0].versions[0].cri[1].name"),
					"Detail": Equal("supported values: \"containerd\""),
				})),
				))
			})

			It("should forbid duplicated container runtime interface names", func() {
				cloudProfile.Spec.MachineImages[0].Versions[0].CRI = []core.CRI{
					{
						Name: core.CRINameContainerD,
					},
					{
						Name: core.CRINameContainerD,
					},
				}

				errorList := ValidateCloudProfile(cloudProfile)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeDuplicate),
					"Field": Equal("spec.machineImages[0].versions[0].cri[1]"),
				}))))
			})

			It("should forbid duplicated container runtime names", func() {
				cloudProfile.Spec.MachineImages[0].Versions[0].CRI = []core.CRI{
					{
						Name: core.CRINameContainerD,
						ContainerRuntimes: []core.ContainerRuntime{
							{
								Type: "cr1",
							},
							{
								Type: "cr1",
							},
						},
					},
				}

				errorList := ValidateCloudProfile(cloudProfile)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeDuplicate),
					"Field": Equal("spec.machineImages[0].versions[0].cri[0].containerRuntimes[1].type"),
				}))))
			})

			Context("machine types validation", func() {
				It("should enforce that at least one machine type has been defined", func() {
					cloudProfile.Spec.MachineTypes = []core.MachineType{}

					errorList := ValidateCloudProfile(cloudProfile)

					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal("spec.machineTypes"),
					}))))
				})

				It("should enforce uniqueness of machine type names", func() {
					cloudProfile.Spec.MachineTypes = []core.MachineType{
						cloudProfile.Spec.MachineTypes[0],
						cloudProfile.Spec.MachineTypes[0],
					}

					errorList := ValidateCloudProfile(cloudProfile)

					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeDuplicate),
						"Field": Equal("spec.machineTypes[1].name"),
					}))))
				})

				It("should forbid machine types with unsupported property values", func() {
					cloudProfile.Spec.MachineTypes = invalidMachineTypes

					errorList := ValidateCloudProfile(cloudProfile)

					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal("spec.machineTypes[0].name"),
					})), PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("spec.machineTypes[0].cpu"),
					})), PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("spec.machineTypes[0].gpu"),
					})), PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("spec.machineTypes[0].memory"),
					})), PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("spec.machineTypes[0].storage.minSize"),
					})), PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("spec.machineTypes[1].storage.size"),
					})), PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("spec.machineTypes[2].storage"),
					})), PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("spec.machineTypes[3].storage"),
					})), PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeNotSupported),
						"Field": Equal("spec.machineTypes[3].architecture"),
					})),
					))
				})

				It("should allow machine types with valid values", func() {
					cloudProfile.Spec.MachineTypes = machineTypesConstraint

					errorList := ValidateCloudProfile(cloudProfile)
					Expect(errorList).To(BeEmpty())
				})
			})

			Context("regions validation", func() {
				It("should forbid regions with unsupported name values", func() {
					cloudProfile.Spec.Regions = []core.Region{
						{
							Name:  "",
							Zones: []core.AvailabilityZone{{Name: ""}},
						},
					}

					errorList := ValidateCloudProfile(cloudProfile)

					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal("spec.regions[0].name"),
					})), PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal("spec.regions[0].zones[0].name"),
					}))))
				})

				It("should forbid duplicated region names", func() {
					cloudProfile.Spec.Regions = duplicatedRegions

					errorList := ValidateCloudProfile(cloudProfile)

					Expect(errorList).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeDuplicate),
							"Field": Equal(fmt.Sprintf("spec.regions[%d].name", len(duplicatedRegions)-1)),
						}))))
				})

				It("should forbid duplicated zone names", func() {
					cloudProfile.Spec.Regions = duplicatedZones

					errorList := ValidateCloudProfile(cloudProfile)

					Expect(errorList).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeDuplicate),
							"Field": Equal(fmt.Sprintf("spec.regions[0].zones[%d].name", len(duplicatedZones[0].Zones)-1)),
						}))))
				})

				It("should forbid invalid label specifications", func() {
					cloudProfile.Spec.Regions[0].Labels = map[string]string{
						"this-is-not-allowed": "?*&!@",
						"toolongtoolongtoolongtoolongtoolongtoolongtoolongtoolongtoolongtoolongtoolong": "toolongtoolongtoolongtoolongtoolongtoolongtoolongtoolongtoolongtoolongtoolong",
					}

					errorList := ValidateCloudProfile(cloudProfile)

					Expect(errorList).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeInvalid),
							"Field": Equal("spec.regions[0].labels"),
						})),
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeInvalid),
							"Field": Equal("spec.regions[0].labels"),
						})),
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeInvalid),
							"Field": Equal("spec.regions[0].labels"),
						})),
					))
				})
			})

			Context("volume types validation", func() {
				It("should enforce uniqueness of volume type names", func() {
					cloudProfile.Spec.VolumeTypes = []core.VolumeType{
						cloudProfile.Spec.VolumeTypes[0],
						cloudProfile.Spec.VolumeTypes[0],
					}

					errorList := ValidateCloudProfile(cloudProfile)

					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeDuplicate),
						"Field": Equal("spec.volumeTypes[1].name"),
					}))))
				})

				It("should forbid volume types with unsupported property values", func() {
					cloudProfile.Spec.VolumeTypes = invalidVolumeTypes

					errorList := ValidateCloudProfile(cloudProfile)

					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal("spec.volumeTypes[0].name"),
					})), PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal("spec.volumeTypes[0].class"),
					})), PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("spec.volumeTypes[0].minSize"),
					})),
					))
				})
			})

			Context("bastion validation", func() {
				It("should forbid empty bastion", func() {
					cloudProfile.Spec.Bastion = &core.Bastion{}

					errorList := ValidateCloudProfile(cloudProfile)

					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("spec.bastion"),
					}))))
				})

				It("should forbid unknown machineType", func() {
					cloudProfile.Spec.Bastion = &core.Bastion{
						MachineType: &core.BastionMachineType{Name: "unknown"},
					}

					errorList := ValidateCloudProfile(cloudProfile)

					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("spec.bastion.machineType.name"),
					}))))
				})

				It("should allow known machineType", func() {
					cloudProfile.Spec.Bastion = &core.Bastion{
						MachineType: &core.BastionMachineType{Name: machineType.Name},
					}

					errorList := ValidateCloudProfile(cloudProfile)
					Expect(errorList).To(BeEmpty())
				})

				It("should forbid unknown machineImage", func() {
					cloudProfile.Spec.Bastion = &core.Bastion{
						MachineImage: &core.BastionMachineImage{Name: "unknown"},
					}

					errorList := ValidateCloudProfile(cloudProfile)

					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("spec.bastion.machineImage.name"),
					}))))
				})

				It("should forbid preview image", func() {
					cloudProfile.Spec.Bastion = &core.Bastion{
						MachineImage: &core.BastionMachineImage{Name: machineImageName},
					}
					cloudProfile.Spec.MachineImages[0].Versions[0].Classification = &previewClassification
					cloudProfile.Spec.MachineImages[0].Versions[0].Architectures = []string{"amd64"}

					errorList := ValidateCloudProfile(cloudProfile)

					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("spec.bastion.machineImage.name"),
					}))))
				})

				It("should forbid no arch images", func() {
					cloudProfile.Spec.Bastion = &core.Bastion{
						MachineImage: &core.BastionMachineImage{Name: machineImageName},
					}
					cloudProfile.Spec.MachineImages[0].Versions[0].Classification = &supportedClassification
					cloudProfile.Spec.MachineImages[0].Versions[0].Architectures = []string{}

					errorList := ValidateCloudProfile(cloudProfile)

					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeRequired),
						"Field":  Equal("spec.machineImages[0].versions[0].architectures"),
						"Detail": Equal("must provide at least one architecture"),
					}))))
				})

				It("should allow images with supported classification and architecture specification", func() {
					cloudProfile.Spec.Bastion = &core.Bastion{
						MachineImage: &core.BastionMachineImage{Name: machineImageName},
					}
					cloudProfile.Spec.MachineImages[0].Versions[0].Classification = &supportedClassification
					cloudProfile.Spec.MachineImages[0].Versions[0].Architectures = []string{"amd64"}

					errorList := ValidateCloudProfile(cloudProfile)
					Expect(errorList).To(BeEmpty())
				})

				It("should allow matching arch of machineType and machineImage", func() {
					cloudProfile.Spec.Bastion = &core.Bastion{
						MachineType: &core.BastionMachineType{Name: machineType.Name},
						MachineImage: &core.BastionMachineImage{
							Name:    machineImageName,
							Version: ptr.To("1.2.3"),
						},
					}
					cloudProfile.Spec.MachineImages[0].Versions[0].Classification = &supportedClassification
					cloudProfile.Spec.MachineImages[0].Versions[0].Architectures = []string{*machineType.Architecture}

					errorList := ValidateCloudProfile(cloudProfile)
					Expect(errorList).To(BeEmpty())
				})

				It("should forbid different arch of machineType and machineImage", func() {
					cloudProfile.Spec.Bastion = &core.Bastion{
						MachineType: &core.BastionMachineType{Name: machineType.Name},
						MachineImage: &core.BastionMachineImage{
							Name:    machineImageName,
							Version: ptr.To("1.2.3"),
						},
					}
					cloudProfile.Spec.MachineImages[0].Versions[0].Classification = &supportedClassification
					cloudProfile.Spec.MachineImages[0].Versions[0].Architectures = []string{"arm64"}

					errorList := ValidateCloudProfile(cloudProfile)

					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("spec.bastion.machineImage.version"),
					}))))
				})

				It("should allow any image architecture if machineType is nil", func() {
					cloudProfile.Spec.Bastion = &core.Bastion{
						MachineType: nil,
						MachineImage: &core.BastionMachineImage{
							Name:    machineImageName,
							Version: ptr.To("1.2.3"),
						},
					}
					cloudProfile.Spec.MachineImages[0].Versions[0].Classification = &supportedClassification
					// architectures must be one of arm64 or amd64
					cloudProfile.Spec.MachineImages[0].Versions[0].Architectures = []string{"arm64"}

					errorList := ValidateCloudProfile(cloudProfile)
					Expect(errorList).To(BeEmpty())
				})
			})

			Context("limits validation", func() {
				It("should allow unset limits", func() {
					cloudProfile.Spec.Limits = nil

					Expect(ValidateCloudProfile(cloudProfile)).To(BeEmpty())
				})

				It("should allow empty limits", func() {
					cloudProfile.Spec.Limits = &core.Limits{}

					Expect(ValidateCloudProfile(cloudProfile)).To(BeEmpty())
				})

				It("should allow positive maxNodesTotal", func() {
					cloudProfile.Spec.Limits = &core.Limits{
						MaxNodesTotal: ptr.To[int32](100),
					}

					Expect(ValidateCloudProfile(cloudProfile)).To(BeEmpty())
				})

				It("should forbid zero maxNodesTotal", func() {
					cloudProfile.Spec.Limits = &core.Limits{
						MaxNodesTotal: ptr.To[int32](0),
					}

					Expect(ValidateCloudProfile(cloudProfile)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("spec.limits.maxNodesTotal"),
					}))))
				})

				It("should forbid negative maxNodesTotal", func() {
					cloudProfile.Spec.Limits = &core.Limits{
						MaxNodesTotal: ptr.To[int32](-1),
					}

					Expect(ValidateCloudProfile(cloudProfile)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("spec.limits.maxNodesTotal"),
					}))))
				})
			})

			It("should forbid unsupported seed selectors", func() {
				cloudProfile.Spec.SeedSelector.MatchLabels["foo"] = "no/slash/allowed"

				errorList := ValidateCloudProfile(cloudProfile)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.seedSelector.matchLabels"),
				}))))
			})
		})

		Context("architecture capabilities", func() {
			var cloudProfile *core.CloudProfile

			BeforeEach(func() {
				cloudProfile = &core.CloudProfile{
					ObjectMeta: metav1.ObjectMeta{
						Name: "profile",
					},
					Spec: core.CloudProfileSpec{
						Type:       "test",
						Kubernetes: core.KubernetesSettings{Versions: []core.ExpirableVersion{{Version: "1.32.0"}}},
						Regions:    []core.Region{{Name: "unit-test"}},
						MachineImages: []core.MachineImage{
							{
								Name:           "image",
								UpdateStrategy: &updateStrategyMajor,
								Versions: []core.MachineImageVersion{
									{ExpirableVersion: core.ExpirableVersion{Version: "1.2.3"}, CRI: []core.CRI{{Name: core.CRINameContainerD}}},
								},
							},
						},
						MachineTypes: []core.MachineType{
							{Name: "machine"},
						},
					},
				}
			})

			Describe("using Capabilities", func() {
				BeforeEach(func() {
					DeferCleanup(test.WithFeatureGate(features.DefaultFeatureGate, features.CloudProfileCapabilities, true))

					cloudProfile.Spec.Capabilities = []core.CapabilitySet{
						{Capabilities: core.Capabilities{"architecture": []string{"amd64", "arm64"}}},
						{Capabilities: core.Capabilities{"anotherCapability": []string{"value1"}}},
					}
				})

				It("should succeed to validate with neither architectures nor capabilities set for machine images", func() {
					cloudProfile.Spec.MachineTypes[0].Architecture = ptr.To("arm64")
					Expect(ValidateCloudProfile(cloudProfile)).To(BeEmpty())
				})

				It("should fail to validate with no architecture capability defined in a machine image capability set", func() {
					cloudProfile.Spec.MachineTypes[0].Architecture = ptr.To("arm64")
					cloudProfile.Spec.MachineImages[0].Versions[0].CapabilitySets = []core.CapabilitySet{
						{Capabilities: core.Capabilities{"anotherCapability": []string{"value1"}}},
					}

					Expect(ValidateCloudProfile(cloudProfile)).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":   Equal(field.ErrorTypeRequired),
							"Field":  Equal("spec.machineImages[0].versions[0].capabilitySets[0].architecture"),
							"Detail": Equal("must provide at least one architecture"),
						})),
					))
				})

				It("should fail to validate with only architectures set", func() {
					cloudProfile.Spec.MachineImages[0].Versions[0].Architectures = []string{"arm64"}
					cloudProfile.Spec.MachineTypes[0].Architecture = ptr.To("arm64")

					Expect(ValidateCloudProfile(cloudProfile)).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":     Equal(field.ErrorTypeInvalid),
							"Field":    Equal("spec.machineImages[0].versions[0].architectures"),
							"BadValue": Equal([]string{"arm64"}),
							"Detail":   Equal("architecture field values set (arm64) conflict with the capability architectures ()"),
						})),
					))
				})

				It("should successfully validate with only capabilities set", func() {
					cloudProfile.Spec.MachineImages[0].Versions[0].CapabilitySets = []core.CapabilitySet{
						{Capabilities: core.Capabilities{"architecture": []string{"arm64"}}},
						{Capabilities: core.Capabilities{"architecture": []string{"amd64"}}},
					}
					cloudProfile.Spec.MachineTypes[0].Capabilities = core.Capabilities{
						"architecture": []string{"arm64"},
					}

					Expect(ValidateCloudProfile(cloudProfile)).To(BeEmpty())
				})

				It("should fail to validate with multiple architectures set in one machine image capability set", func() {
					cloudProfile.Spec.MachineImages[0].Versions[0].CapabilitySets = []core.CapabilitySet{
						{Capabilities: core.Capabilities{"architecture": []string{"arm64", "amd64"}}},
					}
					cloudProfile.Spec.MachineTypes[0].Capabilities = core.Capabilities{
						"architecture": []string{"arm64"},
					}

					Expect(ValidateCloudProfile(cloudProfile)).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":     Equal(field.ErrorTypeInvalid),
							"Field":    Equal("spec.machineImages[0].versions[0].capabilitySets[0].architecture"),
							"BadValue": BeEquivalentTo([]string{"arm64", "amd64"}),
							"Detail":   Equal("must not define more than one architecture within one capability set"),
						})),
					))
				})

				It("should fail to validate with multiple architectures set for machine type capabilities", func() {
					cloudProfile.Spec.MachineTypes[0].Capabilities = core.Capabilities{
						"architecture": []string{"arm64", "amd64"},
					}

					Expect(ValidateCloudProfile(cloudProfile)).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":     Equal(field.ErrorTypeInvalid),
							"Field":    Equal("spec.machineTypes[0].capabilities.architecture"),
							"BadValue": BeEquivalentTo([]string{"arm64", "amd64"}),
							"Detail":   Equal("must not define more than one architecture"),
						})),
					))
				})

				It("should fail to validate with unknown architecture capabilities set", func() {
					cloudProfile.Spec.MachineImages[0].Versions[0].CapabilitySets = []core.CapabilitySet{
						{Capabilities: core.Capabilities{"architecture": []string{"amd64"}}},
						{Capabilities: core.Capabilities{"architecture": []string{"other"}}},
					}
					cloudProfile.Spec.MachineTypes[0].Capabilities = core.Capabilities{
						"architecture": []string{"other"},
					}

					Expect(ValidateCloudProfile(cloudProfile)).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":     Equal(field.ErrorTypeNotSupported),
							"Field":    Equal("spec.machineImages[0].versions[0].capabilitySets[1].architecture[0]"),
							"BadValue": Equal("other"),
							"Detail":   Equal(`supported values: "amd64", "arm64"`),
						})), PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":     Equal(field.ErrorTypeNotSupported),
							"Field":    Equal("spec.machineTypes[0].capabilities.architecture[0]"),
							"BadValue": Equal("other"),
							"Detail":   Equal(`supported values: "amd64", "arm64"`),
						})),
					))
				})

				It("should fail to validate capabilities with empty keys or values", func() {
					cloudProfile.Spec.Capabilities = []core.CapabilitySet{
						{Capabilities: core.Capabilities{"": core.CapabilityValues{}}},
						{Capabilities: core.Capabilities{"hasNoValues": []string{}}},
						{Capabilities: core.Capabilities{"hasEmptyValues": []string{""}}},
					}

					Expect(ValidateCloudProfile(cloudProfile)).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":     Equal(field.ErrorTypeRequired),
							"Field":    Equal("spec.machineTypes[0].architecture"),
							"BadValue": Equal(""),
							"Detail":   Equal("must provide an architecture"),
						})), PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":     Equal(field.ErrorTypeRequired),
							"Field":    Equal("spec.capabilities.architecture"),
							"BadValue": Equal(""),
							"Detail":   Equal("architecture capability is required"),
						})), PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":     Equal(field.ErrorTypeRequired),
							"Field":    Equal("spec.capabilities.hasNoValues"),
							"BadValue": Equal(""),
							"Detail":   Equal("capability values must not be empty"),
						})), PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":     Equal(field.ErrorTypeRequired),
							"Field":    Equal("spec.capabilities.hasEmptyValues[0]"),
							"BadValue": Equal(""),
							"Detail":   Equal("capability values must not be empty"),
						})), PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":     Equal(field.ErrorTypeRequired),
							"Field":    Equal("spec.capabilities"),
							"BadValue": Equal(""),
							"Detail":   Equal("capability keys must not be empty"),
						})), PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":     Equal(field.ErrorTypeRequired),
							"Field":    Equal("spec.capabilities[]"),
							"BadValue": Equal(""),
							"Detail":   Equal("capability values must not be empty"),
						})),
					))
				})

				It("should fail to validate invalid architecture capability values", func() {
					cloudProfile.Spec.Capabilities = []core.CapabilitySet{
						{Capabilities: core.Capabilities{"architecture": []string{"arm64", "custom"}}},
					}

					Expect(ValidateCloudProfile(cloudProfile)).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":     Equal(field.ErrorTypeRequired),
							"Field":    Equal("spec.machineTypes[0].architecture"),
							"BadValue": Equal(""),
							"Detail":   Equal("must provide an architecture"),
						})), PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":     Equal(field.ErrorTypeInvalid),
							"Field":    Equal("spec.capabilities.architecture"),
							"BadValue": Equal("custom"),
							"Detail":   Equal("allowed architectures are: amd64, arm64"),
						})),
					))
				})

				It("should fail to validate if the global capability definition does not define each capability once in its own set", func() {
					cloudProfile.Spec.MachineImages[0].Versions[0].CapabilitySets = []core.CapabilitySet{
						{Capabilities: core.Capabilities{"architecture": []string{"arm64"}}},
					}
					cloudProfile.Spec.MachineTypes[0].Capabilities = core.Capabilities{
						"architecture": []string{"arm64"},
					}

					cloudProfile.Spec.Capabilities = []core.CapabilitySet{
						{Capabilities: core.Capabilities{"architecture": []string{"arm64", "amd64"}}},
						{Capabilities: core.Capabilities{"architecture": []string{"arm64"}, "someKey": []string{"value1", "value2"}}},
					}

					Expect(ValidateCloudProfile(cloudProfile)).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":     Equal(field.ErrorTypeInvalid),
							"Field":    Equal("spec.capabilities[1]"),
							"BadValue": Equal("architecture"),
							"Detail":   Equal("each capability must only be defined once"),
						})), PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":     Equal(field.ErrorTypeInvalid),
							"Field":    Equal("spec.capabilities[1]"),
							"BadValue": HaveLen(2),
							"Detail":   Equal("must have exactly one capability"),
						})),
					))
				})

				Describe("should validate that the architectures do not conflict", func() {
					It("should succeed if both architectures and capabilities set the same values", func() {
						cloudProfile.Spec.MachineImages[0].Versions[0].CapabilitySets = []core.CapabilitySet{
							{Capabilities: core.Capabilities{"architecture": []string{"arm64"}}},
						}
						cloudProfile.Spec.MachineTypes[0].Capabilities = core.Capabilities{
							"architecture": []string{"arm64"},
						}

						cloudProfile.Spec.MachineImages[0].Versions[0].Architectures = []string{"arm64"}
						cloudProfile.Spec.MachineTypes[0].Architecture = ptr.To("arm64")

						Expect(ValidateCloudProfile(cloudProfile)).To(BeEmpty())
					})

					It("should succeed if both architectures and split-up capabilities set the same values", func() {
						cloudProfile.Spec.MachineImages[0].Versions[0].CapabilitySets = []core.CapabilitySet{
							{Capabilities: core.Capabilities{"architecture": []string{"amd64"}}},
							{Capabilities: core.Capabilities{"architecture": []string{"arm64"}}},
						}
						cloudProfile.Spec.MachineTypes[0].Capabilities = core.Capabilities{
							"architecture": []string{"arm64"},
						}

						cloudProfile.Spec.MachineImages[0].Versions[0].Architectures = []string{"arm64", "amd64"}
						cloudProfile.Spec.MachineTypes[0].Architecture = ptr.To("arm64")

						Expect(ValidateCloudProfile(cloudProfile)).To(BeEmpty())
					})

					It("should fail if the values in architectures and capabilities conflict", func() {
						cloudProfile.Spec.MachineImages[0].Versions[0].CapabilitySets = []core.CapabilitySet{
							{Capabilities: core.Capabilities{"architecture": []string{"arm64"}}},
						}
						cloudProfile.Spec.MachineTypes[0].Capabilities = core.Capabilities{
							"architecture": []string{"amd64"},
						}

						cloudProfile.Spec.MachineImages[0].Versions[0].Architectures = []string{"amd64", "arm64"}
						cloudProfile.Spec.MachineTypes[0].Architecture = ptr.To("arm64")

						Expect(ValidateCloudProfile(cloudProfile)).To(ConsistOf(
							PointTo(MatchFields(IgnoreExtras, Fields{
								"Type":     Equal(field.ErrorTypeInvalid),
								"Field":    Equal("spec.machineImages[0].versions[0].architectures"),
								"BadValue": ConsistOf("amd64", "arm64"),
								"Detail":   Equal(`architecture field values set (amd64,arm64) conflict with the capability architectures (arm64)`),
							})), PointTo(MatchFields(IgnoreExtras, Fields{
								"Type":     Equal(field.ErrorTypeInvalid),
								"Field":    Equal("spec.machineTypes[0].capabilities.architecture[0]"),
								"BadValue": Equal("amd64"),
								"Detail":   Equal(`machine type architecture (arm64) conflicts with the capability architecture (amd64)`),
							})),
						))
					})
				})
			})

			Describe("not using Capabilities", func() {
				It("should fail to validate with no architectures set", func() {
					Expect(ValidateCloudProfile(cloudProfile)).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":   Equal(field.ErrorTypeRequired),
							"Field":  Equal("spec.machineImages[0].versions[0].architectures"),
							"Detail": Equal("must provide at least one architecture"),
						})), PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":   Equal(field.ErrorTypeRequired),
							"Field":  Equal("spec.machineTypes[0].architecture"),
							"Detail": Equal("must provide an architecture"),
						})),
					))
				})

				It("should fail to validate provided capabilities with disabled feature gate", func() {
					cloudProfile.Spec.Capabilities = []core.CapabilitySet{
						{Capabilities: core.Capabilities{"architecture": []string{"amd64"}}},
					}

					Expect(ValidateCloudProfile(cloudProfile)).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":     Equal(field.ErrorTypeRequired),
							"Field":    Equal("spec.machineTypes[0].architecture"),
							"BadValue": Equal(""),
							"Detail":   Equal("must provide an architecture"),
						})), PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":   Equal(field.ErrorTypeForbidden),
							"Field":  Equal("spec.capabilities"),
							"Detail": Equal("capabilities are not allowed with disabled CloudProfileCapabilities feature gate"),
						})),
					))
				})

				It("should successfully validate with only architectures set", func() {
					cloudProfile.Spec.MachineImages[0].Versions[0].Architectures = []string{"arm64"}
					cloudProfile.Spec.MachineTypes[0].Architecture = ptr.To("arm64")

					Expect(ValidateCloudProfile(cloudProfile)).To(BeEmpty())
				})

				It("should fail to validate with only capabilities set", func() {
					cloudProfile.Spec.MachineImages[0].Versions[0].CapabilitySets = []core.CapabilitySet{
						{Capabilities: core.Capabilities{"architecture": []string{"arm64"}}},
					}
					cloudProfile.Spec.MachineTypes[0].Capabilities = core.Capabilities{
						"architecture": []string{"arm64"},
					}

					Expect(ValidateCloudProfile(cloudProfile)).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":   Equal(field.ErrorTypeRequired),
							"Field":  Equal("spec.machineImages[0].versions[0].architectures"),
							"Detail": Equal("must provide at least one architecture"),
						})), PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":   Equal(field.ErrorTypeForbidden),
							"Field":  Equal("spec.machineImages[0].versions[0].capabilitySets"),
							"Detail": Equal("must not provide capabilities without global definition"),
						})), PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":   Equal(field.ErrorTypeRequired),
							"Field":  Equal("spec.machineTypes[0].architecture"),
							"Detail": Equal("must provide an architecture"),
						})), PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":   Equal(field.ErrorTypeForbidden),
							"Field":  Equal("spec.machineTypes[0].capabilities"),
							"Detail": Equal("must not provide capabilities without global definition"),
						})),
					))
				})

				It("should fail to validate with unknown architectures set", func() {
					cloudProfile.Spec.MachineImages[0].Versions[0].Architectures = []string{"amd64", "other"}
					cloudProfile.Spec.MachineTypes[0].Architecture = ptr.To("other")

					Expect(ValidateCloudProfile(cloudProfile)).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":     Equal(field.ErrorTypeNotSupported),
							"Field":    Equal("spec.machineImages[0].versions[0].architectures[1]"),
							"BadValue": Equal("other"),
							"Detail":   Equal(`supported values: "amd64", "arm64"`),
						})), PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":     Equal(field.ErrorTypeNotSupported),
							"Field":    Equal("spec.machineTypes[0].architecture"),
							"BadValue": Equal("other"),
							"Detail":   Equal(`supported values: "amd64", "arm64"`),
						})),
					))
				})
			})
		})
	})

	Describe("#ValidateCloudProfileUpdate", func() {
		var (
			cloudProfileNew *core.CloudProfile
			cloudProfileOld *core.CloudProfile
			dateInThePast   = &metav1.Time{Time: time.Now().AddDate(-5, 0, 0)}
		)

		BeforeEach(func() {
			cloudProfileNew = &core.CloudProfile{
				ObjectMeta: metav1.ObjectMeta{
					ResourceVersion: "dummy",
					Name:            "dummy",
				},
				Spec: core.CloudProfileSpec{
					Type: "aws",
					MachineImages: []core.MachineImage{
						{
							Name: machineImageName,
							Versions: []core.MachineImageVersion{
								{
									ExpirableVersion: core.ExpirableVersion{
										Version:        "1.2.3",
										Classification: &supportedClassification,
									},
									CRI:           []core.CRI{{Name: "containerd"}},
									Architectures: []string{"amd64"},
								},
							},
							UpdateStrategy: &updateStrategyMajor,
						},
					},
					Kubernetes: core.KubernetesSettings{
						Versions: []core.ExpirableVersion{
							{
								Version:        "1.17.2",
								Classification: &deprecatedClassification,
							},
						},
					},
					Regions: []core.Region{
						{
							Name: regionName,
							Zones: []core.AvailabilityZone{
								{Name: zoneName},
							},
						},
					},
					MachineTypes: machineTypesConstraint,
					VolumeTypes:  volumeTypesConstraint,
				},
			}
			cloudProfileOld = cloudProfileNew.DeepCopy()
		})

		Context("Removed Kubernetes versions", func() {
			It("deleting version - should not return any errors", func() {
				versions := []core.ExpirableVersion{
					{Version: "1.17.2", Classification: &deprecatedClassification},
					{Version: "1.17.1", Classification: &deprecatedClassification, ExpirationDate: dateInThePast},
					{Version: "1.17.0", Classification: &deprecatedClassification, ExpirationDate: dateInThePast},
				}
				cloudProfileNew.Spec.Kubernetes.Versions = versions[0:1]
				cloudProfileOld.Spec.Kubernetes.Versions = versions
				errorList := ValidateCloudProfileUpdate(cloudProfileNew, cloudProfileOld)

				Expect(errorList).To(BeEmpty())
			})
		})

		Context("Removed MachineImage versions", func() {
			It("deleting version - should not return any errors", func() {
				versions := []core.MachineImageVersion{
					{
						ExpirableVersion: core.ExpirableVersion{
							Version:        "2135.6.2",
							Classification: &deprecatedClassification,
						},
						CRI:           []core.CRI{{Name: "containerd"}},
						Architectures: []string{"amd64"},
					},
					{
						ExpirableVersion: core.ExpirableVersion{
							Version:        "2135.6.1",
							Classification: &deprecatedClassification,
							ExpirationDate: dateInThePast,
						},
						CRI:           []core.CRI{{Name: "containerd"}},
						Architectures: []string{"amd64"},
					},
					{
						ExpirableVersion: core.ExpirableVersion{
							Version:        "2135.6.0",
							Classification: &deprecatedClassification,
							ExpirationDate: dateInThePast,
						},
						CRI:           []core.CRI{{Name: "containerd"}},
						Architectures: []string{"amd64"},
					},
				}
				cloudProfileNew.Spec.MachineImages[0].Versions = versions[0:1]
				cloudProfileOld.Spec.MachineImages[0].Versions = versions
				errorList := ValidateCloudProfileUpdate(cloudProfileNew, cloudProfileOld)

				Expect(errorList).To(BeEmpty())
			})
		})

		Context("limits validation", func() {
			It("should allow adding limits", func() {
				cloudProfileNew.Spec.Limits = &core.Limits{
					MaxNodesTotal: ptr.To[int32](100),
				}

				Expect(ValidateCloudProfileUpdate(cloudProfileNew, cloudProfileOld)).To(BeEmpty())
			})

			It("should allow removing limits", func() {
				cloudProfileOld.Spec.Limits = &core.Limits{
					MaxNodesTotal: ptr.To[int32](100),
				}

				Expect(ValidateCloudProfileUpdate(cloudProfileNew, cloudProfileOld)).To(BeEmpty())
			})

			It("should allow adding maxNodesTotal", func() {
				cloudProfileOld.Spec.Limits = &core.Limits{}
				cloudProfileNew.Spec.Limits = &core.Limits{
					MaxNodesTotal: ptr.To[int32](100),
				}

				Expect(ValidateCloudProfileUpdate(cloudProfileNew, cloudProfileOld)).To(BeEmpty())
			})

			It("should allow removing maxNodesTotal", func() {
				cloudProfileOld.Spec.Limits = &core.Limits{
					MaxNodesTotal: ptr.To[int32](100),
				}
				cloudProfileNew.Spec.Limits = &core.Limits{}

				Expect(ValidateCloudProfileUpdate(cloudProfileNew, cloudProfileOld)).To(BeEmpty())
			})

			It("should allow unchanged maxNodesTotal", func() {
				cloudProfileOld.Spec.Limits = &core.Limits{
					MaxNodesTotal: ptr.To[int32](100),
				}
				cloudProfileNew.Spec.Limits = &core.Limits{
					MaxNodesTotal: ptr.To[int32](100),
				}

				Expect(ValidateCloudProfileUpdate(cloudProfileNew, cloudProfileOld)).To(BeEmpty())
			})

			It("should allow increasing maxNodesTotal", func() {
				cloudProfileOld.Spec.Limits = &core.Limits{
					MaxNodesTotal: ptr.To[int32](100),
				}
				cloudProfileNew.Spec.Limits = &core.Limits{
					MaxNodesTotal: ptr.To[int32](1000),
				}

				Expect(ValidateCloudProfileUpdate(cloudProfileNew, cloudProfileOld)).To(BeEmpty())
			})

			It("should forbid decreasing maxNodesTotal", func() {
				cloudProfileOld.Spec.Limits = &core.Limits{
					MaxNodesTotal: ptr.To[int32](100),
				}
				cloudProfileNew.Spec.Limits = &core.Limits{
					MaxNodesTotal: ptr.To[int32](10),
				}

				Expect(ValidateCloudProfileUpdate(cloudProfileNew, cloudProfileOld)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.limits.maxNodesTotal"),
				}))))
			})
		})

		Context("architecture capabilities", func() {
			BeforeEach(func() {
				cloudProfileNew.Spec.MachineImages = []core.MachineImage{
					{
						Name:           "image",
						UpdateStrategy: &updateStrategyMajor,
						Versions: []core.MachineImageVersion{
							{ExpirableVersion: core.ExpirableVersion{Version: "1.2.3"}, CRI: []core.CRI{{Name: core.CRINameContainerD}}},
						},
					},
				}
				cloudProfileNew.Spec.MachineTypes = []core.MachineType{
					{Name: "machine"},
				}
				cloudProfileOld = cloudProfileNew.DeepCopy()
			})

			Describe("using Capabilities", func() {
				BeforeEach(func() {
					DeferCleanup(test.WithFeatureGate(features.DefaultFeatureGate, features.CloudProfileCapabilities, true))
				})

				It("should validate other capability values against global definition", func() {
					cloudProfileNew.Spec.Capabilities = []core.CapabilitySet{
						{Capabilities: core.Capabilities{"architecture": []string{"amd64", "arm64"}}},
						{Capabilities: core.Capabilities{"foo": []string{"bar", "baz"}}},
					}

					cloudProfileNew.Spec.MachineImages[0].Versions[0].CapabilitySets = []core.CapabilitySet{
						{Capabilities: core.Capabilities{
							"architecture": []string{"arm64"},
							"foo":          []string{"bar", "foobar"},
							"bar":          []string{"baz"},
						}},
					}
					cloudProfileNew.Spec.MachineTypes[0].Capabilities = core.Capabilities{
						"architecture": []string{"arm64"},
						"foo":          []string{"bar", "foobar"},
						"bar":          []string{"baz"},
					}

					Expect(ValidateCloudProfileUpdate(cloudProfileNew, cloudProfileOld)).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":     Equal(field.ErrorTypeNotSupported),
							"Field":    Equal("spec.machineImages[0].versions[0].capabilitySets[0].foo[1]"),
							"BadValue": Equal("foobar"),
							"Detail":   Equal(`supported values: "bar", "baz"`),
						})),
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":     Equal(field.ErrorTypeNotSupported),
							"Field":    Equal("spec.machineImages[0].versions[0].capabilitySets[0]"),
							"BadValue": Equal("bar"),
							"Detail":   And(ContainSubstring("supported values: "), ContainSubstring("architecture"), ContainSubstring("foo")),
						})),
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":     Equal(field.ErrorTypeNotSupported),
							"Field":    Equal("spec.machineTypes[0].capabilities.foo[1]"),
							"BadValue": Equal("foobar"),
							"Detail":   Equal(`supported values: "bar", "baz"`),
						})),
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":     Equal(field.ErrorTypeNotSupported),
							"Field":    Equal("spec.machineTypes[0].capabilities"),
							"BadValue": Equal("bar"),
							"Detail":   And(ContainSubstring("supported values: "), ContainSubstring("architecture"), ContainSubstring("foo")),
						})),
					))
				})

				DescribeTableSubtree("switching to Capabilities",
					func(isInitialSwitch bool) {
						BeforeEach(func() {
							cloudProfileNew.Spec.Capabilities = []core.CapabilitySet{
								{Capabilities: core.Capabilities{"architecture": []string{"amd64", "arm64"}}},
							}

							if !isInitialSwitch {
								cloudProfileOld.Spec.Capabilities = cloudProfileNew.Spec.Capabilities
							}
						})

						It("should successfully validate with neither architectures nor capabilities set for machine images", func() {
							cloudProfileNew.Spec.MachineTypes[0].Architecture = ptr.To("arm64")
							Expect(ValidateCloudProfileUpdate(cloudProfileNew, cloudProfileOld)).To(BeEmpty())
						})

						It("should fail to validate with only architectures set", func() {
							cloudProfileNew.Spec.MachineImages[0].Versions[0].Architectures = []string{"arm64"}
							cloudProfileNew.Spec.MachineTypes[0].Architecture = ptr.To("arm64")

							cloudProfileOld.Spec.MachineImages = cloudProfileNew.Spec.MachineImages
							cloudProfileOld.Spec.MachineTypes = cloudProfileNew.Spec.MachineTypes

							Expect(ValidateCloudProfileUpdate(cloudProfileNew, cloudProfileOld)).To(ConsistOf(
								PointTo(MatchFields(IgnoreExtras, Fields{
									"Type":     Equal(field.ErrorTypeInvalid),
									"Field":    Equal("spec.machineImages[0].versions[0].architectures"),
									"BadValue": Equal([]string{"arm64"}),
									"Detail":   Equal("architecture field values set (arm64) conflict with the capability architectures ()"),
								})),
							))
						})

						It("should successfully to validate with only capabilities set", func() {
							cloudProfileNew.Spec.MachineImages[0].Versions[0].CapabilitySets = []core.CapabilitySet{
								{Capabilities: core.Capabilities{"architecture": []string{"arm64"}}},
							}
							cloudProfileNew.Spec.MachineTypes[0].Capabilities = core.Capabilities{
								"architecture": []string{"arm64"},
							}

							cloudProfileOld.Spec.MachineImages = cloudProfileNew.Spec.MachineImages
							cloudProfileOld.Spec.MachineTypes = cloudProfileNew.Spec.MachineTypes

							Expect(ValidateCloudProfileUpdate(cloudProfileNew, cloudProfileOld)).To(BeEmpty())
						})

						It("should fail to validate with multiple architectures set for machine type capabilities", func() {
							cloudProfileNew.Spec.MachineTypes[0].Capabilities = core.Capabilities{
								"architecture": []string{"arm64", "amd64"},
							}

							cloudProfileOld.Spec.MachineImages = cloudProfileNew.Spec.MachineImages
							cloudProfileOld.Spec.MachineTypes = cloudProfileNew.Spec.MachineTypes

							Expect(ValidateCloudProfileUpdate(cloudProfileNew, cloudProfileOld)).To(ConsistOf(
								PointTo(MatchFields(IgnoreExtras, Fields{
									"Type":     Equal(field.ErrorTypeInvalid),
									"Field":    Equal("spec.machineTypes[0].capabilities.architecture"),
									"BadValue": BeEquivalentTo([]string{"arm64", "amd64"}),
									"Detail":   Equal("must not define more than one architecture"),
								})),
							))
						})

						It("should fail to validate with unknown architecture capabilities set", func() {
							cloudProfileNew.Spec.MachineImages[0].Versions[0].CapabilitySets = []core.CapabilitySet{
								{Capabilities: core.Capabilities{"architecture": []string{"amd64"}}},
								{Capabilities: core.Capabilities{"architecture": []string{"other"}}},
							}
							cloudProfileNew.Spec.MachineTypes[0].Capabilities = core.Capabilities{
								"architecture": []string{"other"},
							}

							cloudProfileOld.Spec.MachineImages = cloudProfileNew.Spec.MachineImages
							cloudProfileOld.Spec.MachineTypes = cloudProfileNew.Spec.MachineTypes

							Expect(ValidateCloudProfileUpdate(cloudProfileNew, cloudProfileOld)).To(ConsistOf(
								PointTo(MatchFields(IgnoreExtras, Fields{
									"Type":     Equal(field.ErrorTypeNotSupported),
									"Field":    Equal("spec.machineImages[0].versions[0].capabilitySets[1].architecture[0]"),
									"BadValue": Equal("other"),
									"Detail":   Equal(`supported values: "amd64", "arm64"`),
								})), PointTo(MatchFields(IgnoreExtras, Fields{
									"Type":     Equal(field.ErrorTypeNotSupported),
									"Field":    Equal("spec.machineTypes[0].capabilities.architecture[0]"),
									"BadValue": Equal("other"),
									"Detail":   Equal(`supported values: "amd64", "arm64"`),
								})),
							))
						})

						Describe("should validate that the architectures do not conflict", func() {
							It("should succeed if both architectures and capabilities are set to the same values", func() {
								cloudProfileNew.Spec.MachineImages[0].Versions[0].Architectures = []string{"arm64"}
								cloudProfileNew.Spec.MachineTypes[0].Architecture = ptr.To("arm64")

								cloudProfileOld.Spec.MachineImages = cloudProfileNew.Spec.MachineImages
								cloudProfileOld.Spec.MachineTypes = cloudProfileNew.Spec.MachineTypes

								cloudProfileNew.Spec.MachineImages[0].Versions[0].CapabilitySets = []core.CapabilitySet{
									{Capabilities: core.Capabilities{"architecture": []string{"arm64"}}},
								}
								cloudProfileNew.Spec.MachineTypes[0].Capabilities = core.Capabilities{
									"architecture": []string{"arm64"},
								}

								Expect(ValidateCloudProfileUpdate(cloudProfileNew, cloudProfileOld)).To(BeEmpty())
							})

							It("should fail if the values in architectures and capabilities conflict", func() {
								cloudProfileNew.Spec.MachineImages[0].Versions[0].Architectures = []string{"amd64"}
								cloudProfileNew.Spec.MachineTypes[0].Architecture = ptr.To("amd64")

								cloudProfileOld.Spec.MachineImages = cloudProfileNew.Spec.MachineImages
								cloudProfileOld.Spec.MachineTypes = cloudProfileNew.Spec.MachineTypes

								cloudProfileNew.Spec.MachineImages[0].Versions[0].CapabilitySets = []core.CapabilitySet{
									{Capabilities: core.Capabilities{"architecture": []string{"arm64"}}},
								}
								cloudProfileNew.Spec.MachineTypes[0].Capabilities = core.Capabilities{
									"architecture": []string{"arm64"},
								}

								Expect(ValidateCloudProfileUpdate(cloudProfileNew, cloudProfileOld)).To(ConsistOf(
									PointTo(MatchFields(IgnoreExtras, Fields{
										"Type":     Equal(field.ErrorTypeInvalid),
										"Field":    Equal("spec.machineImages[0].versions[0].architectures"),
										"BadValue": Equal([]string{"amd64"}),
										"Detail":   Equal(`architecture field values set (amd64) conflict with the capability architectures (arm64)`),
									})), PointTo(MatchFields(IgnoreExtras, Fields{
										"Type":     Equal(field.ErrorTypeInvalid),
										"Field":    Equal("spec.machineTypes[0].capabilities.architecture[0]"),
										"BadValue": Equal("arm64"),
										"Detail":   Equal(`machine type architecture (amd64) conflicts with the capability architecture (arm64)`),
									})),
								))
							})

							It("should succeed if the split-up values in capabilities and the architectures field are equal", func() {
								cloudProfileNew.Spec.MachineImages[0].Versions[0].Architectures = []string{"amd64", "arm64"}
								cloudProfileNew.Spec.MachineTypes[0].Architecture = ptr.To("amd64")

								cloudProfileOld.Spec.MachineImages = cloudProfileNew.Spec.MachineImages
								cloudProfileOld.Spec.MachineTypes = cloudProfileNew.Spec.MachineTypes

								cloudProfileNew.Spec.MachineImages[0].Versions[0].CapabilitySets = []core.CapabilitySet{
									{Capabilities: core.Capabilities{"architecture": []string{"arm64"}}},
									{Capabilities: core.Capabilities{"architecture": []string{"amd64"}}},
								}
								cloudProfileNew.Spec.MachineTypes[0].Capabilities = core.Capabilities{
									"architecture": []string{"amd64"},
								}

								Expect(ValidateCloudProfileUpdate(cloudProfileNew, cloudProfileOld)).To(BeEmpty())
							})
						})
					},
					Entry("switching to Capabilities", true),
					Entry("using Capabilities", false),
				)
			})

			Describe("not using Capabilities", func() {
				BeforeEach(func() {
					cloudProfileOld.Spec.MachineImages[0].Versions[0].Architectures = []string{"arm64"}
					cloudProfileOld.Spec.MachineTypes[0].Architecture = ptr.To("arm64")
				})

				It("should fail to validate with completely removed architectures", func() {
					cloudProfileNew.Spec.MachineImages[0].Versions[0].Architectures = []string{}
					cloudProfileNew.Spec.MachineTypes[0].Architecture = nil

					Expect(ValidateCloudProfile(cloudProfileNew)).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":   Equal(field.ErrorTypeRequired),
							"Field":  Equal("spec.machineImages[0].versions[0].architectures"),
							"Detail": Equal("must provide at least one architecture"),
						})), PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":   Equal(field.ErrorTypeRequired),
							"Field":  Equal("spec.machineTypes[0].architecture"),
							"Detail": Equal("must provide an architecture"),
						})),
					))
				})

				It("should successfully validate with no architectures changes", func() {
					cloudProfileNew.Spec.MachineImages[0].Versions[0].Architectures = []string{"arm64"}
					cloudProfileNew.Spec.MachineTypes[0].Architecture = ptr.To("arm64")

					Expect(ValidateCloudProfileUpdate(cloudProfileNew, cloudProfileOld)).To(BeEmpty())
				})

				It("should fail to validate with only capabilities set", func() {
					cloudProfileNew.Spec.MachineImages[0].Versions[0].CapabilitySets = []core.CapabilitySet{
						{Capabilities: core.Capabilities{"architecture": []string{"arm64"}}},
					}
					cloudProfileNew.Spec.MachineTypes[0].Capabilities = core.Capabilities{
						"architecture": []string{"arm64"},
					}

					Expect(ValidateCloudProfileUpdate(cloudProfileNew, cloudProfileOld)).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":   Equal(field.ErrorTypeRequired),
							"Field":  Equal("spec.machineImages[0].versions[0].architectures"),
							"Detail": Equal("must provide at least one architecture"),
						})), PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":   Equal(field.ErrorTypeForbidden),
							"Field":  Equal("spec.machineImages[0].versions[0].capabilitySets"),
							"Detail": Equal("must not provide capabilities without global definition"),
						})), PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":   Equal(field.ErrorTypeRequired),
							"Field":  Equal("spec.machineTypes[0].architecture"),
							"Detail": Equal("must provide an architecture"),
						})), PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":   Equal(field.ErrorTypeForbidden),
							"Field":  Equal("spec.machineTypes[0].capabilities"),
							"Detail": Equal("must not provide capabilities without global definition"),
						})),
					))
				})

				It("should fail to validate with unknown architectures set", func() {
					cloudProfileNew.Spec.MachineImages[0].Versions[0].Architectures = []string{"amd64", "other"}
					cloudProfileNew.Spec.MachineTypes[0].Architecture = ptr.To("other")

					Expect(ValidateCloudProfileUpdate(cloudProfileNew, cloudProfileOld)).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":     Equal(field.ErrorTypeNotSupported),
							"Field":    Equal("spec.machineImages[0].versions[0].architectures[1]"),
							"BadValue": Equal("other"),
							"Detail":   Equal(`supported values: "amd64", "arm64"`),
						})), PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":     Equal(field.ErrorTypeNotSupported),
							"Field":    Equal("spec.machineTypes[0].architecture"),
							"BadValue": Equal("other"),
							"Detail":   Equal(`supported values: "amd64", "arm64"`),
						})),
					))
				})
			})
		})
	})
})
