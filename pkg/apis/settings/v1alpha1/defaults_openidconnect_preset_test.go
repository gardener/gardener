// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package v1alpha1_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/gardener/gardener/pkg/apis/settings/v1alpha1"
)

var _ = Describe("OpenIDConnectPreset defaulting", func() {
	var (
		given    *v1alpha1.OpenIDConnectPreset
		expected *v1alpha1.OpenIDConnectPreset
	)

	BeforeEach(func() {
		given = &v1alpha1.OpenIDConnectPreset{}
		usernameClaim := "sub"
		expected = &v1alpha1.OpenIDConnectPreset{
			Spec: v1alpha1.OpenIDConnectPresetSpec{
				Server: v1alpha1.KubeAPIServerOpenIDConnect{
					// string literal are used to be sure that the test fails
					// if the constant values are changed.
					UsernameClaim: &usernameClaim,
					SigningAlgs:   []string{"RS256"},
				},
				ShootSelector: &metav1.LabelSelector{},
			},
		}
	})

	It("should default OpenIDConnectPreset correctly", func() {

		v1alpha1.SetDefaults_OpenIDConnectPreset(given)

		Expect(given).To(BeEquivalentTo(expected))
	})

	It("should not default ShootSelector if it is already set", func() {
		given.Spec.ShootSelector = &metav1.LabelSelector{MatchLabels: map[string]string{"foo": "bar"}}

		v1alpha1.SetDefaults_OpenIDConnectPreset(given)

		expected.Spec.ShootSelector = &metav1.LabelSelector{MatchLabels: map[string]string{"foo": "bar"}}
		Expect(given).To(BeEquivalentTo(expected))

	})

	It("should not default SigningAlgs if they are already set", func() {
		given.Spec.Server.SigningAlgs = []string{"alg1", "alg2"}

		v1alpha1.SetDefaults_OpenIDConnectPreset(given)

		expected.Spec.Server.SigningAlgs = []string{"alg1", "alg2"}
		Expect(given).To(BeEquivalentTo(expected))

	})

	It("should not default UsernameClaim if it is already set", func() {
		usernameClaim := "usr"
		given.Spec.Server.UsernameClaim = &usernameClaim

		v1alpha1.SetDefaults_OpenIDConnectPreset(given)

		expectedUsernameClaim := "usr"
		expected.Spec.Server.UsernameClaim = &expectedUsernameClaim
		Expect(given).To(BeEquivalentTo(expected))

	})

})
