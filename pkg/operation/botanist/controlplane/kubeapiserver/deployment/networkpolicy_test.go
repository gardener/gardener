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

package deployment_test

import (
	"context"

	"github.com/gardener/gardener/pkg/operation/botanist/controlplane/etcd"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	"github.com/golang/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func expectDefaultNetpolAllowFromShootAPIServer(ctx context.Context) {
	mockSeedClient.EXPECT().Get(ctx, kutil.Key(defaultSeedNamespace, "allow-from-shoot-apiserver"), gomock.AssignableToTypeOf(&networkingv1.NetworkPolicy{})).Return(apierrors.NewNotFound(schema.GroupResource{}, "foo"))

	expectedNetpolAllowFromShootApiserver := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "allow-from-shoot-apiserver",
			Namespace: defaultSeedNamespace,
			Annotations: map[string]string{
				"gardener.cloud/description": `|
Allows Egress from Shoot's Kubernetes API Server to talk to pods labeled
with 'networking.gardener.cloud/from-shoot-apiserver=allowed'.
`,
			},
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					"networking.gardener.cloud/from-shoot-apiserver": "allowed",
				},
			},
			Ingress: []networkingv1.NetworkPolicyIngressRule{
				{
					From: []networkingv1.NetworkPolicyPeer{
						{
							PodSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"gardener.cloud/role":     "controlplane",
									"garden.sapcloud.io/role": "controlplane",
									"app":                     "kubernetes",
									"role":                    "apiserver",
								},
							},
						},
					},
				},
			},
			PolicyTypes: []networkingv1.PolicyType{
				"Ingress",
			},
			Egress: []networkingv1.NetworkPolicyEgressRule{},
		},
	}
	mockSeedClient.EXPECT().Create(ctx, expectedNetpolAllowFromShootApiserver).Times(1)
}

func expectNetpolAllowKubeAPIServer(ctx context.Context, valuesProvider KubeAPIServerValuesProvider) {
	mockSeedClient.EXPECT().Get(ctx, kutil.Key(defaultSeedNamespace, "allow-kube-apiserver"), gomock.AssignableToTypeOf(&networkingv1.NetworkPolicy{})).Return(apierrors.NewNotFound(schema.GroupResource{}, "foo"))

	tcp := corev1.ProtocolTCP
	port443 := intstr.FromInt(443)
	etcdClientPort := intstr.FromInt(etcd.PortEtcdClient)
	blackboxExporterPort := intstr.FromInt(9115)

	expectedNetpolAllowKubeApiserver := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "allow-kube-apiserver",
			Namespace: defaultSeedNamespace,
			Annotations: map[string]string{
				"gardener.cloud/description": `|
Allows Ingress to the Shoot's Kubernetes API Server from pods labeled with 'networking.gardener.cloud/to-shoot-apiserver=allowed'
and Prometheus, and Egress to etcd pods.
`,
			},
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					"gardener.cloud/role":     "controlplane",
					"garden.sapcloud.io/role": "controlplane",
					"app":                     "kubernetes",
					"role":                    "apiserver",
				},
			},
			Egress: []networkingv1.NetworkPolicyEgressRule{
				{
					// Allow connection to shoot's etcd instances.
					To: []networkingv1.NetworkPolicyPeer{
						{
							PodSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"app":                     "etcd-statefulset",
									"garden.sapcloud.io/role": "controlplane",
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
									"app":                     "prometheus",
									"garden.sapcloud.io/role": "monitoring",
									"role":                    "monitoring",
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
				"Egress",
				"Ingress",
			},
		},
	}

	if valuesProvider.IsKonnectivityTunnelEnabled() && !valuesProvider.IsSNIEnabled() {
		konnectivityAgentPort := intstr.FromInt(8132)

		expectedNetpolAllowKubeApiserver.Spec.Egress = append(expectedNetpolAllowKubeApiserver.Spec.Egress, networkingv1.NetworkPolicyEgressRule{
			To: []networkingv1.NetworkPolicyPeer{
				{
					// Allow connections from the apiserver pod to itself (i.e., konnectivity-server to apiserver)
					PodSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"gardener.cloud/role":     "controlplane",
							"garden.sapcloud.io/role": "controlplane",
							"app":                     "kubernetes",
							"role":                    "apiserver",
						},
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

		expectedNetpolAllowKubeApiserver.Spec.Ingress[0].Ports = append(expectedNetpolAllowKubeApiserver.Spec.Ingress[0].Ports, networkingv1.NetworkPolicyPort{
			Protocol: &tcp,
			Port:     &konnectivityAgentPort,
		})

		// Allow connections from the apiserver pod to itself (i.e., konnectivity-server to apiserver)
		expectedNetpolAllowKubeApiserver.Spec.Ingress[1].From = append(expectedNetpolAllowKubeApiserver.Spec.Ingress[1].From, networkingv1.NetworkPolicyPeer{
			PodSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"gardener.cloud/role":     "controlplane",
					"garden.sapcloud.io/role": "controlplane",
					"app":                     "kubernetes",
					"role":                    "apiserver",
				},
			},
		})
	}

	// in this case konnectivity server is a deployment
	if valuesProvider.IsKonnectivityTunnelEnabled() && valuesProvider.IsSNIEnabled() {
		konnectivitiyServerHTTPSPort := intstr.FromInt(9443)
		expectedNetpolAllowKubeApiserver.Spec.Egress = append(expectedNetpolAllowKubeApiserver.Spec.Egress, networkingv1.NetworkPolicyEgressRule{
			To: []networkingv1.NetworkPolicyPeer{
				{
					// Allow connections from the apiserver pod to the konnectivity-server pod.
					PodSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app": "konnectivity-server",
						},
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

	mockSeedClient.EXPECT().Create(ctx, expectedNetpolAllowKubeApiserver).Times(1)
}

func expectDefaultNetpolAllowToShootAPIServer(ctx context.Context) {
	mockSeedClient.EXPECT().Get(ctx, kutil.Key(defaultSeedNamespace, "allow-to-shoot-apiserver"), gomock.AssignableToTypeOf(&networkingv1.NetworkPolicy{})).Return(apierrors.NewNotFound(schema.GroupResource{}, "foo"))

	tcp := corev1.ProtocolTCP
	port443 := intstr.FromInt(443)
	expectedNetpolAllowToShootApiserver := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "allow-to-shoot-apiserver",
			Namespace: defaultSeedNamespace,
			Annotations: map[string]string{
				"gardener.cloud/description": `|
Allows Egress from pods labeled with 'networking.gardener.cloud/to-shoot-apiserver=allowed'
to talk to Shoot's Kubernetes API Server.
`,
			},
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					"networking.gardener.cloud/to-shoot-apiserver": "allowed",
				},
			},
			Egress: []networkingv1.NetworkPolicyEgressRule{
				{
					To: []networkingv1.NetworkPolicyPeer{
						{
							PodSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"gardener.cloud/role":     "controlplane",
									"garden.sapcloud.io/role": "controlplane",
									"app":                     "kubernetes",
									"role":                    "apiserver",
								},
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
				"Egress",
			},
			Ingress: []networkingv1.NetworkPolicyIngressRule{},
		},
	}
	mockSeedClient.EXPECT().Create(ctx, expectedNetpolAllowToShootApiserver).Times(1)
}
