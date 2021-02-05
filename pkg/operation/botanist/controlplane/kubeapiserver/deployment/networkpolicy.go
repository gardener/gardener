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

package deployment

import (
	"context"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/operation/botanist/controlplane/etcd"
	"github.com/gardener/gardener/pkg/operation/botanist/controlplane/konnectivity"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (k *kubeAPIServer) deployNetworkPolicies(ctx context.Context) error {
	var (
		netpolAllowFromShootApiserver = k.emptyNetworkPolicy("allow-from-shoot-apiserver")
		netpolAllowKubeApiserver      = k.emptyNetworkPolicy("allow-kube-apiserver")
		netpolAllowToShootApiserver   = k.emptyNetworkPolicy("allow-to-shoot-apiserver")
	)

	err := k.deployAllowFromShootAPIServer(ctx, netpolAllowFromShootApiserver)
	if err != nil {
		return err
	}

	err = k.deployAllowKubeAPIServer(ctx, netpolAllowKubeApiserver)
	if err != nil {
		return err
	}

	err = k.deployAllowToShootAPIServer(ctx, netpolAllowToShootApiserver)
	if err != nil {
		return err
	}

	return nil
}

func (k *kubeAPIServer) deployAllowKubeAPIServer(ctx context.Context, netpolAllowKubeApiserver *networkingv1.NetworkPolicy) error {
	tcp := corev1.ProtocolTCP
	port443 := intstr.FromInt(443)
	etcdClientPort := intstr.FromInt(etcd.PortEtcdClient)
	konnectivityAgentPort := intstr.FromInt(int(konnectivity.ServerAgentPort))
	blackboxExporterPort := intstr.FromInt(9115)

	if _, err := controllerutil.CreateOrUpdate(ctx, k.seedClient.Client(), netpolAllowKubeApiserver, func() error {
		netpolAllowKubeApiserver.Annotations = map[string]string{
			v1beta1constants.GardenerDescription: `|
Allows Ingress to the Shoot's Kubernetes API Server from pods labeled with 'networking.gardener.cloud/to-shoot-apiserver=allowed'
and Prometheus, and Egress to etcd pods.
`,
		}

		netpolAllowKubeApiserver.Spec = networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: getAPIServerPodLabels(),
			},
			Egress: []networkingv1.NetworkPolicyEgressRule{
				{
					// Allow connection to shoot's etcd instances.
					To: []networkingv1.NetworkPolicyPeer{
						{
							PodSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									v1beta1constants.LabelApp:             etcd.LabelAppValue,
									v1beta1constants.DeprecatedGardenRole: v1beta1constants.GardenRoleControlPlane,
								},
							},
						},
					},
					Ports: []networkingv1.NetworkPolicyPort{
						{
							Protocol: &tcp,
							Port:     &etcdClientPort,
						},
					},
				},
			},
			// Allow connection from everything which needs to talk to the API server
			Ingress: []networkingv1.NetworkPolicyIngressRule{
				{
					// allow all other Pods in the Seed cluster to access it.
					From: []networkingv1.NetworkPolicyPeer{
						{
							// allow all other Pods in the Seed cluster to access it.
							PodSelector: &metav1.LabelSelector{},
						},
						{
							// kube-apiserver can be accessed from anywhere using the LoadBalancer.
							IPBlock: &networkingv1.IPBlock{
								CIDR: "0.0.0.0/0",
							},
						},
					},
					Ports: []networkingv1.NetworkPolicyPort{
						{
							Protocol: &tcp,
							Port:     &port443,
						},
					},
				},
				{
					// allow from prometheus on scrape port.
					From: []networkingv1.NetworkPolicyPeer{
						{
							// allow all other Pods in the Seed cluster to access it.
							PodSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									v1beta1constants.LabelApp:             v1beta1constants.LabelPrometheus,
									v1beta1constants.DeprecatedGardenRole: v1beta1constants.GardenRoleMonitoring,
									v1beta1constants.LabelRole:            v1beta1constants.GardenRoleMonitoring,
								},
							},
						},
					},
					Ports: []networkingv1.NetworkPolicyPort{
						{
							Protocol: &tcp,
							Port:     &blackboxExporterPort,
						},
						{
							Protocol: &tcp,
							Port:     &port443,
						},
					},
				},
			},
			PolicyTypes: []networkingv1.PolicyType{
				networkingv1.PolicyTypeEgress,
				networkingv1.PolicyTypeIngress,
			},
		}

		if k.konnectivityTunnelEnabled && !k.sniValues.SNIEnabled {
			netpolAllowKubeApiserver.Spec.Egress = append(netpolAllowKubeApiserver.Spec.Egress, networkingv1.NetworkPolicyEgressRule{
				To: []networkingv1.NetworkPolicyPeer{
					{
						// Allow connections from the apiserver pod to itself (i.e., konnectivity-server container to apiserver)
						PodSelector: &metav1.LabelSelector{
							MatchLabels: getAPIServerPodLabels(),
						},
					},
				},
				Ports: []networkingv1.NetworkPolicyPort{
					{
						Protocol: &tcp,
						Port:     &port443,
					},
				},
			})

			// allow ingress from konnectivity agents in the Shoot
			netpolAllowKubeApiserver.Spec.Ingress[0].Ports = append(netpolAllowKubeApiserver.Spec.Ingress[0].Ports, networkingv1.NetworkPolicyPort{
				Protocol: &tcp,
				Port:     &konnectivityAgentPort,
			})

			// Allow connections from the apiserver pod to itself (i.e., konnectivity-server to apiserver)
			netpolAllowKubeApiserver.Spec.Ingress[1].From = append(netpolAllowKubeApiserver.Spec.Ingress[1].From, networkingv1.NetworkPolicyPeer{
				PodSelector: &metav1.LabelSelector{
					MatchLabels: getAPIServerPodLabels(),
				},
			})

		}

		// in this case the konnectivity server is a separate deployment
		// Allow connections from the APIServer pod to the konnectivity-server pod
		// on its TLS port
		if k.konnectivityTunnelEnabled && k.sniValues.SNIEnabled {
			konnectivitiyServerHTTPSPort := intstr.FromInt(int(konnectivity.ServerHTTPSPort))
			netpolAllowKubeApiserver.Spec.Egress = append(netpolAllowKubeApiserver.Spec.Egress, networkingv1.NetworkPolicyEgressRule{
				To: []networkingv1.NetworkPolicyPeer{
					{
						PodSelector: &metav1.LabelSelector{
							MatchLabels: konnectivity.GetLabels(),
						},
					},
				},
				Ports: []networkingv1.NetworkPolicyPort{
					{
						Protocol: &tcp,
						Port:     &konnectivitiyServerHTTPSPort,
					},
				},
			})
		}

		return nil
	}); err != nil {
		return err
	}
	return nil
}

