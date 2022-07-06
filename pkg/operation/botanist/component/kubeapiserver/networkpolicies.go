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

package kubeapiserver

import (
	"context"
	"fmt"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/operation/botanist/component/etcd"
	"github.com/gardener/gardener/pkg/operation/botanist/component/monitoring"
	"github.com/gardener/gardener/pkg/operation/botanist/component/vpnseedserver"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	networkPolicyNameAllowFromShootAPIServer = "allow-from-shoot-apiserver"
	networkPolicyNameAllowToShootAPIServer   = "allow-to-shoot-apiserver"
	networkPolicyNameAllowKubeAPIServer      = "allow-" + v1beta1constants.DeploymentNameKubeAPIServer
)

func (k *kubeAPIServer) emptyNetworkPolicy(name string) *networkingv1.NetworkPolicy {
	return &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: k.namespace}}
}

func (k *kubeAPIServer) reconcileNetworkPolicyAllowFromShootAPIServer(ctx context.Context, networkPolicy *networkingv1.NetworkPolicy) error {
	_, err := controllerutils.GetAndCreateOrMergePatch(ctx, k.client.Client(), networkPolicy, func() error {
		networkPolicy.Annotations = map[string]string{
			v1beta1constants.GardenerDescription: fmt.Sprintf("Allows Egress from Shoot's Kubernetes API Server "+
				"to talk to pods labeled with '%s=%s'.", v1beta1constants.LabelNetworkPolicyFromShootAPIServer,
				v1beta1constants.LabelNetworkPolicyAllowed),
		}
		networkPolicy.Spec = networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					v1beta1constants.LabelNetworkPolicyFromShootAPIServer: v1beta1constants.LabelNetworkPolicyAllowed,
				},
			},
			Ingress: []networkingv1.NetworkPolicyIngressRule{{
				From: []networkingv1.NetworkPolicyPeer{{
					PodSelector: &metav1.LabelSelector{
						MatchLabels: GetLabels(),
					},
				}},
			}},
			PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress},
		}
		return nil
	})
	return err
}

func (k *kubeAPIServer) reconcileNetworkPolicyAllowToShootAPIServer(ctx context.Context, networkPolicy *networkingv1.NetworkPolicy) error {
	var (
		protocol = corev1.ProtocolTCP
		port     = intstr.FromInt(Port)
	)

	_, err := controllerutils.GetAndCreateOrMergePatch(ctx, k.client.Client(), networkPolicy, func() error {
		networkPolicy.Annotations = map[string]string{
			v1beta1constants.GardenerDescription: fmt.Sprintf("Allows Egress from pods labeled with '%s=%s' to "+
				"talk to Shoot's Kubernetes API Server.", v1beta1constants.LabelNetworkPolicyToShootAPIServer,
				v1beta1constants.LabelNetworkPolicyAllowed),
		}
		networkPolicy.Spec = networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					v1beta1constants.LabelNetworkPolicyToShootAPIServer: v1beta1constants.LabelNetworkPolicyAllowed,
				},
			},
			Egress: []networkingv1.NetworkPolicyEgressRule{{
				To: []networkingv1.NetworkPolicyPeer{{
					PodSelector: &metav1.LabelSelector{
						MatchLabels: GetLabels(),
					},
				}},
				Ports: []networkingv1.NetworkPolicyPort{{
					Protocol: &protocol,
					Port:     &port,
				}},
			}},
			PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeEgress},
		}
		return nil
	})
	return err
}

func (k *kubeAPIServer) reconcileNetworkPolicyAllowKubeAPIServer(ctx context.Context, networkPolicy *networkingv1.NetworkPolicy) error {
	var (
		protocol             = corev1.ProtocolTCP
		portAPIServer        = intstr.FromInt(Port)
		portEtcd             = intstr.FromInt(int(etcd.PortEtcdClient))
		portBlackboxExporter = intstr.FromInt(monitoring.BlackboxExporterPort)
		portVPNSeedServer    = intstr.FromInt(vpnseedserver.EnvoyPort)
	)

	_, err := controllerutils.GetAndCreateOrMergePatch(ctx, k.client.Client(), networkPolicy, func() error {
		networkPolicy.Annotations = map[string]string{
			v1beta1constants.GardenerDescription: fmt.Sprintf("Allows Ingress to the Shoot's Kubernetes API "+
				"Server from pods labeled with '%s=%s' and Prometheus, and Egress to etcd pods.",
				v1beta1constants.LabelNetworkPolicyToShootAPIServer, v1beta1constants.LabelNetworkPolicyAllowed),
		}
		networkPolicy.Spec = networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: GetLabels(),
			},
			Egress: []networkingv1.NetworkPolicyEgressRule{{
				// Allow connection to shoot's etcd instances.
				To: []networkingv1.NetworkPolicyPeer{{
					PodSelector: &metav1.LabelSelector{
						MatchLabels: etcd.GetLabels(),
					},
				}},
				Ports: []networkingv1.NetworkPolicyPort{{
					Protocol: &protocol,
					Port:     &portEtcd,
				}},
			}},
			// Allow connections from everything which needs to talk to the API server.
			Ingress: []networkingv1.NetworkPolicyIngressRule{
				{
					From: []networkingv1.NetworkPolicyPeer{
						// Allow all other Pods in the Seed cluster to access it.
						{PodSelector: &metav1.LabelSelector{}, NamespaceSelector: &metav1.LabelSelector{}},
						// kube-apiserver can be accessed from anywhere using the LoadBalancer.
						{IPBlock: &networkingv1.IPBlock{CIDR: "0.0.0.0/0"}},
					},
					Ports: []networkingv1.NetworkPolicyPort{{
						Protocol: &protocol,
						Port:     &portAPIServer,
					}},
				},
				{
					From: []networkingv1.NetworkPolicyPeer{{
						PodSelector: &metav1.LabelSelector{
							MatchLabels: monitoring.GetPrometheusLabels(),
						},
					}},
					Ports: []networkingv1.NetworkPolicyPort{
						{
							Protocol: &protocol,
							Port:     &portBlackboxExporter,
						},
						{
							Protocol: &protocol,
							Port:     &portAPIServer,
						},
					},
				},
			},
			PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress, networkingv1.PolicyTypeEgress},
		}

		if k.values.VPN.ReversedVPNEnabled {
			networkPolicy.Spec.Egress = append(networkPolicy.Spec.Egress, networkingv1.NetworkPolicyEgressRule{
				To: []networkingv1.NetworkPolicyPeer{{
					PodSelector: &metav1.LabelSelector{
						MatchLabels: vpnseedserver.GetLabels(),
					},
				}},
				Ports: []networkingv1.NetworkPolicyPort{{
					Protocol: &protocol,
					Port:     &portVPNSeedServer,
				}},
			})
		}

		return nil
	})
	return err
}
