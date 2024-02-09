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

package cache

import (
	_ "embed"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	monitoringutils "github.com/gardener/gardener/pkg/component/monitoring/utils"
	"github.com/gardener/gardener/pkg/utils"
)

// NetworkPolicyToNodeExporter returns a NetworkPolicy that allows traffic from the cache Prometheus to the
// node-exporter pods running in `kube-system` namespace. Note that it is only applicable/relevant in case the seed
// cluster is a shoot cluster itself (otherwise, there won't be a running node-exporter (typically)).
// The gardener-resource-manager's NetworkPolicy controller is not enabled for the kube-system namespace, hence we need
// to create this custom policy for this network path.
func NetworkPolicyToNodeExporter(namespace string) *networkingv1.NetworkPolicy {
	return &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "egress-from-cache-prometheus-to-kube-system-node-exporter-tcp-16909",
			Namespace: namespace,
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: monitoringutils.Labels(Label),
			},
			Egress: []networkingv1.NetworkPolicyEgressRule{{
				// A pod selector to select the node-exporter pods in the kube-system namespace does not work here
				// because the node-exporter uses the host network. Network policies are currently not supported with
				// pods in the host network.
				// TODO: Is it possible to restrict the traffic to the nodes network CIDR of the seed?
				//  Ref https://github.com/gardener/gardener/pull/9128#discussion_r1483236610
				To:    []networkingv1.NetworkPolicyPeer{},
				Ports: []networkingv1.NetworkPolicyPort{{Port: utils.IntStrPtrFromInt32(16909), Protocol: ptr.To(corev1.ProtocolTCP)}},
			}},
			Ingress:     []networkingv1.NetworkPolicyIngressRule{},
			PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeEgress},
		},
	}
}
