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
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	. "github.com/gardener/gardener/pkg/utils/gardener"
)

var _ = Describe("Region", func() {
	const (
		namespace    = "garden"
		profileName  = "aws-profile"
		otherProfile = "gcp-profile"
	)

	makeCM := func(name, cloudProfilesAnnotation string) *corev1.ConfigMap {
		return &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
				Labels: map[string]string{
					v1beta1constants.SchedulingPurpose: v1beta1constants.SchedulingPurposeRegionConfig,
				},
				Annotations: map[string]string{
					v1beta1constants.AnnotationSchedulingCloudProfiles: cloudProfilesAnnotation,
				},
			},
		}
	}

	Describe("#FindRegionConfigMap", func() {
		It("returns nil when no ConfigMap references the profile", func() {
			got, err := FindRegionConfigMap([]*corev1.ConfigMap{makeCM("cm-b", otherProfile)}, profileName)
			Expect(err).NotTo(HaveOccurred())
			Expect(got).To(BeNil())
		})

		It("returns the matching ConfigMap", func() {
			match := makeCM("cm-a", profileName)
			got, err := FindRegionConfigMap([]*corev1.ConfigMap{match, makeCM("cm-b", otherProfile)}, profileName)
			Expect(err).NotTo(HaveOccurred())
			Expect(got).To(Equal(match))
		})

		It("matches on comma-separated annotation values with surrounding whitespace", func() {
			match := makeCM("cm-multi", " gcp-profile , aws-profile ,azure-profile")
			got, err := FindRegionConfigMap([]*corev1.ConfigMap{match}, profileName)
			Expect(err).NotTo(HaveOccurred())
			Expect(got).To(Equal(match))
		})

		It("returns an error when multiple ConfigMaps reference the same profile", func() {
			first := makeCM("cm-a", profileName)
			second := makeCM("cm-b", profileName)
			got, err := FindRegionConfigMap([]*corev1.ConfigMap{first, second}, profileName)
			Expect(err).To(MatchError(ContainSubstring("multiple scheduler region ConfigMaps reference cloud profile")))
			Expect(err).To(MatchError(ContainSubstring("cm-a")))
			Expect(err).To(MatchError(ContainSubstring("cm-b")))
			Expect(got).To(BeNil())
		})
	})

	Describe("#GetRegionConfigMap", func() {
		var ctx = context.Background()

		It("finds a ConfigMap via the reader", func() {
			match := makeCM("cm-a", profileName)
			reader := fakeclient.NewClientBuilder().WithScheme(kubernetesscheme.Scheme).WithObjects(match).Build()
			got, err := GetRegionConfigMap(ctx, reader, namespace, profileName)
			Expect(err).NotTo(HaveOccurred())
			Expect(got).NotTo(BeNil())
			Expect(got.Name).To(Equal("cm-a"))
		})

		It("returns nil when the reader finds no matching ConfigMap", func() {
			reader := fakeclient.NewClientBuilder().WithScheme(kubernetesscheme.Scheme).WithObjects(makeCM("cm-b", otherProfile)).Build()
			got, err := GetRegionConfigMap(ctx, reader, namespace, profileName)
			Expect(err).NotTo(HaveOccurred())
			Expect(got).To(BeNil())
		})

		It("propagates the duplicate-detection error from FindRegionConfigMap", func() {
			reader := fakeclient.NewClientBuilder().WithScheme(kubernetesscheme.Scheme).WithObjects(
				makeCM("cm-a", profileName),
				makeCM("cm-b", profileName),
			).Build()
			_, err := GetRegionConfigMap(ctx, reader, namespace, profileName)
			Expect(err).To(MatchError(ContainSubstring("multiple scheduler region ConfigMaps")))
		})
	})
})
