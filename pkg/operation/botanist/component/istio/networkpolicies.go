// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package istio

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/operation/botanist/component/vpnauthzserver"
	"github.com/gardener/gardener/pkg/utils"
)

const (
	istiodAppLabelValue = "istiod"
)

type networkPolicyTransformer struct {
	name      string
	transform func(*networkingv1.NetworkPolicy) func() error
}

func getIstioIngressNetworkPolicyTransformers() []networkPolicyTransformer {
	return []networkPolicyTransformer{
		// TODO(timuthy, rfranzke): Replace rule as soon as Networkpolicy controller is activated for the 'garden' namespace.
		{
			name: "allow-to-reversed-vpn-auth-server",
			transform: func(obj *networkingv1.NetworkPolicy) func() error {
				return func() error {
					obj.Annotations = map[string]string{
						v1beta1constants.GardenerDescription: fmt.Sprintf("Allows Egress from pods labeled with "+
							"'%s=%s' to reversed vpn auth servers with label '%s=%s' in namespace %s.",
							v1beta1constants.LabelApp, v1beta1constants.DefaultIngressGatewayAppLabelValue,
							v1beta1constants.LabelApp, vpnauthzserver.Name,
							v1beta1constants.GardenNamespace,
						),
					}
					obj.Spec = networkingv1.NetworkPolicySpec{
						PodSelector: metav1.LabelSelector{
							MatchLabels: map[string]string{
								v1beta1constants.LabelApp: v1beta1constants.DefaultIngressGatewayAppLabelValue,
							},
						},
						PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeEgress},
						Egress: []networkingv1.NetworkPolicyEgressRule{{
							To: []networkingv1.NetworkPolicyPeer{{
								PodSelector: &metav1.LabelSelector{
									MatchLabels: map[string]string{
										v1beta1constants.LabelApp: vpnauthzserver.Name,
									},
								},
								NamespaceSelector: &metav1.LabelSelector{
									MatchLabels: map[string]string{
										v1beta1constants.LabelRole: v1beta1constants.GardenNamespace,
									},
								},
							}},
							Ports: []networkingv1.NetworkPolicyPort{{
								Protocol: utils.ProtocolPtr(corev1.ProtocolTCP),
								Port:     utils.IntStrPtrFromInt(vpnauthzserver.ServerPort),
							}},
						}},
					}
					return nil
				}
			},
		},
	}
}
