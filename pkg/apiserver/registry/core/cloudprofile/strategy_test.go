// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
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
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener/pkg/apis/core"
	"github.com/gardener/gardener/pkg/apiserver/registry/core/cloudprofile"
)

var _ = Describe("Strategy", func() {
	var (
		ctx      = context.Background()
		strategy rest.RESTCreateUpdateStrategy
	)

	BeforeEach(func() {
		strategy = cloudprofile.Strategy
	})

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
								Version: "1.25.6",
								Lifecycle: []core.LifecycleStage{
									{
										Classification: core.ClassificationSupported,
									},
									{
										Classification: core.ClassificationExpired,
										StartTime:      validExpirationDate2,
									},
								},
							},
							{
								Version:        "1.24.8",
								ExpirationDate: expiredExpirationDate1,
							},
							{
								Version: "1.24.6",
								Lifecycle: []core.LifecycleStage{
									{
										Classification: core.ClassificationSupported,
									},
									{
										Classification: core.ClassificationExpired,
										StartTime:      expiredExpirationDate2,
									},
								},
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
						{
							Name: "machineImage3",
							Versions: []core.MachineImageVersion{
								{
									ExpirableVersion: core.ExpirableVersion{
										Version: "1.3.0",
										Lifecycle: []core.LifecycleStage{
											{
												Classification: core.ClassificationSupported,
											},
											{
												Classification: core.ClassificationExpired,
												StartTime:      validExpirationDate1,
											},
										},
									},
								},
								{
									ExpirableVersion: core.ExpirableVersion{
										Version: "1.2.3",
										Lifecycle: []core.LifecycleStage{
											{
												Classification: core.ClassificationSupported,
											},
										},
									},
								},
								{
									ExpirableVersion: core.ExpirableVersion{
										Version: "1.1.8",
										Lifecycle: []core.LifecycleStage{
											{
												Classification: core.ClassificationSupported,
											},
											{
												Classification: core.ClassificationExpired,
												StartTime:      expiredExpirationDate1,
											},
										},
									},
								},
							},
						},
					},
				},
			}

			strategy.PrepareForCreate(ctx, cloudProfile)

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
				}), MatchFields(IgnoreExtras, Fields{
					"Name": Equal("machineImage3"),
					"Versions": ConsistOf(MatchFields(IgnoreExtras, Fields{
						"ExpirableVersion": MatchFields(IgnoreExtras, Fields{
							"Version": Equal("1.3.0"),
						})},
					), MatchFields(IgnoreExtras, Fields{
						"ExpirableVersion": MatchFields(IgnoreExtras, Fields{
							"Version": Equal("1.2.3"),
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
			oldCloudProfile = &core.CloudProfile{}
			newCloudProfile = &core.CloudProfile{}
		})

		It("should not allow editing the status", func() {
			k8sStatus := core.KubernetesStatus{
				Versions: []core.ExpirableVersionStatus{
					{
						Version: "foo",
					},
				},
			}
			newCloudProfile.Status.Kubernetes = &k8sStatus

			strategy.PrepareForUpdate(ctx, newCloudProfile, oldCloudProfile)

			Expect(newCloudProfile.Status).To(Equal(oldCloudProfile.Status))
		})

		It("should correctly sync the architecture fields on migration to Capabilities", func() {
			oldCloudProfile = &core.CloudProfile{
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
			newCloudProfile.Spec.MachineCapabilities = []core.CapabilityDefinition{
				{Name: "architecture", Values: []string{"amd64", "arm64"}},
			}

			strategy.PrepareForUpdate(ctx, newCloudProfile, oldCloudProfile)

			Expect(newCloudProfile.Spec.MachineTypes[0].Architecture).To(Equal(ptr.To("amd64")))
			Expect(newCloudProfile.Spec.MachineTypes[0].Capabilities["architecture"]).To(ConsistOf("amd64"))

			Expect(newCloudProfile.Spec.MachineImages[0].Versions[0].Architectures).To(ConsistOf("amd64"))
			Expect(newCloudProfile.Spec.MachineImages[0].Versions[0].CapabilityFlavors).To(ConsistOf(core.MachineImageFlavor{
				Capabilities: core.Capabilities{"architecture": []string{"amd64"}},
			}))
		})
	})

	Describe("#Canonicalize", func() {
		It("should sync architecture capabilities to empty architecture fields", func() {
			cloudProfile := &core.CloudProfile{
				Spec: core.CloudProfileSpec{
					MachineCapabilities: []core.CapabilityDefinition{
						{Name: "architecture", Values: []string{"amd64"}},
					},
					MachineImages: []core.MachineImage{{Versions: []core.MachineImageVersion{
						{CapabilityFlavors: []core.MachineImageFlavor{{Capabilities: core.Capabilities{
							"architecture": []string{"amd64"}}}}},
					}}},
					MachineTypes: []core.MachineType{{Capabilities: core.Capabilities{
						"architecture": []string{"amd64"},
					}}},
				},
			}

			strategy.Canonicalize(cloudProfile)

			Expect(cloudProfile.Spec.MachineTypes[0].Architecture).To(PointTo(Equal("amd64")))
			Expect(cloudProfile.Spec.MachineImages[0].Versions[0].Architectures).To(ConsistOf("amd64"))
		})
	})

	Describe("StatusStrategy", func() {
		BeforeEach(func() {
			strategy = cloudprofile.StatusStrategy
		})

		var (
			newCloudProfile *core.CloudProfile
			oldCloudProfile *core.CloudProfile
		)

		BeforeEach(func() {
			oldCloudProfile = &core.CloudProfile{}
			newCloudProfile = &core.CloudProfile{}
		})

		It("should allow updating the status", func() {
			newCloudProfile.Status.Kubernetes = &core.KubernetesStatus{
				Versions: []core.ExpirableVersionStatus{{Version: "foo"}},
			}
			strategy.PrepareForUpdate(ctx, newCloudProfile, oldCloudProfile)

			Expect(newCloudProfile.Status.Kubernetes.Versions).To(ConsistOf(core.ExpirableVersionStatus{Version: "foo"}))
		})

		It("should not allow editing the spec", func() {
			newCloudProfile.Spec.Type = "foo"
			strategy.PrepareForUpdate(ctx, newCloudProfile, oldCloudProfile)

			Expect(newCloudProfile.Spec).To(Equal(oldCloudProfile.Spec))
		})
	})
})
