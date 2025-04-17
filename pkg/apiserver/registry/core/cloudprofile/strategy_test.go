// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package cloudprofile_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener/pkg/apis/core"
	cloudprofileregistry "github.com/gardener/gardener/pkg/apiserver/registry/core/cloudprofile"
)

var _ = Describe("Strategy", func() {
	Describe("#PrepareForCreate", func() {
		var cloudProfile *core.CloudProfile

		It("should drop the expired Kubernetes and MachineImage versions from the cloudprofile", func() {
			var (
				validExpirationDate1   = &metav1.Time{Time: time.Now().Add(144 * time.Hour)}
				validExpirationDate2   = &metav1.Time{Time: time.Now().Add(24 * time.Hour)}
				expiredExpirationDate1 = &metav1.Time{Time: time.Now().Add(-time.Hour)}
				expiredExpirationDate2 = &metav1.Time{Time: time.Now().Add(-24 * time.Hour)}
			)

			cloudProfile = &core.CloudProfile{
				Spec: core.CloudProfileSpec{
					Kubernetes: core.KubernetesSettings{
						Versions: []core.ExpirableVersion{
							{
								Version: "1.27.3",
							},
							{
								Version:        "1.26.4",
								ExpirationDate: validExpirationDate1,
							},
							{
								Version:        "1.25.6",
								ExpirationDate: validExpirationDate2,
							},
							{
								Version:        "1.24.8",
								ExpirationDate: expiredExpirationDate1,
							},
							{
								Version:        "1.24.6",
								ExpirationDate: expiredExpirationDate2,
							},
						},
					},
					MachineImages: []core.MachineImage{
						{
							Name: "machineImage1",
							Versions: []core.MachineImageVersion{
								{
									ExpirableVersion: core.ExpirableVersion{
										Version: "2.1.0",
									},
								},
								{
									ExpirableVersion: core.ExpirableVersion{
										Version:        "2.0.3",
										ExpirationDate: validExpirationDate1,
									},
								},
								{
									ExpirableVersion: core.ExpirableVersion{
										Version:        "1.9.7",
										ExpirationDate: expiredExpirationDate2,
									},
								},
							},
						},
						{
							Name: "machineImage2",
							Versions: []core.MachineImageVersion{
								{
									ExpirableVersion: core.ExpirableVersion{
										Version:        "4.3.0",
										ExpirationDate: validExpirationDate2,
									},
								},
								{
									ExpirableVersion: core.ExpirableVersion{
										Version: "4.2.3",
									},
								},
								{
									ExpirableVersion: core.ExpirableVersion{
										Version:        "4.1.8",
										ExpirationDate: expiredExpirationDate1,
									},
								},
							},
						},
					},
				},
			}

			cloudprofileregistry.Strategy.PrepareForCreate(context.TODO(), cloudProfile)

			Expect(cloudProfile.Spec.Kubernetes.Versions).To(ConsistOf(
				MatchFields(IgnoreExtras, Fields{
					"Version": Equal("1.27.3"),
				}), MatchFields(IgnoreExtras, Fields{
					"Version": Equal("1.26.4"),
				}), MatchFields(IgnoreExtras, Fields{
					"Version": Equal("1.25.6"),
				}),
			))

			Expect(cloudProfile.Spec.MachineImages).To(ConsistOf(
				MatchFields(IgnoreExtras, Fields{
					"Name": Equal("machineImage1"),
					"Versions": ConsistOf(MatchFields(IgnoreExtras, Fields{
						"ExpirableVersion": MatchFields(IgnoreExtras, Fields{
							"Version": Equal("2.1.0"),
						})},
					), MatchFields(IgnoreExtras, Fields{
						"ExpirableVersion": MatchFields(IgnoreExtras, Fields{
							"Version": Equal("2.0.3"),
						})},
					)),
				}), MatchFields(IgnoreExtras, Fields{
					"Name": Equal("machineImage2"),
					"Versions": ConsistOf(MatchFields(IgnoreExtras, Fields{
						"ExpirableVersion": MatchFields(IgnoreExtras, Fields{
							"Version": Equal("4.3.0"),
						})},
					), MatchFields(IgnoreExtras, Fields{
						"ExpirableVersion": MatchFields(IgnoreExtras, Fields{
							"Version": Equal("4.2.3"),
						})},
					)),
				}),
			))
		})
	})

	Describe("#PrepareForUpdate", func() {
		var (
			newCloudProfile *core.CloudProfile
			oldCloudProfile *core.CloudProfile
		)

		BeforeEach(func() {
			newCloudProfile = &core.CloudProfile{
				Spec: core.CloudProfileSpec{
					Regions: []core.Region{{
						Name: "local",
					}},
				},
			}
			oldCloudProfile = newCloudProfile.DeepCopy()
		})

		It("should remove the access restriction when the label is dropped", func() {
			oldCloudProfile.Spec.Regions[0].Labels = map[string]string{"seed.gardener.cloud/eu-access": "true"}
			oldCloudProfile.Spec.Regions[0].AccessRestrictions = []core.AccessRestriction{{Name: "eu-access-only"}}
			newCloudProfile.Spec.Regions[0].AccessRestrictions = []core.AccessRestriction{{Name: "eu-access-only"}}

			cloudprofileregistry.Strategy.PrepareForUpdate(context.Background(), newCloudProfile, oldCloudProfile)

			Expect(newCloudProfile.Spec.Regions[0].AccessRestrictions).To(BeEmpty())
			Expect(newCloudProfile.Spec.Regions[0].Labels).To(BeEmpty())
		})

		It("should not remove the label when the access restriction is dropped", func() {
			oldCloudProfile.Spec.Regions[0].Labels = map[string]string{"seed.gardener.cloud/eu-access": "true"}
			oldCloudProfile.Spec.Regions[0].AccessRestrictions = []core.AccessRestriction{{Name: "eu-access-only"}}
			newCloudProfile.Spec.Regions[0].Labels = map[string]string{"seed.gardener.cloud/eu-access": "true"}

			cloudprofileregistry.Strategy.PrepareForUpdate(context.Background(), newCloudProfile, oldCloudProfile)

			Expect(newCloudProfile.Spec.Regions[0].AccessRestrictions).To(BeEmpty())
			Expect(newCloudProfile.Spec.Regions[0].Labels).To(Equal(map[string]string{"seed.gardener.cloud/eu-access": "true"}))
		})

		It("should correctly sync the architecture fields on migration to Capabilities", func() {
			oldCloudProfile := &core.CloudProfile{
				Spec: core.CloudProfileSpec{
					MachineTypes: []core.MachineType{
						{
							Name:         "machineType1",
							Architecture: ptr.To("amd64"),
						},
					},
					MachineImages: []core.MachineImage{
						{
							Versions: []core.MachineImageVersion{
								{
									ExpirableVersion: core.ExpirableVersion{
										Version: "1.0.0",
									},
									Architectures: []string{"amd64"},
								},
							},
						},
					},
				},
			}
			newCloudProfile := oldCloudProfile.DeepCopy()
			newCloudProfile.Spec.Capabilities = []core.CapabilityDefinition{
				{Name: "architecture", Values: []string{"amd64"}},
			}

			cloudprofileregistry.Strategy.PrepareForUpdate(context.Background(), newCloudProfile, oldCloudProfile)

			Expect(newCloudProfile.Spec.MachineTypes[0].Architecture).To(Equal(ptr.To("amd64")))
			Expect(newCloudProfile.Spec.MachineTypes[0].Capabilities["architecture"]).To(ConsistOf("amd64"))

			Expect(newCloudProfile.Spec.MachineImages[0].Versions[0].Architectures).To(ConsistOf("amd64"))
			Expect(newCloudProfile.Spec.MachineImages[0].Versions[0].CapabilitySets).To(ConsistOf(core.CapabilitySet{
				Capabilities: core.Capabilities{"architecture": []string{"amd64"}},
			}))
		})
	})

	Describe("#Canonicalize", func() {
		It("should add the access restriction if the legacy label is present", func() {
			cloudProfile := &core.CloudProfile{
				Spec: core.CloudProfileSpec{
					Regions: []core.Region{{
						Name:   "local",
						Labels: map[string]string{"seed.gardener.cloud/eu-access": "true"},
					}},
				},
			}

			cloudprofileregistry.Strategy.Canonicalize(cloudProfile)

			Expect(cloudProfile.Spec.Regions[0].AccessRestrictions).To(HaveExactElements(core.AccessRestriction{Name: "eu-access-only"}))
		})

		It("should add the label if the access restriction is configured", func() {
			cloudProfile := &core.CloudProfile{
				Spec: core.CloudProfileSpec{
					Regions: []core.Region{{
						Name:               "local",
						Labels:             map[string]string{"seed.gardener.cloud/eu-access": "true"},
						AccessRestrictions: []core.AccessRestriction{{Name: "eu-access-only"}},
					}},
				},
			}

			cloudprofileregistry.Strategy.Canonicalize(cloudProfile)

			Expect(cloudProfile.Spec.Regions[0].Labels).To(HaveKeyWithValue("seed.gardener.cloud/eu-access", "true"))
		})

		It("should not add the access restriction or label", func() {
			cloudProfile := &core.CloudProfile{
				Spec: core.CloudProfileSpec{
					Regions: []core.Region{{
						Name: "local",
					}},
				},
			}

			cloudprofileregistry.Strategy.Canonicalize(cloudProfile)

			Expect(cloudProfile.Spec.Regions[0].AccessRestrictions).To(BeEmpty())
			Expect(cloudProfile.Spec.Regions[0].Labels).To(BeEmpty())
		})

		It("should sync architecture capabilities to empty architecture fields", func() {
			cloudProfile := &core.CloudProfile{
				Spec: core.CloudProfileSpec{
					Capabilities: []core.CapabilityDefinition{
						{Name: "architecture", Values: []string{"amd64"}},
					},
					MachineImages: []core.MachineImage{{Versions: []core.MachineImageVersion{
						{CapabilitySets: []core.CapabilitySet{{Capabilities: core.Capabilities{
							"architecture": []string{"amd64"}}}}},
					}}},
					MachineTypes: []core.MachineType{{Capabilities: core.Capabilities{
						"architecture": []string{"amd64"},
					}}},
				},
			}

			cloudprofileregistry.Strategy.Canonicalize(cloudProfile)

			Expect(cloudProfile.Spec.MachineTypes[0].Architecture).To(PointTo(Equal("amd64")))
			Expect(cloudProfile.Spec.MachineImages[0].Versions[0].Architectures).To(ConsistOf("amd64"))
		})
	})
})
