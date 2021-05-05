// Copyright (c) 201 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package konnectivity_test

import (
	"context"

	resourcesv1alpha1 "github.com/gardener/gardener-resource-manager/pkg/apis/resources/v1alpha1"

	"github.com/gardener/gardener/pkg/operation/botanist/component"
	. "github.com/gardener/gardener/pkg/operation/botanist/component/konnectivity"

	prototypes "github.com/gogo/protobuf/types"
	istioapinetworkingv1beta1 "istio.io/api/networking/v1beta1"
	istiok8snetworkingv1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	k8snetworkingv1 "k8s.io/api/networking/v1"
	policyv1beta1 "k8s.io/api/policy/v1beta1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("NewServer", func() {
	const (
		deployNS = "test-namespace"
		image    = "foo:v1"
	)

	var (
		ctx                context.Context
		c                  client.Client
		serverOptions      *ServerOptions
		sched              component.DeployWaiter
		expectedLabels     map[string]string
		istioIngressLabels map[string]string
		hosts              []string
		codec              serializer.CodecFactory
		secrets            ServerSecrets
		proberSuccess      = func(_ context.Context, _ client.Client, _, _ string) error {
			return nil
		}
	)

	BeforeEach(func() {
		ctx = context.TODO()
		expectedLabels = map[string]string{"app": "konnectivity-server"}
		hosts = []string{"foo", "bar"}
		istioIngressLabels = map[string]string{"foo": "bar"}
		secrets = ServerSecrets{
			Kubeconfig: component.Secret{Name: "kubeconfig", Checksum: "123", Data: map[string][]byte{}},
			Server:     component.Secret{Name: "server-tls", Checksum: "456", Data: map[string][]byte{}},
			ClientCA:   component.Secret{Name: "client-ca", Checksum: "789", Data: map[string][]byte{}},
		}

		s := runtime.NewScheme()
		sb := runtime.NewSchemeBuilder(
			appsv1.AddToScheme,
			corev1.AddToScheme,
			rbacv1.AddToScheme,
			k8snetworkingv1.AddToScheme,
			istiok8snetworkingv1beta1.AddToScheme,
			resourcesv1alpha1.AddToScheme,
		)

		Expect(sb.AddToScheme(s)).To(Succeed())

		codec = serializer.NewCodecFactory(s, serializer.EnableStrict)
		c = fake.NewFakeClientWithScheme(s)

		serverOptions = &ServerOptions{
			Client:             c,
			Namespace:          deployNS,
			Image:              image,
			Replicas:           2,
			Hosts:              hosts,
			IstioIngressLabels: istioIngressLabels,
			Healthy:            proberSuccess,
			Removed:            proberSuccess,
		}
	})

	Context("fails", func() {
		var err error

		It("when server options are nil", func() {
			sched, err = NewServer(nil)
		})

		It("when client is nil", func() {
			serverOptions.Client = nil
			sched, err = NewServer(serverOptions)
		})

		It("when namespace is empty", func() {
			serverOptions.Namespace = ""
			sched, err = NewServer(serverOptions)
		})

		AfterEach(func() {
			Expect(err).To(HaveOccurred())
			Expect(sched).To(BeNil())
		})
	})

	Context("succeeds", func() {
		var managedResourceSecret *corev1.Secret

		BeforeEach(func() {
			managedResourceSecret = &corev1.Secret{}
		})

		JustBeforeEach(func() {
			s, err := NewServer(serverOptions)
			Expect(err).NotTo(HaveOccurred(), "New succeeds")

			sched = s

			s.SetSecrets(secrets)
		})

		Context("Deploy fails", func() {
			JustAfterEach(func() {
				sched, err := NewServer(serverOptions)
				Expect(err).NotTo(HaveOccurred(), "New succeeds")

				sched.SetSecrets(secrets)
				Expect(sched.Deploy(ctx)).ToNot(Succeed(), "deploy fails")
			})

			It("fails when kubeconfig secret has no name", func() {
				secrets.Kubeconfig.Name = ""
			})
			It("fails when kubeconfig secret has no checksum", func() {
				secrets.Kubeconfig.Checksum = ""
			})

			It("fails when serveer secret has no name", func() {
				secrets.Server.Name = ""
			})
			It("fails when serveer secret has no checksum", func() {
				secrets.Server.Checksum = ""
			})

			It("fails when clientCA secret has no name", func() {
				secrets.ClientCA.Name = ""
			})
			It("fails when clientCA secret has no checksum", func() {
				secrets.ClientCA.Checksum = ""
			})

			It("fails when hosts are empty", func() {
				serverOptions.Hosts = nil
			})

			It("fails when image is empty", func() {
				serverOptions.Image = ""
			})

			It("fails when istioIngressLabels are empty", func() {
				serverOptions.IstioIngressLabels = nil
			})
		})

		Context("Deploy", func() {
			JustBeforeEach(func() {
				Expect(sched.Deploy(ctx)).To(Succeed(), "deploy succeeds")

				Expect(c.Get(ctx, types.NamespacedName{
					Name:      "konnectivity-server",
					Namespace: deployNS,
				}, &resourcesv1alpha1.ManagedResource{})).To(Succeed(), "can get managed resource")

				Expect(c.Get(ctx, types.NamespacedName{
					Name:      "managedresource-konnectivity-server",
					Namespace: deployNS,
				}, managedResourceSecret)).To(Succeed(), "can get managed resource's secret")
			})

			It("has correct resource count", func() {
				// increase this count only after adding tests for the new resource.
				Expect(managedResourceSecret.Data).To(HaveLen(10))
			})

			It("serviceaccount is created", func() {
				const key = "serviceaccount__test-namespace__konnectivity-server.yaml"
				actual := &corev1.ServiceAccount{}
				expected := &corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "konnectivity-server",
						Namespace: deployNS,
						Labels:    expectedLabels,
					},
				}

				Expect(managedResourceSecret.Data).To(HaveKey(key))
				_, _, err := codec.UniversalDecoder().Decode(managedResourceSecret.Data[key], nil, actual)
				Expect(err).NotTo(HaveOccurred())

				Expect(actual).To(DeepDerivativeEqual(expected))
			})

			It("role is created", func() {
				const key = "role__test-namespace__konnectivity-server.yaml"
				actual := &rbacv1.Role{}
				expected := &rbacv1.Role{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "konnectivity-server",
						Namespace: deployNS,
						Labels:    expectedLabels,
					},
					Rules: []rbacv1.PolicyRule{{
						Verbs:         []string{"get", "list", "watch"},
						APIGroups:     []string{"apps"},
						Resources:     []string{"deployments"},
						ResourceNames: []string{"konnectivity-server"},
					}},
				}

				Expect(managedResourceSecret.Data).To(HaveKey(key))
				_, _, err := codec.UniversalDecoder().Decode(managedResourceSecret.Data[key], nil, actual)
				Expect(err).NotTo(HaveOccurred())

				Expect(actual).To(DeepDerivativeEqual(expected))
			})

			It("rolebinding is created", func() {
				const key = "rolebinding__test-namespace__konnectivity-server.yaml"
				actual := &rbacv1.RoleBinding{}
				expected := &rbacv1.RoleBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "konnectivity-server",
						Namespace: deployNS,
						Labels:    expectedLabels,
					},
					RoleRef: rbacv1.RoleRef{
						APIGroup: rbacv1.SchemeGroupVersion.Group,
						Kind:     "Role",
						Name:     "konnectivity-server",
					},
					Subjects: []rbacv1.Subject{{
						APIGroup:  corev1.SchemeGroupVersion.Group,
						Kind:      "ServiceAccount",
						Name:      "konnectivity-server",
						Namespace: deployNS,
					}},
				}

				Expect(managedResourceSecret.Data).To(HaveKey(key))
				_, _, err := codec.UniversalDecoder().Decode(managedResourceSecret.Data[key], nil, actual)
				Expect(err).NotTo(HaveOccurred())

				Expect(actual).To(DeepDerivativeEqual(expected))
			})

			It("deployment is created", func() {
				const key = "deployment__test-namespace__konnectivity-server.yaml"
				actual := &appsv1.Deployment{}
				expected := &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "konnectivity-server",
						Namespace: deployNS,
						Labels:    expectedLabels,
					},
					Spec: appsv1.DeploymentSpec{
						Replicas:             pointer.Int32Ptr(2),
						RevisionHistoryLimit: pointer.Int32Ptr(1),
						Selector:             &metav1.LabelSelector{MatchLabels: expectedLabels},
						Strategy: appsv1.DeploymentStrategy{
							Type: appsv1.RollingUpdateDeploymentStrategyType,
							RollingUpdate: &appsv1.RollingUpdateDeployment{
								MaxUnavailable: &intstr.IntOrString{Type: intstr.Int, IntVal: 1},
								MaxSurge:       &intstr.IntOrString{Type: intstr.Int, IntVal: 1},
							},
						},
						Template: corev1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Labels: map[string]string{
									"app":                              "konnectivity-server",
									"networking.gardener.cloud/to-dns": "allowed",
									"networking.gardener.cloud/from-shoot-apiserver": "allowed",
									"networking.gardener.cloud/to-shoot-apiserver":   "allowed",
									"networking.gardener.cloud/to-seed-apiserver":    "allowed",
								},
								Annotations: map[string]string{
									"checksum/secret-kubeconfig": "123",
									"checksum/secret-server-tls": "456",
									"checksum/secret-client-ca":  "789",
								},
							},
							Spec: corev1.PodSpec{
								Affinity: &corev1.Affinity{
									PodAntiAffinity: &corev1.PodAntiAffinity{
										PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{{
											Weight: 100,
											PodAffinityTerm: corev1.PodAffinityTerm{
												TopologyKey:   corev1.LabelHostname,
												LabelSelector: &metav1.LabelSelector{MatchLabels: expectedLabels},
											},
										}},
									},
								},
								ServiceAccountName: "konnectivity-server",
								Containers: []corev1.Container{
									{
										Name:            "konnectivity-server",
										Image:           image,
										ImagePullPolicy: corev1.PullIfNotPresent,
										Args: []string{
											"--namespace=" + deployNS,
											"--deployment-name=konnectivity-server",
											"--jitter=10s",
											"--jitter-factor=5",
											"--v=2",
											"--",
											"/proxy-server",
											"--logtostderr=true",
											"--cluster-cert=/certs/konnectivity-server/konnectivity-server.crt",
											"--cluster-key=/certs/konnectivity-server/konnectivity-server.key",
											"--server-cert=/certs/konnectivity-server/konnectivity-server.crt",
											"--server-key=/certs/konnectivity-server/konnectivity-server.key",
											"--server-ca-cert=/certs/client-ca/ca.crt",
											"--agent-namespace=kube-system",
											"--agent-service-account=konnectivity-agent",
											"--kubeconfig=/auth/kubeconfig",
											"--authentication-audience=system:konnectivity-server",
											"--keepalive-time=1m",
											"--log-file-max-size=0",
											"--delete-existing-uds-file=true",
											"--mode=http-connect",
											"--server-port=9443",
											"--agent-port=8132",
											"--admin-port=8133",
											"--health-port=8134",
											"--v=2",
											"--server-count",
										},
										LivenessProbe: &corev1.Probe{
											Handler: corev1.Handler{
												HTTPGet: &corev1.HTTPGetAction{
													Path:   "/healthz",
													Scheme: corev1.URISchemeHTTP,
													Port:   intstr.FromInt(8134),
												},
											},
										},
										Resources: corev1.ResourceRequirements{
											Requests: corev1.ResourceList{
												corev1.ResourceCPU:    resource.MustParse("30m"),
												corev1.ResourceMemory: resource.MustParse("40Mi"),
											},
											Limits: corev1.ResourceList{
												corev1.ResourceCPU:    resource.MustParse("300m"),
												corev1.ResourceMemory: resource.MustParse("400Mi"),
											},
										},
										Ports: []corev1.ContainerPort{{
											Name:          "server",
											ContainerPort: 9443,
										}, {
											Name:          "agent",
											ContainerPort: 8132,
										}, {
											Name:          "admin",
											ContainerPort: 8133,
										}, {
											Name:          "health",
											ContainerPort: 8134,
										}},
										VolumeMounts: []corev1.VolumeMount{
											{
												Name:      "server-tls",
												MountPath: "/certs/konnectivity-server",
											},
											{
												Name:      "client-ca",
												MountPath: "/certs/client-ca",
											},
											{
												Name:      "kubeconfig",
												MountPath: "/auth",
											},
										},
									},
								},
								Volumes: []corev1.Volume{
									{
										Name: "server-tls",
										VolumeSource: corev1.VolumeSource{
											Secret: &corev1.SecretVolumeSource{
												SecretName: "server-tls",
											},
										},
									},
									{
										Name: "client-ca",
										VolumeSource: corev1.VolumeSource{
											Secret: &corev1.SecretVolumeSource{
												SecretName: "client-ca",
											},
										},
									},
									{
										Name: "kubeconfig",
										VolumeSource: corev1.VolumeSource{
											Secret: &corev1.SecretVolumeSource{
												SecretName: "kubeconfig",
											},
										},
									},
								},
							},
						},
					},
				}

				Expect(managedResourceSecret.Data).To(HaveKey(key))
				_, _, err := codec.UniversalDecoder().Decode(managedResourceSecret.Data[key], nil, actual)
				Expect(err).NotTo(HaveOccurred())

				Expect(actual).To(DeepDerivativeEqual(expected))
			})

			It("service is created", func() {
				const key = "service__test-namespace__konnectivity-server.yaml"
				actual := &corev1.Service{}
				expected := &corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "konnectivity-server",
						Namespace:   deployNS,
						Labels:      expectedLabels,
						Annotations: map[string]string{"networking.istio.io/exportTo": "*"},
					},
					Spec: corev1.ServiceSpec{
						Selector: expectedLabels,
						Type:     corev1.ServiceTypeClusterIP,
						Ports: []corev1.ServicePort{
							{
								Name:     "server",
								Protocol: corev1.ProtocolTCP,
								Port:     9443,
							}, {
								Name:     "agent",
								Protocol: corev1.ProtocolTCP,
								Port:     8132,
							},
						},
					},
				}

				Expect(managedResourceSecret.Data).To(HaveKey(key))
				_, _, err := codec.UniversalDecoder().Decode(managedResourceSecret.Data[key], nil, actual)
				Expect(err).NotTo(HaveOccurred())

				Expect(actual).To(DeepDerivativeEqual(expected))
			})

			It("networkpolicy is created", func() {
				tcpProto := corev1.ProtocolTCP
				const key = "networkpolicy__test-namespace__konnectivity-server.yaml"
				actual := &k8snetworkingv1.NetworkPolicy{}
				expected := &k8snetworkingv1.NetworkPolicy{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "konnectivity-server",
						Namespace: deployNS,
						Labels:    expectedLabels,
					},
					Spec: k8snetworkingv1.NetworkPolicySpec{
						PodSelector: metav1.LabelSelector{
							MatchLabels: expectedLabels,
						},
						PolicyTypes: []k8snetworkingv1.PolicyType{k8snetworkingv1.PolicyTypeIngress},
						Egress:      []k8snetworkingv1.NetworkPolicyEgressRule{},
						Ingress: []k8snetworkingv1.NetworkPolicyIngressRule{{
							From: []k8snetworkingv1.NetworkPolicyPeer{{
								PodSelector: &metav1.LabelSelector{
									MatchLabels: map[string]string{"app": "istio-ingressgateway"},
								},
								// match all namespaces since there is no guarantee about the labels of the istio-ingress namespace
								NamespaceSelector: &metav1.LabelSelector{},
							}},
							Ports: []k8snetworkingv1.NetworkPolicyPort{{
								Protocol: &tcpProto,
								Port:     &intstr.IntOrString{Type: intstr.Int, IntVal: 8132},
							}},
						}},
					},
				}

				Expect(managedResourceSecret.Data).To(HaveKey(key))
				_, _, err := codec.UniversalDecoder().Decode(managedResourceSecret.Data[key], nil, actual)
				Expect(err).NotTo(HaveOccurred())

				Expect(actual).To(DeepDerivativeEqual(expected))
			})

			It("poddisruptionbudget is created", func() {
				const key = "poddisruptionbudget__test-namespace__konnectivity-server.yaml"
				actual := &policyv1beta1.PodDisruptionBudget{}
				expected := &policyv1beta1.PodDisruptionBudget{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "konnectivity-server",
						Namespace: deployNS,
						Labels:    expectedLabels,
					},
					Spec: policyv1beta1.PodDisruptionBudgetSpec{
						MinAvailable: &intstr.IntOrString{
							Type:   intstr.Int,
							IntVal: int32(1),
						},
						MaxUnavailable: nil,
						Selector: &metav1.LabelSelector{
							MatchLabels: expectedLabels,
						},
					},
				}

				Expect(managedResourceSecret.Data).To(HaveKey(key))
				_, _, err := codec.UniversalDecoder().Decode(managedResourceSecret.Data[key], nil, actual)
				Expect(err).NotTo(HaveOccurred())

				Expect(actual).To(DeepDerivativeEqual(expected))
			})

			It("gateway is created", func() {
				const key = "gateway__test-namespace__konnectivity-server.yaml"
				actual := &istiok8snetworkingv1beta1.Gateway{}
				expected := &istiok8snetworkingv1beta1.Gateway{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "konnectivity-server",
						Namespace: deployNS,
						Labels:    expectedLabels,
					},
					Spec: istioapinetworkingv1beta1.Gateway{
						Selector: istioIngressLabels,
						Servers: []*istioapinetworkingv1beta1.Server{{
							Hosts: hosts,
							Port: &istioapinetworkingv1beta1.Port{
								Number:   8132,
								Protocol: "TLS",
								Name:     "tls-tunnel",
							},
							Tls: &istioapinetworkingv1beta1.ServerTLSSettings{
								Mode: istioapinetworkingv1beta1.ServerTLSSettings_PASSTHROUGH,
							},
						}},
					},
				}

				Expect(managedResourceSecret.Data).To(HaveKey(key))
				_, _, err := codec.UniversalDecoder().Decode(managedResourceSecret.Data[key], nil, actual)
				Expect(err).NotTo(HaveOccurred())

				Expect(actual).To(DeepDerivativeEqual(expected))
			})

			It("virtualservice is created", func() {
				const key = "virtualservice__test-namespace__konnectivity-server.yaml"
				actual := &istiok8snetworkingv1beta1.VirtualService{}
				expected := &istiok8snetworkingv1beta1.VirtualService{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "konnectivity-server",
						Namespace: deployNS,
						Labels:    expectedLabels,
					},
					Spec: istioapinetworkingv1beta1.VirtualService{
						Hosts:    hosts,
						Gateways: []string{"konnectivity-server"},
						ExportTo: []string{"*"},
						Tls: []*istioapinetworkingv1beta1.TLSRoute{{
							Match: []*istioapinetworkingv1beta1.TLSMatchAttributes{{
								SniHosts: hosts,
								Port:     8132,
							}},
							Route: []*istioapinetworkingv1beta1.RouteDestination{{
								Destination: &istioapinetworkingv1beta1.Destination{
									Host: "konnectivity-server.test-namespace.svc.cluster.local",
									Port: &istioapinetworkingv1beta1.PortSelector{
										Number: 8132,
									},
								},
							}},
						}},
					},
				}

				Expect(managedResourceSecret.Data).To(HaveKey(key))
				_, _, err := codec.UniversalDecoder().Decode(managedResourceSecret.Data[key], nil, actual)
				Expect(err).NotTo(HaveOccurred())

				Expect(actual).To(DeepDerivativeEqual(expected))
			})

			It("destinationrule is created", func() {
				const key = "destinationrule__test-namespace__konnectivity-server.yaml"
				actual := &istiok8snetworkingv1beta1.DestinationRule{}
				expected := &istiok8snetworkingv1beta1.DestinationRule{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "konnectivity-server",
						Namespace: deployNS,
						Labels:    expectedLabels,
					},
					Spec: istioapinetworkingv1beta1.DestinationRule{
						ExportTo: []string{"*"},
						Host:     "konnectivity-server.test-namespace.svc.cluster.local",
						TrafficPolicy: &istioapinetworkingv1beta1.TrafficPolicy{
							ConnectionPool: &istioapinetworkingv1beta1.ConnectionPoolSettings{
								Tcp: &istioapinetworkingv1beta1.ConnectionPoolSettings_TCPSettings{
									MaxConnections: 5000,
									TcpKeepalive: &istioapinetworkingv1beta1.ConnectionPoolSettings_TCPSettings_TcpKeepalive{
										Time:     &prototypes.Duration{Seconds: 7200},
										Interval: &prototypes.Duration{Seconds: 75},
									},
								},
							},
							Tls: &istioapinetworkingv1beta1.ClientTLSSettings{
								Mode: istioapinetworkingv1beta1.ClientTLSSettings_DISABLE,
							},
						},
					},
				}

				Expect(managedResourceSecret.Data).To(HaveKey(key))
				_, _, err := codec.UniversalDecoder().Decode(managedResourceSecret.Data[key], nil, actual)
				Expect(err).NotTo(HaveOccurred())

				Expect(actual).To(DeepDerivativeEqual(expected))
			})
		})

		Context("Destroy", func() {
			JustBeforeEach(func() {
				Expect(sched.Deploy(ctx)).To(Succeed(), "deploy succeeds")
				Expect(sched.Destroy(ctx)).To(Succeed(), "destroy succeeds")
			})

			It("removes managed resource", func() {
				Expect(c.Get(ctx, types.NamespacedName{
					Name:      "konnectivity-server",
					Namespace: deployNS,
				}, &resourcesv1alpha1.ManagedResource{})).To(BeNotFoundError(), "managed resource is not found")
			})
		})

		Context("Wait", func() {
			It("succeeds", func() {
				Expect(sched.Wait(ctx)).To(Succeed())
			})
		})

		Context("WaitCleanup", func() {
			It("succeeds", func() {
				Expect(sched.WaitCleanup(ctx)).To(Succeed())
			})
		})
	})
})
