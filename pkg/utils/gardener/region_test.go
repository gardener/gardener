// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gardener_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubernetesscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	. "github.com/gardener/gardener/pkg/utils/gardener"
)

var _ = Describe("Region", func() {
	var configMap *corev1.ConfigMap

	BeforeEach(func() {
		configMap = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "region-config",
				Namespace: "garden",
				Labels: map[string]string{
					v1beta1constants.SchedulingPurpose: v1beta1constants.SchedulingPurposeRegionConfig,
				},
				Annotations: map[string]string{
					v1beta1constants.AnnotationSchedulingCloudProfiles: "aws-profile,gcp-profile",
				},
			},
		}
	})

	Describe("#GetRegionConfigMap", func() {
		var (
			ctx        context.Context
			fakeClient client.Client
		)

		BeforeEach(func() {
			ctx = context.Background()
		})

		It("should find the ConfigMap matching the cloud profile name", func() {
			fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetesscheme.Scheme).WithObjects(configMap).Build()
			cm, err := GetRegionConfigMap(ctx, fakeClient, "garden", "aws-profile")
			Expect(err).NotTo(HaveOccurred())
			Expect(cm).NotTo(BeNil())
			Expect(cm.Name).To(Equal("region-config"))
		})

		It("should find the ConfigMap for second entry in comma-separated list", func() {
			fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetesscheme.Scheme).WithObjects(configMap).Build()
			cm, err := GetRegionConfigMap(ctx, fakeClient, "garden", "gcp-profile")
			Expect(err).NotTo(HaveOccurred())
			Expect(cm).NotTo(BeNil())
			Expect(cm.Name).To(Equal("region-config"))
		})

		It("should return nil when no ConfigMap matches", func() {
			fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetesscheme.Scheme).WithObjects(configMap).Build()
			cm, err := GetRegionConfigMap(ctx, fakeClient, "garden", "unknown-profile")
			Expect(err).NotTo(HaveOccurred())
			Expect(cm).To(BeNil())
		})

		It("should handle empty list", func() {
			fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetesscheme.Scheme).Build()
			cm, err := GetRegionConfigMap(ctx, fakeClient, "garden", "aws-profile")
			Expect(err).NotTo(HaveOccurred())
			Expect(cm).To(BeNil())
		})
	})
})
