// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package namespacedcloudprofile_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener/pkg/apis/core"
	namespacedcloudprofileregistry "github.com/gardener/gardener/pkg/apiserver/registry/core/namespacedcloudprofile"
)

var _ = Describe("NamespacedCloudProfile Strategy", func() {
	var (
		ctx context.Context

		namespacedCloudProfile *core.NamespacedCloudProfile

		validExpirationDate1   *metav1.Time
		validExpirationDate2   *metav1.Time
		expiredExpirationDate1 *metav1.Time
		expiredExpirationDate2 *metav1.Time

		kubernetesSettings *core.KubernetesSettings
		machineImages      []core.MachineImage
	)

	BeforeEach(func() {
		ctx = context.Background()

		namespacedCloudProfile = &core.NamespacedCloudProfile{}

		validExpirationDate1 = &metav1.Time{Time: time.Now().Add(144 * time.Hour)}
		validExpirationDate2 = &metav1.Time{Time: time.Now().Add(24 * time.Hour)}
		expiredExpirationDate1 = &metav1.Time{Time: time.Now().Add(-time.Hour)}
		expiredExpirationDate2 = &metav1.Time{Time: time.Now().Add(-24 * time.Hour)}

		kubernetesSettings = &core.KubernetesSettings{
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
		}

		machineImages = []core.MachineImage{
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
		}
	})

	Describe("#PrepareForCreate", func() {
		DescribeTable("should drop expired versions from the NamespacedCloudProfile",
			func(useKubernetesSettings, useMachineImages bool) {
				namespacedCloudProfile = &core.NamespacedCloudProfile{}

				if useKubernetesSettings {
					namespacedCloudProfile.Spec.Kubernetes = kubernetesSettings
				}
				if useMachineImages {
					namespacedCloudProfile.Spec.MachineImages = machineImages
				}

				namespacedcloudprofileregistry.Strategy.PrepareForCreate(ctx, namespacedCloudProfile)

				if useKubernetesSettings {
					Expect(namespacedCloudProfile.Spec.Kubernetes.Versions).To(ConsistOf(
						MatchFields(IgnoreExtras, Fields{
							"Version": Equal("1.27.3"),
						}), MatchFields(IgnoreExtras, Fields{
							"Version": Equal("1.26.4"),
						}), MatchFields(IgnoreExtras, Fields{
							"Version": Equal("1.25.6"),
						}),
					))
				}
				if useMachineImages {
					Expect(namespacedCloudProfile.Spec.MachineImages).To(ConsistOf(
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
				}
			},

			Entry("only machineImage set", false, true),
			Entry("only kubernetes set", true, false),
			Entry("both kubernetes and machineImages set", true, true),
		)

		It("should drop empty machine image entries from NamespacedCloudProfile after dropping expired versions", func() {
			namespacedCloudProfile.Spec.MachineImages = []core.MachineImage{
				{Name: "machineImage1", Versions: []core.MachineImageVersion{
					{ExpirableVersion: core.ExpirableVersion{Version: "1.0.0", ExpirationDate: expiredExpirationDate1}},
				}},
				{Name: "machineImage2", Versions: []core.MachineImageVersion{
					{ExpirableVersion: core.ExpirableVersion{Version: "1.2.0"}},
				}},
			}

			namespacedcloudprofileregistry.Strategy.PrepareForCreate(ctx, namespacedCloudProfile)

			Expect(namespacedCloudProfile.Spec.MachineImages).To(Equal([]core.MachineImage{
				{Name: "machineImage2", Versions: []core.MachineImageVersion{
					{ExpirableVersion: core.ExpirableVersion{Version: "1.2.0"}},
				}},
			}))
		})

		It("should not drop a machine image entry from NamespacedCloudProfile if the updateStrategy is set", func() {
			namespacedCloudProfile.Spec.MachineImages = []core.MachineImage{
				{
					Name:           "machineImage1",
					UpdateStrategy: ptr.To(core.UpdateStrategyMajor),
					Versions: []core.MachineImageVersion{
						{ExpirableVersion: core.ExpirableVersion{Version: "1.0.0", ExpirationDate: expiredExpirationDate1}},
					},
				},
				{
					Name:           "machineImage2",
					UpdateStrategy: ptr.To(core.UpdateStrategyMajor),
				},
			}

			namespacedcloudprofileregistry.Strategy.PrepareForCreate(ctx, namespacedCloudProfile)

			Expect(namespacedCloudProfile.Spec.MachineImages).To(Equal([]core.MachineImage{
				{Name: "machineImage1", UpdateStrategy: ptr.To(core.UpdateStrategyMajor)},
				{Name: "machineImage2", UpdateStrategy: ptr.To(core.UpdateStrategyMajor)},
			}))
		})

		Describe("generation increment", func() {
			It("should set generation to 1 initially", func() {
				namespacedcloudprofileregistry.Strategy.PrepareForCreate(ctx, namespacedCloudProfile)

				Expect(namespacedCloudProfile.Generation).To(Equal(int64(1)))
			})
		})
	})

	Describe("#PrepareForUpdate", func() {
		var (
			oldNamespacedCloudProfile *core.NamespacedCloudProfile
		)

		BeforeEach(func() {
			oldNamespacedCloudProfile = &core.NamespacedCloudProfile{}
		})

		It("should drop expired Kubernetes versions not already present in the NamespacedCloudProfile", func() {
			namespacedCloudProfile.Spec.Kubernetes = kubernetesSettings

			namespacedcloudprofileregistry.Strategy.PrepareForUpdate(ctx, namespacedCloudProfile, oldNamespacedCloudProfile)

			Expect(namespacedCloudProfile.Spec.Kubernetes.Versions).To(ConsistOf(
				MatchFields(IgnoreExtras, Fields{
					"Version": Equal("1.27.3"),
				}), MatchFields(IgnoreExtras, Fields{
					"Version": Equal("1.26.4"),
				}), MatchFields(IgnoreExtras, Fields{
					"Version": Equal("1.25.6"),
				}),
			))
		})

		It("should not drop expired Kubernetes versions already present in the NamespacedCloudProfile", func() {
			oldNamespacedCloudProfile.Spec.Kubernetes = kubernetesSettings
			namespacedCloudProfile.Spec.Kubernetes = kubernetesSettings

			namespacedcloudprofileregistry.Strategy.PrepareForUpdate(ctx, namespacedCloudProfile, oldNamespacedCloudProfile)

			Expect(namespacedCloudProfile.Spec.Kubernetes.Versions).To(ConsistOf(
				MatchFields(IgnoreExtras, Fields{
					"Version": Equal("1.27.3"),
				}), MatchFields(IgnoreExtras, Fields{
					"Version": Equal("1.26.4"),
				}), MatchFields(IgnoreExtras, Fields{
					"Version": Equal("1.25.6"),
				}), MatchFields(IgnoreExtras, Fields{
					"Version": Equal("1.24.8"),
				}), MatchFields(IgnoreExtras, Fields{
					"Version": Equal("1.24.6"),
				}),
			))
		})

		It("should drop expired MachineImage versions not already present in the NamespacedCloudProfile", func() {
			namespacedCloudProfile.Spec.MachineImages = machineImages

			namespacedcloudprofileregistry.Strategy.PrepareForUpdate(ctx, namespacedCloudProfile, oldNamespacedCloudProfile)

			Expect(namespacedCloudProfile.Spec.MachineImages).To(ConsistOf(
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

		It("should not drop expired MachineImage versions already present in the NamespacedCloudProfile", func() {
			oldNamespacedCloudProfile.Spec.MachineImages = machineImages
			namespacedCloudProfile.Spec.MachineImages = machineImages

			namespacedcloudprofileregistry.Strategy.PrepareForUpdate(ctx, namespacedCloudProfile, oldNamespacedCloudProfile)

			Expect(namespacedCloudProfile.Spec.MachineImages).To(ConsistOf(
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
					), MatchFields(IgnoreExtras, Fields{
						"ExpirableVersion": MatchFields(IgnoreExtras, Fields{
							"Version": Equal("1.9.7"),
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
					), MatchFields(IgnoreExtras, Fields{
						"ExpirableVersion": MatchFields(IgnoreExtras, Fields{
							"Version": Equal("4.1.8"),
						})},
					)),
				}),
			))
		})

		Describe("generation increment", func() {
			It("should not increment generation if spec has not changed", func() {
				namespacedcloudprofileregistry.Strategy.PrepareForUpdate(ctx, namespacedCloudProfile, oldNamespacedCloudProfile)

				Expect(namespacedCloudProfile.Generation).To(Equal(oldNamespacedCloudProfile.Generation))
			})

			It("should increment generation if spec has changed", func() {
				namespacedCloudProfile.Spec.Parent = core.CloudProfileReference{
					Kind: "abc",
					Name: "def",
				}

				namespacedcloudprofileregistry.Strategy.PrepareForUpdate(ctx, namespacedCloudProfile, oldNamespacedCloudProfile)

				Expect(namespacedCloudProfile.Generation).To(Equal(oldNamespacedCloudProfile.Generation + 1))
			})

			It("should increment generation if deletion timestamp is set", func() {
				namespacedCloudProfile.DeletionTimestamp = &metav1.Time{Time: time.Now()}

				namespacedcloudprofileregistry.Strategy.PrepareForUpdate(ctx, namespacedCloudProfile, oldNamespacedCloudProfile)

				Expect(namespacedCloudProfile.Generation).To(Equal(oldNamespacedCloudProfile.Generation + 1))
			})
		})

		It("should prevent manual updates of the status field", func() {
			namespacedCloudProfile.Status = core.NamespacedCloudProfileStatus{
				CloudProfileSpec: core.CloudProfileSpec{
					Kubernetes: core.KubernetesSettings{Versions: []core.ExpirableVersion{
						{Version: "1.27.3"},
					}},
				},
			}

			namespacedcloudprofileregistry.Strategy.PrepareForUpdate(ctx, namespacedCloudProfile, oldNamespacedCloudProfile)

			Expect(namespacedCloudProfile.Status).To(Equal(oldNamespacedCloudProfile.Status))
		})
	})
})
