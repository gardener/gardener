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

	"github.com/gardener/gardener/pkg/apis/core"
	namespacedcloudprofileregistry "github.com/gardener/gardener/pkg/apiserver/registry/core/namespacedcloudprofile"
)

var _ = Describe("PrepareForCreate", func() {
	var (
		validExpirationDate1   = &metav1.Time{Time: time.Now().Add(144 * time.Hour)}
		validExpirationDate2   = &metav1.Time{Time: time.Now().Add(24 * time.Hour)}
		expiredExpirationDate1 = &metav1.Time{Time: time.Now().Add(-time.Hour)}
		expiredExpirationDate2 = &metav1.Time{Time: time.Now().Add(-24 * time.Hour)}

		namespacedCloudProfile *core.NamespacedCloudProfile

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
	)

	DescribeTable("should drop expired versions from the NamespacedCloudProfile",
		func(useKubernetesSettings, useMachineImages bool) {
			namespacedCloudProfile = &core.NamespacedCloudProfile{}

			if useKubernetesSettings {
				namespacedCloudProfile.Spec.Kubernetes = kubernetesSettings
			}
			if useMachineImages {
				namespacedCloudProfile.Spec.MachineImages = machineImages
			}

			namespacedcloudprofileregistry.Strategy.PrepareForCreate(context.TODO(), namespacedCloudProfile)

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
		namespacedCloudProfile = &core.NamespacedCloudProfile{}
		namespacedCloudProfile.Spec.MachineImages = []core.MachineImage{
			{Name: "machineImage1", Versions: []core.MachineImageVersion{
				{ExpirableVersion: core.ExpirableVersion{Version: "1.0.0", ExpirationDate: expiredExpirationDate1}},
			}},
			{Name: "machineImage2", Versions: []core.MachineImageVersion{
				{ExpirableVersion: core.ExpirableVersion{Version: "1.2.0"}},
			}},
		}

		namespacedcloudprofileregistry.Strategy.PrepareForCreate(context.TODO(), namespacedCloudProfile)

		Expect(namespacedCloudProfile.Spec.MachineImages).To(Equal([]core.MachineImage{
			{Name: "machineImage2", Versions: []core.MachineImageVersion{
				{ExpirableVersion: core.ExpirableVersion{Version: "1.2.0"}},
			}},
		}))
	})

	Describe("generation increment", func() {
		var (
			ctx context.Context

			oldNamespacedCloudProfile *core.NamespacedCloudProfile
			newNamespacedCloudProfile *core.NamespacedCloudProfile
		)

		BeforeEach(func() {
			ctx = context.TODO()

			oldNamespacedCloudProfile = &core.NamespacedCloudProfile{}
			newNamespacedCloudProfile = &core.NamespacedCloudProfile{}
		})

		It("should set generation to 1 initially", func() {
			namespacedcloudprofileregistry.Strategy.PrepareForCreate(ctx, newNamespacedCloudProfile)

			Expect(newNamespacedCloudProfile.Generation).To(Equal(int64(1)))
		})

		It("should not increment generation if spec has not changed", func() {
			namespacedcloudprofileregistry.Strategy.PrepareForUpdate(ctx, newNamespacedCloudProfile, oldNamespacedCloudProfile)

			Expect(newNamespacedCloudProfile.Generation).To(Equal(oldNamespacedCloudProfile.Generation))
		})

		It("should increment generation if spec has changed", func() {
			newNamespacedCloudProfile.Spec.Parent = core.CloudProfileReference{
				Kind: "abc",
				Name: "def",
			}

			namespacedcloudprofileregistry.Strategy.PrepareForUpdate(ctx, newNamespacedCloudProfile, oldNamespacedCloudProfile)

			Expect(newNamespacedCloudProfile.Generation).To(Equal(oldNamespacedCloudProfile.Generation + 1))
		})

		It("should increment generation if deletion timestamp is set", func() {
			newNamespacedCloudProfile.DeletionTimestamp = &metav1.Time{Time: time.Now()}

			namespacedcloudprofileregistry.Strategy.PrepareForUpdate(ctx, newNamespacedCloudProfile, oldNamespacedCloudProfile)

			Expect(newNamespacedCloudProfile.Generation).To(Equal(oldNamespacedCloudProfile.Generation + 1))
		})
	})
})
