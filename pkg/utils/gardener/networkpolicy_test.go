// Copyright (c) 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package gardener_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/gardener/gardener/pkg/utils"
	. "github.com/gardener/gardener/pkg/utils/gardener"
)

var _ = Describe("NetworkPolicy", func() {
	Describe("#InjectNetworkPolicyAnnotationsForScrapeTargets", func() {
		It("should inject the annotations", func() {
			obj := &corev1.Service{}

			Expect(InjectNetworkPolicyAnnotationsForScrapeTargets(
				obj,
				networkingv1.NetworkPolicyPort{Port: utils.IntStrPtrFromInt(1234), Protocol: utils.ProtocolPtr(corev1.ProtocolTCP)},
				networkingv1.NetworkPolicyPort{Port: utils.IntStrPtrFromString("foo"), Protocol: utils.ProtocolPtr(corev1.ProtocolUDP)},
			)).Should(Succeed())

			Expect(obj.Annotations).To(And(
				HaveKeyWithValue("networking.resources.gardener.cloud/from-policy-pod-label-selector", "all-scrape-targets"),
				HaveKeyWithValue("networking.resources.gardener.cloud/from-policy-allowed-ports", `[{"protocol":"TCP","port":1234},{"protocol":"UDP","port":"foo"}]`),
			))
		})
	})

	Describe("#InjectNetworkPolicyNamespaceSelectors", func() {
		It("should inject the annotation", func() {
			obj := &corev1.Service{}

			Expect(InjectNetworkPolicyNamespaceSelectors(
				obj,
				metav1.LabelSelector{MatchLabels: map[string]string{"foo": "bar"}},
				metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{{Key: "foo", Operator: metav1.LabelSelectorOpIn, Values: []string{"bar"}}}},
			)).Should(Succeed())

			Expect(obj.Annotations).To(HaveKeyWithValue("networking.resources.gardener.cloud/namespace-selectors", `[{"matchLabels":{"foo":"bar"}},{"matchExpressions":[{"key":"foo","operator":"In","values":["bar"]}]}]`))
		})
	})

	Describe("#NetworkPolicyLabel", func() {
		It("should return the expected value", func() {
			Expect(NetworkPolicyLabel("foo", 1234)).To(Equal("networking.resources.gardener.cloud/to-foo-tcp-1234"))
		})
	})
})