func (k *kubeAPIServer) deployAllowToShootAPIServer(ctx context.Context, netpolAllowToShootApiserver *networkingv1.NetworkPolicy) error {
	tcp := corev1.ProtocolTCP
	port443 := intstr.FromInt(443)

	if _, err := controllerutil.CreateOrUpdate(ctx, k.seedClient.Client(), netpolAllowToShootApiserver, func() error {
		netpolAllowToShootApiserver.Annotations = map[string]string{
			v1beta1constants.GardenerDescription: `|
Allows Egress from pods labeled with 'networking.gardener.cloud/to-shoot-apiserver=allowed'
to talk to Shoot's Kubernetes API Server.
`,
		}

		netpolAllowToShootApiserver.Spec = networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					v1beta1constants.LabelNetworkPolicyToShootAPIServer: v1beta1constants.LabelNetworkPolicyAllowed,
				},
			},
			Egress: []networkingv1.NetworkPolicyEgressRule{
				{
					To: []networkingv1.NetworkPolicyPeer{
						{
							PodSelector: &metav1.LabelSelector{
								MatchLabels: getAPIServerPodLabels(),
							},
						},
					},
					Ports: []networkingv1.NetworkPolicyPort{
						{
							Protocol: &tcp,
							Port:     &port443,
						},
					},
				},
			},
			PolicyTypes: []networkingv1.PolicyType{
				networkingv1.PolicyTypeEgress,
			},
			Ingress: []networkingv1.NetworkPolicyIngressRule{},
		}
		return nil
	}); err != nil {
		return err
	}
	return nil
}

func (k *kubeAPIServer) deployAllowFromShootAPIServer(ctx context.Context, netpolAllowFromShootApiserver *networkingv1.NetworkPolicy) error {
	if _, err := controllerutil.CreateOrUpdate(ctx, k.seedClient.Client(), netpolAllowFromShootApiserver, func() error {
		netpolAllowFromShootApiserver.Annotations = map[string]string{
			v1beta1constants.GardenerDescription: `|
Allows Egress from Shoot's Kubernetes API Server to talk to pods labeled
with 'networking.gardener.cloud/from-shoot-apiserver=allowed'.
`,
		}

		netpolAllowFromShootApiserver.Spec = networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					v1beta1constants.LabelNetworkPolicyFromShootAPIServer: v1beta1constants.LabelNetworkPolicyAllowed,
				},
			},
			Ingress: []networkingv1.NetworkPolicyIngressRule{
				{
					From: []networkingv1.NetworkPolicyPeer{
						{
							PodSelector: &metav1.LabelSelector{
								MatchLabels: getAPIServerPodLabels(),
							},
						},
					},
				},
			},
			PolicyTypes: []networkingv1.PolicyType{
				networkingv1.PolicyTypeIngress,
			},
			Egress: []networkingv1.NetworkPolicyEgressRule{},
		}
		return nil
	}); err != nil {
		return err
	}
	return nil
}

func (k *kubeAPIServer) emptyNetworkPolicy(name string) *networkingv1.NetworkPolicy {
	return &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: k.seedNamespace}}
}
