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
