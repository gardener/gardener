// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package v1alpha1_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	. "github.com/gardener/gardener/pkg/apis/settings/v1alpha1"
)

var _ = Describe("ClusterOpenIDConnectPreset defaulting", func() {
	It("should default ClusterOpenIDConnectPreset correctly", func() {
		obj := &ClusterOpenIDConnectPreset{}
		expected := &ClusterOpenIDConnectPreset{
			Spec: ClusterOpenIDConnectPresetSpec{
				OpenIDConnectPresetSpec: OpenIDConnectPresetSpec{
					Server: KubeAPIServerOpenIDConnect{
						// string literal are used to be sure that the test fails
						// if the constant values are changed.
						UsernameClaim: ptr.To("sub"),
						SigningAlgs:   []string{"RS256"},
					},
					ShootSelector: &metav1.LabelSelector{},
				},
				ProjectSelector: &metav1.LabelSelector{},
			},
		}
		SetObjectDefaults_ClusterOpenIDConnectPreset(obj)

		Expect(obj).To(Equal(expected))
	})

	It("should not default ClusterOpenIDConnectPreset if it is already set", func() {
		obj := &ClusterOpenIDConnectPreset{
			Spec: ClusterOpenIDConnectPresetSpec{
				OpenIDConnectPresetSpec: OpenIDConnectPresetSpec{
					Server: KubeAPIServerOpenIDConnect{
						// string literal are used to be sure that the test fails
						// if the constant values are changed.
						UsernameClaim: ptr.To("usr"),
						SigningAlgs:   []string{"alg1", "alg2"},
					},
					ShootSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"foo": "bar"}},
				},
				ProjectSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"foo": "bar"}},
			},
		}
		expected := obj.DeepCopy()
		SetObjectDefaults_ClusterOpenIDConnectPreset(obj)

		Expect(obj).To(Equal(expected))
	})
})
