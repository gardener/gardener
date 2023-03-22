// Copyright 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/gardener/gardener/pkg/apis/settings/v1alpha1"
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
