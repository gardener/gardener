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
	"k8s.io/utils/pointer"

	. "github.com/gardener/gardener/pkg/apis/settings/v1alpha1"
)

var _ = Describe("OpenIDConnectPreset defaulting", func() {
	var (
		given    *OpenIDConnectPreset
		expected *OpenIDConnectPreset
	)

	BeforeEach(func() {
		given = &OpenIDConnectPreset{}
		expected = &OpenIDConnectPreset{
			Spec: OpenIDConnectPresetSpec{
				Server: KubeAPIServerOpenIDConnect{
					// string literal are used to be sure that the test fails
					// if the constant values are changed.
					UsernameClaim: pointer.String("sub"),
					SigningAlgs:   []string{"RS256"},
				},
				ShootSelector: &metav1.LabelSelector{},
			},
		}
	})

	It("should default OpenIDConnectPreset correctly", func() {
		SetObjectDefaults_OpenIDConnectPreset(given)

		Expect(given).To(BeEquivalentTo(expected))
	})

	It("should not default ShootSelector if it is already set", func() {
		given.Spec.ShootSelector = &metav1.LabelSelector{MatchLabels: map[string]string{"foo": "bar"}}
		expected.Spec.ShootSelector = &metav1.LabelSelector{MatchLabels: map[string]string{"foo": "bar"}}

		SetObjectDefaults_OpenIDConnectPreset(given)

		Expect(given).To(BeEquivalentTo(expected))
	})

	It("should not default SigningAlgs if they are already set", func() {
		given.Spec.Server.SigningAlgs = []string{"alg1", "alg2"}
		expected.Spec.Server.SigningAlgs = []string{"alg1", "alg2"}

		SetObjectDefaults_OpenIDConnectPreset(given)

		Expect(given).To(BeEquivalentTo(expected))
	})

	It("should not default UsernameClaim if it is already set", func() {
		given.Spec.Server.UsernameClaim = pointer.String("usr")
		expected.Spec.Server.UsernameClaim = pointer.String("usr")

		SetObjectDefaults_OpenIDConnectPreset(given)

		Expect(given).To(BeEquivalentTo(expected))
	})
})
