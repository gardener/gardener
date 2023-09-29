// Copyright 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package helper

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/sets"
)

// GetEgressRules creates Network Policy egress rules from endpoint subsets.
func GetEgressRules(subsets ...corev1.EndpointSubset) []networkingv1.NetworkPolicyEgressRule {
	var egressRules []networkingv1.NetworkPolicyEgressRule

	for _, subset := range subsets {
		egressRule := networkingv1.NetworkPolicyEgressRule{}
		existingIPs := sets.New[string]()

		for _, address := range subset.Addresses {
			if existingIPs.Has(address.IP) {
				continue
			}

			existingIPs.Insert(address.IP)

			egressRule.To = append(egressRule.To, networkingv1.NetworkPolicyPeer{
				IPBlock: &networkingv1.IPBlock{
					CIDR: fmt.Sprintf("%s/32", address.IP),
				},
			})
		}

		for _, port := range subset.Ports {
			// do not use named port as this is e.g not support on Weave
			parse := intstr.FromInt32(port.Port)
			networkPolicyPort := networkingv1.NetworkPolicyPort{
				Port:     &parse,
				Protocol: &port.Protocol,
			}
			egressRule.Ports = append(egressRule.Ports, networkPolicyPort)
		}

		egressRules = append(egressRules, egressRule)
	}

	return egressRules
}
