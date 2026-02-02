// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package garden_test

import (
	"time"

	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	. "github.com/gardener/gardener/pkg/operator/webhook/validation/garden"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gomegatypes "github.com/onsi/gomega/types"
)

var _ = Describe("Warnings", func() {
	Describe("#GetWarnings", func() {
		var garden *operatorv1alpha1.Garden

		BeforeEach(func() {
			garden = &operatorv1alpha1.Garden{}
		})

		DescribeTable("spec.virtualCluster.kubernetes.kubeAPIServer.enableAnonymousAuthentication",
			func(kubeAPIServerConfig *operatorv1alpha1.KubeAPIServerConfig, matcher gomegatypes.GomegaMatcher) {
				garden.Spec.VirtualCluster.Kubernetes.KubeAPIServer = kubeAPIServerConfig
				Expect(GetWarnings(garden)).To(matcher)
			},

			Entry("should not return a warning when kubeAPIServerConfig is nil",
				nil,
				BeEmpty(),
			),
			Entry("should not return a warning when enableAnonymousAuthentication is nil",
				&operatorv1alpha1.KubeAPIServerConfig{KubeAPIServerConfig: &gardencorev1beta1.KubeAPIServerConfig{EnableAnonymousAuthentication: nil}},
				BeEmpty(),
			),
			Entry("should return a warning when enableAnonymousAuthentication is set",
				&operatorv1alpha1.KubeAPIServerConfig{KubeAPIServerConfig: &gardencorev1beta1.KubeAPIServerConfig{EnableAnonymousAuthentication: ptr.To(true)}},
				ContainElement(Equal("you are setting the spec.virtualCluster.kubernetes.kubeAPIServer.enableAnonymousAuthentication field. The field is deprecated. Using Kubernetes v1.32 and above, please use anonymous authentication configuration. See: https://kubernetes.io/docs/reference/access-authn-authz/authentication/#anonymous-authenticator-configuration")),
			),
		)

		DescribeTable("spec.kubernetes.kubeAPIServer.watchCacheSizes.default",
			func(kubeAPIServerConfig *operatorv1alpha1.KubeAPIServerConfig, matcher gomegatypes.GomegaMatcher) {
				garden.Spec.VirtualCluster.Kubernetes.KubeAPIServer = kubeAPIServerConfig
				Expect(GetWarnings(garden)).To(matcher)
			},

			Entry("should not return a warning when kubeAPIServerConfig is nil",
				nil,
				BeEmpty(),
			),
			Entry("should not return a warning when watchCacheSizes is nil",
				&operatorv1alpha1.KubeAPIServerConfig{KubeAPIServerConfig: &gardencorev1beta1.KubeAPIServerConfig{WatchCacheSizes: nil}},
				BeEmpty(),
			),
			Entry("should not return a warning when watchCacheSizes.default is nil",
				&operatorv1alpha1.KubeAPIServerConfig{KubeAPIServerConfig: &gardencorev1beta1.KubeAPIServerConfig{WatchCacheSizes: &gardencorev1beta1.WatchCacheSizes{Default: nil}}},
				BeEmpty(),
			),
			Entry("should return a warning when watchCacheSizes.default is set",
				&operatorv1alpha1.KubeAPIServerConfig{KubeAPIServerConfig: &gardencorev1beta1.KubeAPIServerConfig{WatchCacheSizes: &gardencorev1beta1.WatchCacheSizes{Default: ptr.To[int32](50)}}},
				ContainElement(Equal("you are setting the spec.virtualCluster.kubernetes.kubeAPIServer.watchCacheSizes.default field. The field has been deprecated and is forbidden to be set starting from Kubernetes 1.35. The cache size is automatically sized by the kube-apiserver.")),
			),
		)

		DescribeTable("spec.virtualCluster.kubernetes.kubeAPIServer.eventTTL",
			func(kubeAPIServerConfig *operatorv1alpha1.KubeAPIServerConfig, matcher gomegatypes.GomegaMatcher) {
				garden.Spec.VirtualCluster.Kubernetes.KubeAPIServer = kubeAPIServerConfig
				Expect(GetWarnings(garden)).To(matcher)
			},

			Entry("should not return a warning when kubeAPIServerConfig is nil",
				nil,
				BeEmpty(),
			),
			Entry("should not return a warning when eventTTL is nil",
				&operatorv1alpha1.KubeAPIServerConfig{KubeAPIServerConfig: &gardencorev1beta1.KubeAPIServerConfig{EventTTL: nil}},
				BeEmpty(),
			),
			Entry("should not return a warning for valid eventTTL duration",
				&operatorv1alpha1.KubeAPIServerConfig{KubeAPIServerConfig: &gardencorev1beta1.KubeAPIServerConfig{EventTTL: &metav1.Duration{Duration: time.Hour * 24}}},
				BeEmpty(),
			),
			Entry("should return a warning for invalid eventTTL duration",
				&operatorv1alpha1.KubeAPIServerConfig{KubeAPIServerConfig: &gardencorev1beta1.KubeAPIServerConfig{EventTTL: &metav1.Duration{Duration: time.Hour * 24 * 10}}},
				ContainElement(Equal("you are setting the spec.virtualCluster.kubernetes.kubeAPIServer.eventTTL field to an invalid value. Invalid value: '240h0m0s', valid values: [0, 24h]. Invalid values for existing resources will be no longer allowed in Gardener v1.142.0. See: https://github.com/gardener/gardener/issues/13825")),
			),
		)
	})
})
