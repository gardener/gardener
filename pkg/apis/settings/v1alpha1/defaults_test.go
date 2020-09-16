// SPDX-FileCopyrightText: 2019 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package v1alpha1_test

import (
	"testing"

	"github.com/gardener/gardener/pkg/apis/settings/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestAPI(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Settings V1alpha1 Suite")
}

var _ = Describe("Defaults", func() {

	Describe("SetDefaults_OpenIDConnectPreset", func() {

		It("correct defaults are set", func() {
			given := &v1alpha1.OpenIDConnectPreset{}
			expected := &v1alpha1.OpenIDConnectPreset{
				Spec: defaultSpec(),
			}

			v1alpha1.SetDefaults_OpenIDConnectPreset(given)

			Expect(given).To(BeEquivalentTo(expected))
		})

	})

	Describe("SetDefaults_ClusterOpenIDConnectPreset", func() {

		It("correct defaults are set", func() {
			given := &v1alpha1.ClusterOpenIDConnectPreset{}
			expected := &v1alpha1.ClusterOpenIDConnectPreset{
				Spec: v1alpha1.ClusterOpenIDConnectPresetSpec{
					OpenIDConnectPresetSpec: defaultSpec(),
					ProjectSelector:         &metav1.LabelSelector{},
				},
			}

			v1alpha1.SetDefaults_ClusterOpenIDConnectPreset(given)

			Expect(given).To(BeEquivalentTo(expected))
		})

	})

})

func defaultSpec() v1alpha1.OpenIDConnectPresetSpec {
	usernameClaim := "sub"
	return v1alpha1.OpenIDConnectPresetSpec{
		Server: v1alpha1.KubeAPIServerOpenIDConnect{
			// string literal are used to be sure that the test fails
			// if the constant values are changed.
			UsernameClaim: &usernameClaim,
			SigningAlgs:   []string{"RS256"},
		},
		ShootSelector: &metav1.LabelSelector{},
	}
}
