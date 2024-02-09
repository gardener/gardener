// Copyright 2024 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package cache_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener/pkg/component/monitoring/prometheus/cache"
	"github.com/gardener/gardener/pkg/utils"
)

var _ = Describe("NetworkPolicy", func() {
	Describe("#NetworkPolicyToNodeExporter", func() {
		It("should return the expected network policy", func() {
			Expect(cache.NetworkPolicyToNodeExporter("foo")).To(Equal(&networkingv1.NetworkPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "egress-from-cache-prometheus-to-kube-system-node-exporter-tcp-16909",
					Namespace: "foo",
				},
				Spec: networkingv1.NetworkPolicySpec{
					PodSelector: metav1.LabelSelector{
						MatchLabels: map[string]string{"prometheus": "cache"},
					},
					Egress: []networkingv1.NetworkPolicyEgressRule{{
						To:    []networkingv1.NetworkPolicyPeer{},
						Ports: []networkingv1.NetworkPolicyPort{{Port: utils.IntStrPtrFromInt32(16909), Protocol: ptr.To(corev1.ProtocolTCP)}},
					}},
					Ingress:     []networkingv1.NetworkPolicyIngressRule{},
					PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeEgress},
				},
			}))
		})
	})
})
