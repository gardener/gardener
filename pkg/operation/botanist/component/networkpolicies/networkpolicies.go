// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package networkpolicies

import (
	"context"
	"fmt"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/operation/botanist/component"

	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Values contains deployment parameters for the network policies.
type Values struct {
	// ShootNetworkPeers is the list of peers for the shoot networks.
	ShootNetworkPeers []networkingv1.NetworkPolicyPeer
	// GlobalValues are the values for the global network policies.
	GlobalValues
}

// New creates a new instance of DeployWaiter for the network policies.
func New(client client.Client, namespace string, values Values) component.Deployer {
	return &networkPolicies{
		client:    client,
		namespace: namespace,
		values:    values,
	}
}

type networkPolicies struct {
	client    client.Client
	namespace string
	values    Values
}

func (n *networkPolicies) Deploy(ctx context.Context) error {
	for _, transformer := range n.getNetworkPolicyTransformers(n.values) {
		obj := &networkingv1.NetworkPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      transformer.name,
				Namespace: n.namespace,
			},
		}

		if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, n.client, obj, transformer.transform(obj)); err != nil {
			return err
		}
	}

	return nil
}

func (n *networkPolicies) Destroy(ctx context.Context) error {
	for _, transformer := range n.getNetworkPolicyTransformers(n.values) {
		if err := n.client.Delete(ctx, &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: transformer.name, Namespace: n.namespace}}); client.IgnoreNotFound(err) != nil {
			return err
		}
	}

	return nil
}

func (n *networkPolicies) getNetworkPolicyTransformers(values Values) []networkPolicyTransformer {
	return append(getGlobalNetworkPolicyTransformers(n.values.GlobalValues),
		networkPolicyTransformer{
			name: "allow-to-shoot-networks",
			transform: func(obj *networkingv1.NetworkPolicy) func() error {
				return func() error {
					obj.Annotations = map[string]string{
						v1beta1constants.GardenerDescription: fmt.Sprintf("Allows Egress from pods labeled with "+
							"'%s=%s' to IPv4 blocks belonging to the Shoot network. In practice, this should be used by "+
							"components which use 'vpn-seed' to communicate to Pods in the Shoot cluster.",
							v1beta1constants.LabelNetworkPolicyToShootNetworks, v1beta1constants.LabelNetworkPolicyAllowed),
					}
					obj.Spec = networkingv1.NetworkPolicySpec{
						PodSelector: metav1.LabelSelector{
							MatchLabels: map[string]string{
								v1beta1constants.LabelNetworkPolicyToShootNetworks: v1beta1constants.LabelNetworkPolicyAllowed,
							},
						},
						Egress: []networkingv1.NetworkPolicyEgressRule{{
							To: values.ShootNetworkPeers,
						}},
						PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeEgress},
					}
					return nil
				}
			},
		},
	)
}
