// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package managedseedset_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	. "github.com/gardener/gardener/pkg/api/seedmanagement/managedseedset"
	"github.com/gardener/gardener/pkg/apis/core"
	"github.com/gardener/gardener/pkg/apis/seedmanagement"
)

var _ = Describe("Warnings", func() {
	Describe("#GetWarnings", func() {
		var managedSeedSet *seedmanagement.ManagedSeedSet

		BeforeEach(func() {
			managedSeedSet = &seedmanagement.ManagedSeedSet{}
		})

		DescribeTable("spec.shootTemplate.spec.kubernetes.kubeAPIServer",
			func(kubeAPIServerConfig *core.KubeAPIServerConfig, matcher gomegatypes.GomegaMatcher) {
				managedSeedSet.Spec.ShootTemplate.Spec.Kubernetes.KubeAPIServer = kubeAPIServerConfig
				Expect(GetWarnings(managedSeedSet)).To(matcher)
			},

			Entry("should not return a warning when kubeAPIServerConfig is nil",
				nil,
				BeEmpty(),
			),
			Entry("should not return a warning when enableAnonymousAuthentication is nil",
				&core.KubeAPIServerConfig{EnableAnonymousAuthentication: nil},
				BeEmpty(),
			),
			Entry("should return a warning when enableAnonymousAuthentication is set",
				&core.KubeAPIServerConfig{EnableAnonymousAuthentication: ptr.To(true)},
				ContainElement(Equal("you are setting the spec.shootTemplate.spec.kubernetes.kubeAPIServer.enableAnonymousAuthentication field. The field is deprecated. Using Kubernetes v1.32 and above, please use anonymous authentication configuration. See: https://kubernetes.io/docs/reference/access-authn-authz/authentication/#anonymous-authenticator-configuration")),
			),
		)

		DescribeTable("spec.shootTemplate.spec.kubernetes.kubeAPIServer.watchCacheSizes.default",
			func(kubeAPIServerConfig *core.KubeAPIServerConfig, matcher gomegatypes.GomegaMatcher) {
				managedSeedSet.Spec.ShootTemplate.Spec.Kubernetes.KubeAPIServer = kubeAPIServerConfig
				Expect(GetWarnings(managedSeedSet)).To(matcher)
			},

			Entry("should not return a warning when kubeAPIServerConfig is nil",
				nil,
				BeEmpty(),
			),
			Entry("should not return a warning when watchCacheSizes is nil",
				&core.KubeAPIServerConfig{WatchCacheSizes: nil},
				BeEmpty(),
			),
			Entry("should not return a warning when watchCacheSizes.default is nil",
				&core.KubeAPIServerConfig{WatchCacheSizes: &core.WatchCacheSizes{Default: nil}},
				BeEmpty(),
			),
			Entry("should return a warning when watchCacheSizes.default is set",
				&core.KubeAPIServerConfig{WatchCacheSizes: &core.WatchCacheSizes{Default: ptr.To[int32](50)}},
				ContainElement(Equal("you are setting the spec.shootTemplate.spec.kubernetes.kubeAPIServer.watchCacheSizes.default field. The field has been deprecated and is forbidden to be set starting from Kubernetes 1.35. The cache size is automatically sized by the kube-apiserver.")),
			),
		)

		DescribeTable("spec.shootTemplate.spec.kubernetes.kubeAPIServer.eventTTL",
			func(kubeAPIServerConfig *core.KubeAPIServerConfig, matcher gomegatypes.GomegaMatcher) {
				managedSeedSet.Spec.ShootTemplate.Spec.Kubernetes.KubeAPIServer = kubeAPIServerConfig
				Expect(GetWarnings(managedSeedSet)).To(matcher)
			},

			Entry("should not return a warning when kubeAPIServerConfig is nil",
				nil,
				BeEmpty(),
			),
			Entry("should not return a warning when eventTTL is nil",
				&core.KubeAPIServerConfig{EventTTL: nil},
				BeEmpty(),
			),
			Entry("should not return a warning for valid eventTTL duration",
				&core.KubeAPIServerConfig{EventTTL: &metav1.Duration{Duration: time.Hour * 24}},
				BeEmpty(),
			),
			Entry("should return a warning for invalid eventTTL duration",
				&core.KubeAPIServerConfig{EventTTL: &metav1.Duration{Duration: time.Hour * 24 * 10}},
				ContainElement(Equal("you are setting the spec.shootTemplate.spec.kubernetes.kubeAPIServer.eventTTL field to an invalid value. Invalid value: '240h0m0s', valid values: [0, 24h]. Invalid values for existing resources will be no longer allowed in Gardener v1.142.0. See: https://github.com/gardener/gardener/issues/13825")),
			),
		)

		DescribeTable("aesgcm encryption provider type",
			func(etcdEncryptionKeyAutoRotationEnabled bool, providerType *core.EncryptionProviderType, matcher gomegatypes.GomegaMatcher) {
				if etcdEncryptionKeyAutoRotationEnabled {
					managedSeedSet.Spec.ShootTemplate.Spec.Maintenance = &core.Maintenance{
						AutoRotation: &core.MaintenanceAutoRotation{
							Credentials: &core.MaintenanceCredentialsAutoRotation{
								ETCDEncryptionKey: &core.MaintenanceRotationConfig{RotationPeriod: &metav1.Duration{Duration: time.Hour}},
							},
						},
					}
				}
				managedSeedSet.Spec.ShootTemplate.Spec.Kubernetes.KubeAPIServer = &core.KubeAPIServerConfig{
					EncryptionConfig: &core.EncryptionConfig{
						Provider: core.EncryptionProvider{
							Type: providerType,
						},
					},
				}

				Expect(GetWarnings(managedSeedSet)).To(matcher)
			},

			Entry("should return a warning when encryption provider type is aesgcm and auto rotation is not enabled",
				false, ptr.To(core.EncryptionProviderTypeAESGCM),
				ContainElement(Equal("aesgcm encryption provider type is not recommended to be used without enabling auto encryption key rotation in the maintenance window. For enabling auto rotation, see: https://github.com/gardener/gardener/blob/master/docs/usage/shoot/shoot_maintenance.md#automatic-credentials-rotation"))),
			Entry("should not return a warning when encryption provider type is aesgcm and auto rotation is enabled",
				true, ptr.To(core.EncryptionProviderTypeAESGCM), BeEmpty()),
			Entry("should not return a warning when encryption provider type is not aesgcm and auto rotation is not enabled",
				false, ptr.To(core.EncryptionProviderTypeSecretbox), BeEmpty()),
			Entry("should not return a warning when encryption provider type is not aesgcm and auto rotation is enabled",
				true, ptr.To(core.EncryptionProviderTypeSecretbox), BeEmpty()),
			Entry("should not return a warning when encryption provider type is not set and auto rotation is not enabled",
				false, nil, BeEmpty()),
		)
	})
})
