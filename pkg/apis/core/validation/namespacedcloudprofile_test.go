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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener/pkg/apis/core"
	. "github.com/gardener/gardener/pkg/apis/core/validation"
)

var _ = Describe("NamespacedCloudProfile Validation Tests ", func() {
	var machineImageName = "some-machine-image"
	Describe("#ValidateNamespacedCloudProfile", func() {
		It("should forbid empty NamespacedCloudProfile resources", func() {
			namespacedCloudProfile := &core.NamespacedCloudProfile{
				ObjectMeta: metav1.ObjectMeta{},
				Spec:       core.NamespacedCloudProfileSpec{},
			}

			errorList := ValidateNamespacedCloudProfile(namespacedCloudProfile)

			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("metadata.name"),
				})), PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("metadata.namespace"),
				})), PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeNotSupported),
					"Field": Equal("spec.parent.kind"),
				})), PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.parent.name"),
				}))))
		})

		It("should allow NamespacedCloudProfile resource with only name and parent field", func() {
			namespacedCloudProfile := &core.NamespacedCloudProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "profile",
					Namespace: "default",
				},
				Spec: core.NamespacedCloudProfileSpec{
					Parent: core.CloudProfileReference{
						Kind: "CloudProfile",
						Name: "profile-parent",
					},
				},
			}

			errorList := ValidateNamespacedCloudProfile(namespacedCloudProfile)
			Expect(errorList).To(BeEmpty())
		})

		It("should not allow a NamespacedCloudProfile to change its parent reference", func() {
			namespacedCloudProfile := &core.NamespacedCloudProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "profile",
					Namespace: "default",
				},
				Spec: core.NamespacedCloudProfileSpec{
					Parent: core.CloudProfileReference{
						Kind: "CloudProfile",
						Name: "profile-parent",
					},
				},
			}

			newNamespacedCloudProfile := namespacedCloudProfile.DeepCopy()
			newNamespacedCloudProfile.Spec.Parent.Name = "other-profile-parent"
			newNamespacedCloudProfile.ResourceVersion = "2"

			errorList := ValidateNamespacedCloudProfileUpdate(newNamespacedCloudProfile, namespacedCloudProfile)
			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.parent"),
				}))))
		})

		Context("tests for invalid NamespacedCloudProfile spec", func() {
			var (
				namespacedCloudProfile *core.NamespacedCloudProfile

				duplicatedKubernetes = core.KubernetesSettings{
					Versions: []core.ExpirableVersion{{Version: "1.11.4"}, {Version: "1.11.4", ExpirationDate: &metav1.Time{Time: time.Now().Add(24 * time.Hour)}}},
				}
			)

			BeforeEach(func() {
				namespacedCloudProfile = &core.NamespacedCloudProfile{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "profile",
						Namespace: "default",
					},
					Spec: core.NamespacedCloudProfileSpec{
						Parent: core.CloudProfileReference{
							Kind: "CloudProfile",
							Name: "unknown",
						},
						Kubernetes: &core.KubernetesSettings{
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
									},
								},
							},
						},
						MachineTypes: machineTypesConstraint,
						VolumeTypes:  volumeTypesConstraint,
					},
				}
			})

			It("should not return any errors", func() {
				errorList := ValidateNamespacedCloudProfile(namespacedCloudProfile)

				Expect(errorList).To(BeEmpty())
			})

			DescribeTable("namespacedCloudProfile metadata",
				func(objectMeta metav1.ObjectMeta, matcher gomegatypes.GomegaMatcher) {
					namespacedCloudProfile.ObjectMeta = objectMeta
					namespacedCloudProfile.ObjectMeta.Namespace = "default"

					errorList := ValidateNamespacedCloudProfile(namespacedCloudProfile)

					Expect(errorList).To(matcher)
				},

				Entry("should forbid namespacedCloudProfile with empty metadata",
					metav1.ObjectMeta{},
					ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal("metadata.name"),
					}))),
				),
				Entry("should forbid namespacedCloudProfile with empty name",
					metav1.ObjectMeta{Name: ""},
					ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal("metadata.name"),
					}))),
				),
				Entry("should allow namespacedCloudProfile with '.' in the name",
					metav1.ObjectMeta{Name: "profile.test"},
					BeEmpty(),
				),
				Entry("should forbid namespacedCloudProfile with '_' in the name (not a DNS-1123 subdomain)",
					metav1.ObjectMeta{Name: "profile_test"},
					ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("metadata.name"),
					}))),
				),
			)

			It("should forbid not specifying a parent", func() {
				namespacedCloudProfile.Spec.Parent = core.CloudProfileReference{Kind: "", Name: ""}

				errorList := ValidateNamespacedCloudProfile(namespacedCloudProfile)

				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeRequired),
						"Field":  Equal("spec.parent.name"),
						"Detail": Equal("must provide a parent name"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeNotSupported),
						"Field":  Equal("spec.parent.kind"),
						"Detail": Equal("supported values: \"CloudProfile\""),
					}))))
			})

			It("should forbid specifying an invalid parent kind", func() {
				namespacedCloudProfile.Spec.Parent = core.CloudProfileReference{Kind: "SomeOtherCloudPofile", Name: "my-profile"}

				errorList := ValidateNamespacedCloudProfile(namespacedCloudProfile)

				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeNotSupported),
						"Field":  Equal("spec.parent.kind"),
						"Detail": Equal("supported values: \"CloudProfile\""),
					}))))
			})

			It("should forbid ca bundles with unsupported format", func() {
				namespacedCloudProfile.Spec.CABundle = ptr.To("unsupported")

				errorList := ValidateNamespacedCloudProfile(namespacedCloudProfile)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.caBundle"),
				}))))
			})

			Context("kubernetes version constraints", func() {
				It("should forbid versions of a not allowed pattern", func() {
					namespacedCloudProfile.Spec.Kubernetes.Versions = []core.ExpirableVersion{{Version: "1.11"}}

					errorList := ValidateNamespacedCloudProfile(namespacedCloudProfile)

					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("spec.kubernetes.versions[0]"),
					}))))
				})

				It("should forbid duplicated kubernetes versions", func() {
					namespacedCloudProfile.Spec.Kubernetes = &duplicatedKubernetes

					errorList := ValidateNamespacedCloudProfile(namespacedCloudProfile)

					Expect(errorList).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeDuplicate),
							"Field": Equal(fmt.Sprintf("spec.kubernetes.versions[%d].version", len(duplicatedKubernetes.Versions)-1)),
						}))))
				})

				It("should forbid providing a classification", func() {
					classification := core.ClassificationSupported
					namespacedCloudProfile.Spec.Kubernetes.Versions = []core.ExpirableVersion{
						{
							Version:        "1.1.0",
							Classification: &classification,
							ExpirationDate: ptr.To(metav1.Time{Time: time.Now().Add(24 * time.Hour)}),
						},
					}

					errorList := ValidateNamespacedCloudProfile(namespacedCloudProfile)

					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeForbidden),
						"Field":  Equal("spec.kubernetes.versions[0].classification"),
						"Detail": Equal("must not provide a classification to a Kubernetes version in NamespacedCloudProfile"),
					}))))
				})
			})

			Context("machine image validation", func() {
				It("should forbid duplicate names in list of machine images", func() {
					namespacedCloudProfile.Spec.MachineImages = []core.MachineImage{
						{
							Name: machineImageName,
							Versions: []core.MachineImageVersion{
								{
									ExpirableVersion: core.ExpirableVersion{
										Version: "3.4.6",
									},
								},
							},
						},
						{
							Name: machineImageName,
							Versions: []core.MachineImageVersion{
								{
									ExpirableVersion: core.ExpirableVersion{
										Version: "3.4.5",
									},
								},
							},
						},
					}

					errorList := ValidateNamespacedCloudProfile(namespacedCloudProfile)

					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeDuplicate),
						"Field": Equal("spec.machineImages[1]"),
					}))))
				})

				It("should allow machine images that only override the update strategy", func() {
					namespacedCloudProfile.Spec.MachineImages = []core.MachineImage{
						{Name: machineImageName, UpdateStrategy: ptr.To(core.UpdateStrategyMajor)},
					}

					Expect(ValidateNamespacedCloudProfile(namespacedCloudProfile)).To(BeEmpty())
				})

				It("should allow providing new machine image versions with all fields filled out", func() {
					validExpirationDate := &metav1.Time{Time: time.Now().AddDate(0, 1, 0)}
					namespacedCloudProfile.Spec.MachineImages = []core.MachineImage{
						{
							Name:           machineImageName,
							UpdateStrategy: ptr.To(core.UpdateStrategyPatch),
							Versions: []core.MachineImageVersion{
								{
									ExpirableVersion: core.ExpirableVersion{
										Version:        "3.4.6",
										ExpirationDate: validExpirationDate,
										Classification: ptr.To(core.ClassificationDeprecated),
									},
									CRI:                      []core.CRI{{Name: "containerd"}},
									Architectures:            []string{"amd64"},
									KubeletVersionConstraint: ptr.To(">= 1.30.0"),
								},
							},
						},
					}

					Expect(ValidateNamespacedCloudProfile(namespacedCloudProfile)).To(BeEmpty())
				})

				It("should allow empty additional values", func() {
					validExpirationDate := &metav1.Time{Time: time.Now().AddDate(0, 1, 0)}
					namespacedCloudProfile.Spec.MachineImages = []core.MachineImage{
						{
							Name: machineImageName,
							Versions: []core.MachineImageVersion{
								{
									ExpirableVersion: core.ExpirableVersion{
										Version:        "3.4.6",
										ExpirationDate: validExpirationDate,
									},
									CRI:           []core.CRI{},
									Architectures: []string{},
								},
							},
						},
					}

					Expect(ValidateNamespacedCloudProfile(namespacedCloudProfile)).To(BeEmpty())
				})

				It("should forbid nonSemVer machine image versions", func() {
					namespacedCloudProfile.Spec.MachineImages = []core.MachineImage{
						{
							Name: machineImageName,
							Versions: []core.MachineImageVersion{
								{
									ExpirableVersion: core.ExpirableVersion{
										Version: "0.1.2",
									},
								},
							},
						},
						{
							Name: "xy",
							Versions: []core.MachineImageVersion{
								{
									ExpirableVersion: core.ExpirableVersion{
										Version: "a.b.c",
									},
								},
							},
						},
					}

					errorList := ValidateNamespacedCloudProfile(namespacedCloudProfile)

					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("spec.machineImages"),
					})), PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("spec.machineImages[1].versions[0].version"),
					}))))
				})
			})

			Context("machine types validation", func() {
				It("should enforce uniqueness of machine type names", func() {
					namespacedCloudProfile.Spec.MachineTypes = []core.MachineType{
						namespacedCloudProfile.Spec.MachineTypes[0],
						namespacedCloudProfile.Spec.MachineTypes[0],
					}

					errorList := ValidateNamespacedCloudProfile(namespacedCloudProfile)

					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeDuplicate),
						"Field": Equal("spec.machineTypes[1].name"),
					}))))
				})

				It("should forbid machine types with unsupported property values", func() {
					namespacedCloudProfile.Spec.MachineTypes = invalidMachineTypes

					errorList := ValidateNamespacedCloudProfile(namespacedCloudProfile)

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
					namespacedCloudProfile.Spec.MachineTypes = machineTypesConstraint

					errorList := ValidateNamespacedCloudProfile(namespacedCloudProfile)
					Expect(errorList).To(BeEmpty())
				})
			})

			Context("volume types validation", func() {
				It("should enforce uniqueness of volume type names", func() {
					namespacedCloudProfile.Spec.VolumeTypes = []core.VolumeType{
						namespacedCloudProfile.Spec.VolumeTypes[0],
						namespacedCloudProfile.Spec.VolumeTypes[0],
					}

					errorList := ValidateNamespacedCloudProfile(namespacedCloudProfile)

					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeDuplicate),
						"Field": Equal("spec.volumeTypes[1].name"),
					}))))
				})

				It("should forbid volume types with unsupported property values", func() {
					namespacedCloudProfile.Spec.VolumeTypes = invalidVolumeTypes

					errorList := ValidateNamespacedCloudProfile(namespacedCloudProfile)

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
		})

		Context("limits validation", func() {
			var (
				namespacedCloudProfile *core.NamespacedCloudProfile
			)

			BeforeEach(func() {
				namespacedCloudProfile = &core.NamespacedCloudProfile{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "profile",
						Namespace: "default",
					},
					Spec: core.NamespacedCloudProfileSpec{
						Parent: core.CloudProfileReference{
							Kind: "CloudProfile",
							Name: "profile-parent",
						},
					},
				}
			})

			It("should allow unset limits", func() {
				namespacedCloudProfile.Spec.Limits = nil

				Expect(ValidateNamespacedCloudProfile(namespacedCloudProfile)).To(BeEmpty())
			})

			It("should allow empty limits", func() {
				namespacedCloudProfile.Spec.Limits = &core.Limits{}

				Expect(ValidateNamespacedCloudProfile(namespacedCloudProfile)).To(BeEmpty())
			})

			It("should allow positive maxNodesTotal", func() {
				namespacedCloudProfile.Spec.Limits = &core.Limits{
					MaxNodesTotal: ptr.To[int32](100),
				}

				Expect(ValidateNamespacedCloudProfile(namespacedCloudProfile)).To(BeEmpty())
			})

			It("should forbid zero maxNodesTotal", func() {
				namespacedCloudProfile.Spec.Limits = &core.Limits{
					MaxNodesTotal: ptr.To[int32](0),
				}

				Expect(ValidateNamespacedCloudProfile(namespacedCloudProfile)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.limits.maxNodesTotal"),
				}))))
			})

			It("should forbid negative maxNodesTotal", func() {
				namespacedCloudProfile.Spec.Limits = &core.Limits{
					MaxNodesTotal: ptr.To[int32](-1),
				}

				Expect(ValidateNamespacedCloudProfile(namespacedCloudProfile)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.limits.maxNodesTotal"),
				}))))
			})
		})
	})

	var (
		cloudProfileNew *core.NamespacedCloudProfile
		cloudProfileOld *core.NamespacedCloudProfile
		dateInThePast   = &metav1.Time{Time: time.Now().AddDate(-5, 0, 0)}
	)

	Describe("#ValidateNamespacedCloudProfileUpdate", func() {
		BeforeEach(func() {
			cloudProfileNew = &core.NamespacedCloudProfile{
				ObjectMeta: metav1.ObjectMeta{
					ResourceVersion: "dummy",
					Name:            "dummy",
					Namespace:       "dummy",
				},
				Spec: core.NamespacedCloudProfileSpec{
					Parent: core.CloudProfileReference{
						Kind: "CloudProfile",
						Name: "aws-profile",
					},
					MachineImages: []core.MachineImage{
						{
							Name: machineImageName,
							Versions: []core.MachineImageVersion{
								{
									ExpirableVersion: core.ExpirableVersion{
										Version:        "1.2.3",
										ExpirationDate: dateInThePast,
									},
								},
							},
						},
					},
					Kubernetes: &core.KubernetesSettings{
						Versions: []core.ExpirableVersion{
							{
								Version:        "1.17.2",
								ExpirationDate: dateInThePast,
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
					{Version: "1.17.2"},
					{Version: "1.17.1", ExpirationDate: dateInThePast},
					{Version: "1.17.0", ExpirationDate: dateInThePast},
				}
				cloudProfileNew.Spec.Kubernetes.Versions = versions[0:1]
				cloudProfileOld.Spec.Kubernetes.Versions = versions
				errorList := ValidateNamespacedCloudProfileUpdate(cloudProfileNew, cloudProfileOld)

				Expect(errorList).To(BeEmpty())
			})
		})

		Context("Removed MachineImage versions", func() {
			It("deleting version - should not return any errors", func() {
				versions := []core.MachineImageVersion{
					{
						ExpirableVersion: core.ExpirableVersion{
							Version: "2135.6.2",
						},
					},
					{
						ExpirableVersion: core.ExpirableVersion{
							Version:        "2135.6.1",
							ExpirationDate: dateInThePast,
						},
					},
					{
						ExpirableVersion: core.ExpirableVersion{
							Version:        "2135.6.0",
							ExpirationDate: dateInThePast,
						},
					},
				}
				cloudProfileNew.Spec.MachineImages[0].Versions = versions[0:1]
				cloudProfileOld.Spec.MachineImages[0].Versions = versions
				errorList := ValidateNamespacedCloudProfileUpdate(cloudProfileNew, cloudProfileOld)

				Expect(errorList).To(BeEmpty())
			})
		})

		Context("limits validation", func() {
			BeforeEach(func() {
				cloudProfileNew = &core.NamespacedCloudProfile{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "profile",
						Namespace: "default",

						ResourceVersion: "1",
					},
					Spec: core.NamespacedCloudProfileSpec{
						Parent: core.CloudProfileReference{
							Kind: "CloudProfile",
							Name: "profile-parent",
						},
					},
				}
				cloudProfileOld = cloudProfileNew.DeepCopy()
			})

			It("should allow adding limits", func() {
				cloudProfileNew.Spec.Limits = &core.Limits{
					MaxNodesTotal: ptr.To[int32](100),
				}

				Expect(ValidateNamespacedCloudProfileUpdate(cloudProfileNew, cloudProfileOld)).To(BeEmpty())
			})

			It("should allow removing limits", func() {
				cloudProfileOld.Spec.Limits = &core.Limits{
					MaxNodesTotal: ptr.To[int32](100),
				}

				Expect(ValidateNamespacedCloudProfileUpdate(cloudProfileNew, cloudProfileOld)).To(BeEmpty())
			})

			It("should allow adding maxNodesTotal", func() {
				cloudProfileOld.Spec.Limits = &core.Limits{}
				cloudProfileNew.Spec.Limits = &core.Limits{
					MaxNodesTotal: ptr.To[int32](100),
				}

				Expect(ValidateNamespacedCloudProfileUpdate(cloudProfileNew, cloudProfileOld)).To(BeEmpty())
			})

			It("should allow removing maxNodesTotal", func() {
				cloudProfileOld.Spec.Limits = &core.Limits{
					MaxNodesTotal: ptr.To[int32](100),
				}
				cloudProfileNew.Spec.Limits = &core.Limits{}

				Expect(ValidateNamespacedCloudProfileUpdate(cloudProfileNew, cloudProfileOld)).To(BeEmpty())
			})

			It("should allow unchanged maxNodesTotal", func() {
				cloudProfileOld.Spec.Limits = &core.Limits{
					MaxNodesTotal: ptr.To[int32](100),
				}
				cloudProfileNew.Spec.Limits = &core.Limits{
					MaxNodesTotal: ptr.To[int32](100),
				}

				Expect(ValidateNamespacedCloudProfileUpdate(cloudProfileNew, cloudProfileOld)).To(BeEmpty())
			})

			It("should allow increasing maxNodesTotal", func() {
				cloudProfileOld.Spec.Limits = &core.Limits{
					MaxNodesTotal: ptr.To[int32](100),
				}
				cloudProfileNew.Spec.Limits = &core.Limits{
					MaxNodesTotal: ptr.To[int32](1000),
				}

				Expect(ValidateNamespacedCloudProfileUpdate(cloudProfileNew, cloudProfileOld)).To(BeEmpty())
			})

			It("should forbid decreasing maxNodesTotal", func() {
				cloudProfileOld.Spec.Limits = &core.Limits{
					MaxNodesTotal: ptr.To[int32](100),
				}
				cloudProfileNew.Spec.Limits = &core.Limits{
					MaxNodesTotal: ptr.To[int32](10),
				}

				Expect(ValidateNamespacedCloudProfileUpdate(cloudProfileNew, cloudProfileOld)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.limits.maxNodesTotal"),
				}))))
			})
		})
	})
})
