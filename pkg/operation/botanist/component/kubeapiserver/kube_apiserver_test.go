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

package kubeapiserver_test

import (
	"context"
	"strconv"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	fakekubernetes "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	. "github.com/gardener/gardener/pkg/operation/botanist/component/kubeapiserver"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"

	"github.com/Masterminds/semver"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	hvpav1alpha1 "github.com/gardener/hvpa-controller/api/v1alpha1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	autoscalingv2beta1 "k8s.io/api/autoscaling/v2beta1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	policyv1beta1 "k8s.io/api/policy/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
	autoscalingv1beta2 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1beta2"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("KubeAPIServer", func() {
	var (
		ctx = context.TODO()

		namespace          = "some-namespace"
		vpaUpdateMode      = autoscalingv1beta2.UpdateModeOff
		containerPolicyOff = autoscalingv1beta2.ContainerScalingModeOff

		kubernetesInterface kubernetes.Interface
		c                   client.Client
		kapi                Interface
		version             = semver.MustParse("1.22.1")

		secretNameBasicAuthentication        = "BasicAuthentication-secret"
		secretChecksumBasicAuthentication    = "12345"
		secretNameCA                         = "CA-secret"
		secretChecksumCA                     = "12345"
		secretNameCAEtcd                     = "CAEtcd-secret"
		secretChecksumCAEtcd                 = "12345"
		secretNameCAFrontProxy               = "CAFrontProxy-secret"
		secretChecksumCAFrontProxy           = "12345"
		secretNameEtcd                       = "Etcd-secret"
		secretChecksumEtcd                   = "12345"
		secretNameHTTPProxy                  = "HttpProxy-secret"
		secretChecksumHTTPProxy              = "12345"
		secretNameEtcdEncryptionConfig       = "EtcdEncryptionConfig-secret"
		secretChecksumEtcdEncryptionConfig   = "12345"
		secretNameKubeAggregator             = "KubeAggregator-secret"
		secretChecksumKubeAggregator         = "12345"
		secretNameKubeAPIServerToKubelet     = "KubeAPIServerToKubelet-secret"
		secretChecksumKubeAPIServerToKubelet = "12345"
		secretNameServer                     = "Server-secret"
		secretChecksumServer                 = "12345"
		secretNameServiceAccountKey          = "ServiceAccountKey-secret"
		secretChecksumServiceAccountKey      = "12345"
		secretNameStaticToken                = "StaticToken-secret"
		secretChecksumStaticToken            = "12345"
		secretNameVPNSeed                    = "VPNSeed-secret"
		secretChecksumVPNSeed                = "12345"
		secretNameVPNSeedTLSAuth             = "VPNSeedTLSAuth-secret"
		secretChecksumVPNSeedTLSAuth         = "12345"
		secretNameVPNSeedServerTLSAuth       = "VPNSeedServerTLSAuth-secret"
		secretChecksumVPNSeedServerTLSAuth   = "12345"
		secrets                              Secrets

		deployment                           *appsv1.Deployment
		horizontalPodAutoscaler              *autoscalingv2beta1.HorizontalPodAutoscaler
		verticalPodAutoscaler                *autoscalingv1beta2.VerticalPodAutoscaler
		hvpa                                 *hvpav1alpha1.Hvpa
		podDisruptionBudget                  *policyv1beta1.PodDisruptionBudget
		networkPolicyAllowFromShootAPIServer *networkingv1.NetworkPolicy
		networkPolicyAllowToShootAPIServer   *networkingv1.NetworkPolicy
		networkPolicyAllowKubeAPIServer      *networkingv1.NetworkPolicy
		secretOIDCCABundle                   *corev1.Secret
		secretServiceAccountSigningKey       *corev1.Secret
		configMapAdmission                   *corev1.ConfigMap
		configMapAuditPolicy                 *corev1.ConfigMap
		configMapEgressSelector              *corev1.ConfigMap
		managedResource                      *resourcesv1alpha1.ManagedResource
		managedResourceSecret                *corev1.Secret
	)

	BeforeEach(func() {
		c = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		kubernetesInterface = fakekubernetes.NewClientSetBuilder().WithAPIReader(c).WithClient(c).Build()
		kapi = New(kubernetesInterface, namespace, Values{Version: version})

		secrets = Secrets{
			BasicAuthentication:    &component.Secret{Name: secretNameBasicAuthentication, Checksum: secretChecksumBasicAuthentication},
			CA:                     component.Secret{Name: secretNameCA, Checksum: secretChecksumCA},
			CAEtcd:                 component.Secret{Name: secretNameCAEtcd, Checksum: secretChecksumCAEtcd},
			CAFrontProxy:           component.Secret{Name: secretNameCAFrontProxy, Checksum: secretChecksumCAFrontProxy},
			Etcd:                   component.Secret{Name: secretNameEtcd, Checksum: secretChecksumEtcd},
			HTTPProxy:              &component.Secret{Name: secretNameHTTPProxy, Checksum: secretChecksumHTTPProxy},
			EtcdEncryptionConfig:   component.Secret{Name: secretNameEtcdEncryptionConfig, Checksum: secretChecksumEtcdEncryptionConfig},
			KubeAggregator:         component.Secret{Name: secretNameKubeAggregator, Checksum: secretChecksumKubeAggregator},
			KubeAPIServerToKubelet: component.Secret{Name: secretNameKubeAPIServerToKubelet, Checksum: secretChecksumKubeAPIServerToKubelet},
			Server:                 component.Secret{Name: secretNameServer, Checksum: secretChecksumServer},
			ServiceAccountKey:      component.Secret{Name: secretNameServiceAccountKey, Checksum: secretChecksumServiceAccountKey},
			StaticToken:            component.Secret{Name: secretNameStaticToken, Checksum: secretChecksumStaticToken},
			VPNSeed:                &component.Secret{Name: secretNameVPNSeed, Checksum: secretChecksumVPNSeed},
			VPNSeedTLSAuth:         &component.Secret{Name: secretNameVPNSeedTLSAuth, Checksum: secretChecksumVPNSeedTLSAuth},
			VPNSeedServerTLSAuth:   &component.Secret{Name: secretNameVPNSeedServerTLSAuth, Checksum: secretChecksumVPNSeedServerTLSAuth},
		}

		deployment = &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "kube-apiserver",
				Namespace: namespace,
			},
		}
		horizontalPodAutoscaler = &autoscalingv2beta1.HorizontalPodAutoscaler{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "kube-apiserver",
				Namespace: namespace,
			},
		}
		verticalPodAutoscaler = &autoscalingv1beta2.VerticalPodAutoscaler{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "kube-apiserver-vpa",
				Namespace: namespace,
			},
		}
		hvpa = &hvpav1alpha1.Hvpa{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "kube-apiserver",
				Namespace: namespace,
			},
		}
		podDisruptionBudget = &policyv1beta1.PodDisruptionBudget{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "kube-apiserver",
				Namespace: namespace,
			},
		}
		networkPolicyAllowFromShootAPIServer = &networkingv1.NetworkPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "allow-from-shoot-apiserver",
				Namespace: namespace,
			},
		}
		networkPolicyAllowToShootAPIServer = &networkingv1.NetworkPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "allow-to-shoot-apiserver",
				Namespace: namespace,
			},
		}
		networkPolicyAllowKubeAPIServer = &networkingv1.NetworkPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "allow-kube-apiserver",
				Namespace: namespace,
			},
		}
		managedResource = &resourcesv1alpha1.ManagedResource{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "shoot-core-kube-apiserver",
				Namespace: namespace,
			},
		}
		managedResourceSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "managedresource-shoot-core-kube-apiserver",
				Namespace: namespace,
			},
		}
	})

	Describe("#Deploy", func() {
		Context("missing secret information", func() {
			DescribeTable("should return an error because secret information is not provided",
				func(name string, mutateSecrets func(*Secrets), values Values) {
					mutateSecrets(&secrets)
					values.Version = version

					kapi = New(kubernetesInterface, namespace, values)
					kapi.SetSecrets(secrets)

					Expect(kapi.Deploy(ctx)).To(MatchError(ContainSubstring("missing information for required secret " + name)))
				},

				Entry("BasicAuthentication enabled but missing",
					"BasicAuthentication", func(s *Secrets) { s.BasicAuthentication = nil }, Values{BasicAuthenticationEnabled: true},
				),
				Entry("CA missing",
					"CA", func(s *Secrets) { s.CA.Name = "" }, Values{},
				),
				Entry("CAEtcd missing",
					"CAEtcd", func(s *Secrets) { s.CAEtcd.Name = "" }, Values{},
				),
				Entry("CAFrontProxy missing",
					"CAFrontProxy", func(s *Secrets) { s.CAFrontProxy.Name = "" }, Values{},
				),
				Entry("Etcd missing",
					"Etcd", func(s *Secrets) { s.Etcd.Name = "" }, Values{},
				),
				Entry("EtcdEncryptionConfig missing",
					"EtcdEncryptionConfig", func(s *Secrets) { s.EtcdEncryptionConfig.Name = "" }, Values{},
				),
				Entry("KubeAggregator missing",
					"KubeAggregator", func(s *Secrets) { s.KubeAggregator.Name = "" }, Values{},
				),
				Entry("KubeAPIServerToKubelet missing",
					"KubeAPIServerToKubelet", func(s *Secrets) { s.KubeAPIServerToKubelet.Name = "" }, Values{},
				),
				Entry("Server missing",
					"Server", func(s *Secrets) { s.Server.Name = "" }, Values{},
				),
				Entry("ServiceAccountKey missing",
					"ServiceAccountKey", func(s *Secrets) { s.ServiceAccountKey.Name = "" }, Values{},
				),
				Entry("StaticToken missing",
					"StaticToken", func(s *Secrets) { s.StaticToken.Name = "" }, Values{},
				),
				Entry("ReversedVPN disabled but VPNSeed missing",
					"VPNSeed", func(s *Secrets) { s.VPNSeed = nil }, Values{VPN: VPNConfig{ReversedVPNEnabled: false}},
				),
				Entry("ReversedVPN disabled but VPNSeedTLSAuth missing",
					"VPNSeedTLSAuth", func(s *Secrets) { s.VPNSeedTLSAuth = nil }, Values{VPN: VPNConfig{ReversedVPNEnabled: false}},
				),
				Entry("ReversedVPN enabled but VPNSeedServerTLSAuth missing",
					"VPNSeedServerTLSAuth", func(s *Secrets) { s.VPNSeedServerTLSAuth = nil }, Values{VPN: VPNConfig{ReversedVPNEnabled: true}},
				),
			)
		})

		Context("secret information available", func() {
			BeforeEach(func() {
				kapi.SetSecrets(secrets)
			})

			Describe("HorizontalPodAutoscaler", func() {
				DescribeTable("should delete the HPA resource",
					func(autoscalingConfig AutoscalingConfig) {
						kapi = New(kubernetesInterface, namespace, Values{Autoscaling: autoscalingConfig, Version: version})
						kapi.SetSecrets(secrets)

						Expect(c.Create(ctx, horizontalPodAutoscaler)).To(Succeed())
						Expect(c.Get(ctx, client.ObjectKeyFromObject(horizontalPodAutoscaler), horizontalPodAutoscaler)).To(Succeed())
						Expect(kapi.Deploy(ctx)).To(Succeed())
						Expect(c.Get(ctx, client.ObjectKeyFromObject(horizontalPodAutoscaler), horizontalPodAutoscaler)).To(MatchError(apierrors.NewNotFound(schema.GroupResource{Group: autoscalingv2beta1.SchemeGroupVersion.Group, Resource: "horizontalpodautoscalers"}, horizontalPodAutoscaler.Name)))
					},

					Entry("HVPA is enabled", AutoscalingConfig{HVPAEnabled: true}),
					Entry("replicas is nil", AutoscalingConfig{HVPAEnabled: false, Replicas: nil}),
					Entry("replicas is 0", AutoscalingConfig{HVPAEnabled: false, Replicas: pointer.Int32(0)}),
				)

				It("should successfully deploy the HPA resource", func() {
					autoscalingConfig := AutoscalingConfig{
						HVPAEnabled: false,
						Replicas:    pointer.Int32(2),
						MinReplicas: 4,
						MaxReplicas: 6,
					}
					kapi = New(kubernetesInterface, namespace, Values{Autoscaling: autoscalingConfig, Version: version})
					kapi.SetSecrets(secrets)

					Expect(c.Get(ctx, client.ObjectKeyFromObject(horizontalPodAutoscaler), horizontalPodAutoscaler)).To(MatchError(apierrors.NewNotFound(schema.GroupResource{Group: autoscalingv2beta1.SchemeGroupVersion.Group, Resource: "horizontalpodautoscalers"}, horizontalPodAutoscaler.Name)))
					Expect(kapi.Deploy(ctx)).To(Succeed())
					Expect(c.Get(ctx, client.ObjectKeyFromObject(horizontalPodAutoscaler), horizontalPodAutoscaler)).To(Succeed())
					Expect(horizontalPodAutoscaler).To(DeepEqual(&autoscalingv2beta1.HorizontalPodAutoscaler{
						TypeMeta: metav1.TypeMeta{
							APIVersion: autoscalingv2beta1.SchemeGroupVersion.String(),
							Kind:       "HorizontalPodAutoscaler",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name:            horizontalPodAutoscaler.Name,
							Namespace:       horizontalPodAutoscaler.Namespace,
							ResourceVersion: "1",
						},
						Spec: autoscalingv2beta1.HorizontalPodAutoscalerSpec{
							MinReplicas: &autoscalingConfig.MinReplicas,
							MaxReplicas: autoscalingConfig.MaxReplicas,
							ScaleTargetRef: autoscalingv2beta1.CrossVersionObjectReference{
								APIVersion: "apps/v1",
								Kind:       "Deployment",
								Name:       "kube-apiserver",
							},
							Metrics: []autoscalingv2beta1.MetricSpec{
								{
									Type: "Resource",
									Resource: &autoscalingv2beta1.ResourceMetricSource{
										Name:                     "cpu",
										TargetAverageUtilization: pointer.Int32(80),
									},
								},
								{
									Type: "Resource",
									Resource: &autoscalingv2beta1.ResourceMetricSource{
										Name:                     "memory",
										TargetAverageUtilization: pointer.Int32(80),
									},
								},
							},
						},
					}))
				})
			})

			Describe("VerticalPodAutoscaler", func() {
				It("should delete the VPA resource", func() {
					kapi = New(kubernetesInterface, namespace, Values{Autoscaling: AutoscalingConfig{HVPAEnabled: true}, Version: version})
					kapi.SetSecrets(secrets)

					Expect(c.Create(ctx, verticalPodAutoscaler)).To(Succeed())
					Expect(c.Get(ctx, client.ObjectKeyFromObject(verticalPodAutoscaler), verticalPodAutoscaler)).To(Succeed())
					Expect(kapi.Deploy(ctx)).To(Succeed())
					Expect(c.Get(ctx, client.ObjectKeyFromObject(verticalPodAutoscaler), verticalPodAutoscaler)).To(MatchError(apierrors.NewNotFound(schema.GroupResource{Group: autoscalingv1beta2.SchemeGroupVersion.Group, Resource: "verticalpodautoscalers"}, verticalPodAutoscaler.Name)))
				})

				It("should successfully deploy the VPA resource", func() {
					autoscalingConfig := AutoscalingConfig{HVPAEnabled: false}
					kapi = New(kubernetesInterface, namespace, Values{Autoscaling: autoscalingConfig, Version: version})
					kapi.SetSecrets(secrets)

					Expect(c.Get(ctx, client.ObjectKeyFromObject(verticalPodAutoscaler), verticalPodAutoscaler)).To(MatchError(apierrors.NewNotFound(schema.GroupResource{Group: autoscalingv1beta2.SchemeGroupVersion.Group, Resource: "verticalpodautoscalers"}, verticalPodAutoscaler.Name)))
					Expect(kapi.Deploy(ctx)).To(Succeed())
					Expect(c.Get(ctx, client.ObjectKeyFromObject(verticalPodAutoscaler), verticalPodAutoscaler)).To(Succeed())
					Expect(verticalPodAutoscaler).To(DeepEqual(&autoscalingv1beta2.VerticalPodAutoscaler{
						TypeMeta: metav1.TypeMeta{
							APIVersion: autoscalingv1beta2.SchemeGroupVersion.String(),
							Kind:       "VerticalPodAutoscaler",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name:            verticalPodAutoscaler.Name,
							Namespace:       verticalPodAutoscaler.Namespace,
							ResourceVersion: "1",
						},
						Spec: autoscalingv1beta2.VerticalPodAutoscalerSpec{
							TargetRef: &autoscalingv1.CrossVersionObjectReference{
								APIVersion: "apps/v1",
								Kind:       "Deployment",
								Name:       "kube-apiserver",
							},
							UpdatePolicy: &autoscalingv1beta2.PodUpdatePolicy{
								UpdateMode: &vpaUpdateMode,
							},
						},
					}))
				})
			})

			Describe("HVPA", func() {
				DescribeTable("should delete the HVPA resource",
					func(autoscalingConfig AutoscalingConfig) {
						kapi = New(kubernetesInterface, namespace, Values{Autoscaling: autoscalingConfig, Version: version})
						kapi.SetSecrets(secrets)

						Expect(c.Create(ctx, hvpa)).To(Succeed())
						Expect(c.Get(ctx, client.ObjectKeyFromObject(hvpa), hvpa)).To(Succeed())
						Expect(kapi.Deploy(ctx)).To(Succeed())
						Expect(c.Get(ctx, client.ObjectKeyFromObject(hvpa), hvpa)).To(MatchError(apierrors.NewNotFound(schema.GroupResource{Group: hvpav1alpha1.SchemeGroupVersionHvpa.Group, Resource: "hvpas"}, hvpa.Name)))
					},

					Entry("HVPA disabled", AutoscalingConfig{HVPAEnabled: false}),
					Entry("HVPA enabled but replicas nil", AutoscalingConfig{HVPAEnabled: true}),
					Entry("HVPA enabled but replicas zero", AutoscalingConfig{HVPAEnabled: true, Replicas: pointer.Int32(0)}),
				)

				var (
					defaultExpectedScaleDownUpdateMode = "Auto"
					defaultExpectedHPAMetrics          = []autoscalingv2beta1.MetricSpec{
						{
							Type: "Resource",
							Resource: &autoscalingv2beta1.ResourceMetricSource{
								Name:                     "cpu",
								TargetAverageUtilization: pointer.Int32(80),
							},
						},
					}
					defaultExpectedVPAContainerResourcePolicies = []autoscalingv1beta2.ContainerResourcePolicy{
						{
							ContainerName: "kube-apiserver",
							MinAllowed: corev1.ResourceList{
								"cpu":    resource.MustParse("300m"),
								"memory": resource.MustParse("400M"),
							},
							MaxAllowed: corev1.ResourceList{
								"cpu":    resource.MustParse("8"),
								"memory": resource.MustParse("25G"),
							},
						},
						{
							ContainerName: "vpn-seed",
							Mode:          &containerPolicyOff,
						},
					}
					defaultExpectedWeightBasedScalingIntervals = []hvpav1alpha1.WeightBasedScalingInterval{
						{
							VpaWeight:         100,
							StartReplicaCount: 5,
							LastReplicaCount:  5,
						},
					}
				)

				DescribeTable("should successfully deploy the HVPA resource",
					func(
						autoscalingConfig AutoscalingConfig,
						sniConfig SNIConfig,
						expectedScaleDownUpdateMode string,
						expectedHPAMetrics []autoscalingv2beta1.MetricSpec,
						expectedVPAContainerResourcePolicies []autoscalingv1beta2.ContainerResourcePolicy,
						expectedWeightBasedScalingIntervals []hvpav1alpha1.WeightBasedScalingInterval,
					) {
						kapi = New(kubernetesInterface, namespace, Values{
							Autoscaling: autoscalingConfig,
							SNI:         sniConfig,
							Version:     version,
						})
						kapi.SetSecrets(secrets)

						Expect(c.Get(ctx, client.ObjectKeyFromObject(hvpa), hvpa)).To(MatchError(apierrors.NewNotFound(schema.GroupResource{Group: hvpav1alpha1.SchemeGroupVersionHvpa.Group, Resource: "hvpas"}, hvpa.Name)))
						Expect(kapi.Deploy(ctx)).To(Succeed())
						Expect(c.Get(ctx, client.ObjectKeyFromObject(hvpa), hvpa)).To(Succeed())
						Expect(hvpa).To(DeepEqual(&hvpav1alpha1.Hvpa{
							TypeMeta: metav1.TypeMeta{
								APIVersion: hvpav1alpha1.SchemeGroupVersionHvpa.String(),
								Kind:       "Hvpa",
							},
							ObjectMeta: metav1.ObjectMeta{
								Name:            hvpa.Name,
								Namespace:       hvpa.Namespace,
								ResourceVersion: "1",
							},
							Spec: hvpav1alpha1.HvpaSpec{
								Replicas: pointer.Int32(1),
								Hpa: hvpav1alpha1.HpaSpec{
									Selector: &metav1.LabelSelector{
										MatchLabels: map[string]string{"role": "apiserver-hpa"},
									},
									Deploy: true,
									ScaleUp: hvpav1alpha1.ScaleType{
										UpdatePolicy: hvpav1alpha1.UpdatePolicy{
											UpdateMode: pointer.StringPtr("Auto"),
										},
									},
									ScaleDown: hvpav1alpha1.ScaleType{
										UpdatePolicy: hvpav1alpha1.UpdatePolicy{
											UpdateMode: pointer.StringPtr("Auto"),
										},
									},
									Template: hvpav1alpha1.HpaTemplate{
										ObjectMeta: metav1.ObjectMeta{
											Labels: map[string]string{"role": "apiserver-hpa"},
										},
										Spec: hvpav1alpha1.HpaTemplateSpec{
											MinReplicas: &autoscalingConfig.MinReplicas,
											MaxReplicas: autoscalingConfig.MaxReplicas,
											Metrics:     expectedHPAMetrics,
										},
									},
								},
								Vpa: hvpav1alpha1.VpaSpec{
									Selector: &metav1.LabelSelector{
										MatchLabels: map[string]string{"role": "apiserver-vpa"},
									},
									Deploy: true,
									ScaleUp: hvpav1alpha1.ScaleType{
										UpdatePolicy: hvpav1alpha1.UpdatePolicy{
											UpdateMode: pointer.StringPtr("Auto"),
										},
										StabilizationDuration: pointer.StringPtr("3m"),
										MinChange: hvpav1alpha1.ScaleParams{
											CPU: hvpav1alpha1.ChangeParams{
												Value:      pointer.StringPtr("300m"),
												Percentage: pointer.Int32Ptr(80),
											},
											Memory: hvpav1alpha1.ChangeParams{
												Value:      pointer.StringPtr("200M"),
												Percentage: pointer.Int32Ptr(80),
											},
										},
									},
									ScaleDown: hvpav1alpha1.ScaleType{
										UpdatePolicy: hvpav1alpha1.UpdatePolicy{
											UpdateMode: &expectedScaleDownUpdateMode,
										},
										StabilizationDuration: pointer.StringPtr("15m"),
										MinChange: hvpav1alpha1.ScaleParams{
											CPU: hvpav1alpha1.ChangeParams{
												Value:      pointer.StringPtr("300m"),
												Percentage: pointer.Int32Ptr(80),
											},
											Memory: hvpav1alpha1.ChangeParams{
												Value:      pointer.StringPtr("200M"),
												Percentage: pointer.Int32Ptr(80),
											},
										},
									},
									LimitsRequestsGapScaleParams: hvpav1alpha1.ScaleParams{
										CPU: hvpav1alpha1.ChangeParams{
											Value:      pointer.StringPtr("1"),
											Percentage: pointer.Int32Ptr(70),
										},
										Memory: hvpav1alpha1.ChangeParams{
											Value:      pointer.StringPtr("1G"),
											Percentage: pointer.Int32Ptr(70),
										},
									},
									Template: hvpav1alpha1.VpaTemplate{
										ObjectMeta: metav1.ObjectMeta{
											Labels: map[string]string{"role": "apiserver-vpa"},
										},
										Spec: hvpav1alpha1.VpaTemplateSpec{
											ResourcePolicy: &autoscalingv1beta2.PodResourcePolicy{
												ContainerPolicies: expectedVPAContainerResourcePolicies,
											},
										},
									},
								},
								WeightBasedScalingIntervals: expectedWeightBasedScalingIntervals,
								TargetRef: &autoscalingv2beta1.CrossVersionObjectReference{
									APIVersion: "apps/v1",
									Kind:       "Deployment",
									Name:       "kube-apiserver",
								},
							},
						}))
					},

					Entry("default behaviour",
						AutoscalingConfig{
							HVPAEnabled: true,
							Replicas:    pointer.Int32(2),
							MinReplicas: 5,
							MaxReplicas: 5,
						},
						SNIConfig{},
						defaultExpectedScaleDownUpdateMode,
						defaultExpectedHPAMetrics,
						defaultExpectedVPAContainerResourcePolicies,
						defaultExpectedWeightBasedScalingIntervals,
					),
					Entry("UseMemoryMetricForHvpaHPA is true",
						AutoscalingConfig{
							HVPAEnabled:               true,
							Replicas:                  pointer.Int32(2),
							UseMemoryMetricForHvpaHPA: true,
							MinReplicas:               5,
							MaxReplicas:               5,
						},
						SNIConfig{},
						defaultExpectedScaleDownUpdateMode,
						[]autoscalingv2beta1.MetricSpec{
							{
								Type: "Resource",
								Resource: &autoscalingv2beta1.ResourceMetricSource{
									Name:                     "cpu",
									TargetAverageUtilization: pointer.Int32(80),
								},
							},
							{
								Type: "Resource",
								Resource: &autoscalingv2beta1.ResourceMetricSource{
									Name:                     "memory",
									TargetAverageUtilization: pointer.Int32(80),
								},
							},
						},
						defaultExpectedVPAContainerResourcePolicies,
						defaultExpectedWeightBasedScalingIntervals,
					),
					Entry("scale down is disabled",
						AutoscalingConfig{
							HVPAEnabled:              true,
							Replicas:                 pointer.Int32(2),
							MinReplicas:              5,
							MaxReplicas:              5,
							ScaleDownDisabledForHvpa: true,
						},
						SNIConfig{},
						"Off",
						defaultExpectedHPAMetrics,
						defaultExpectedVPAContainerResourcePolicies,
						defaultExpectedWeightBasedScalingIntervals,
					),
					Entry("SNI pod mutator is enabled",
						AutoscalingConfig{
							HVPAEnabled: true,
							Replicas:    pointer.Int32(2),
							MinReplicas: 5,
							MaxReplicas: 5,
						},
						SNIConfig{
							PodMutatorEnabled: true,
						},
						defaultExpectedScaleDownUpdateMode,
						defaultExpectedHPAMetrics,
						[]autoscalingv1beta2.ContainerResourcePolicy{
							{
								ContainerName: "kube-apiserver",
								MinAllowed: corev1.ResourceList{
									"cpu":    resource.MustParse("300m"),
									"memory": resource.MustParse("400M"),
								},
								MaxAllowed: corev1.ResourceList{
									"cpu":    resource.MustParse("8"),
									"memory": resource.MustParse("25G"),
								},
							},
							{
								ContainerName: "vpn-seed",
								Mode:          &containerPolicyOff,
							},
							{
								ContainerName: "apiserver-proxy-pod-mutator",
								Mode:          &containerPolicyOff,
							},
						},
						defaultExpectedWeightBasedScalingIntervals,
					),
					Entry("max replicas > min replicas",
						AutoscalingConfig{
							HVPAEnabled: true,
							Replicas:    pointer.Int32(2),
							MinReplicas: 3,
							MaxReplicas: 5,
						},
						SNIConfig{},
						defaultExpectedScaleDownUpdateMode,
						defaultExpectedHPAMetrics,
						defaultExpectedVPAContainerResourcePolicies,
						[]hvpav1alpha1.WeightBasedScalingInterval{
							{
								VpaWeight:         100,
								StartReplicaCount: 5,
								LastReplicaCount:  5,
							},
							{
								VpaWeight:         0,
								StartReplicaCount: 3,
								LastReplicaCount:  4,
							},
						},
					),
				)
			})

			Describe("PodDisruptionBudget", func() {
				It("should successfully deploy the PDB resource", func() {
					Expect(c.Get(ctx, client.ObjectKeyFromObject(podDisruptionBudget), podDisruptionBudget)).To(MatchError(apierrors.NewNotFound(schema.GroupResource{Group: policyv1beta1.SchemeGroupVersion.Group, Resource: "poddisruptionbudgets"}, podDisruptionBudget.Name)))
					Expect(kapi.Deploy(ctx)).To(Succeed())
					Expect(c.Get(ctx, client.ObjectKeyFromObject(podDisruptionBudget), podDisruptionBudget)).To(Succeed())
					Expect(podDisruptionBudget).To(DeepEqual(&policyv1beta1.PodDisruptionBudget{
						TypeMeta: metav1.TypeMeta{
							APIVersion: policyv1beta1.SchemeGroupVersion.String(),
							Kind:       "PodDisruptionBudget",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name:            podDisruptionBudget.Name,
							Namespace:       podDisruptionBudget.Namespace,
							ResourceVersion: "1",
							Labels: map[string]string{
								"app":  "kubernetes",
								"role": "apiserver",
							},
						},
						Spec: policyv1beta1.PodDisruptionBudgetSpec{
							MaxUnavailable: intOrStrPtr(intstr.FromInt(1)),
							Selector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"app":  "kubernetes",
									"role": "apiserver",
								},
							},
						},
					}))
				})
			})

			Describe("NetworkPolicy", func() {
				It("should successfully deploy the allow-from-shoot-apiserver NetworkPolicy resource", func() {
					Expect(c.Get(ctx, client.ObjectKeyFromObject(networkPolicyAllowFromShootAPIServer), networkPolicyAllowFromShootAPIServer)).To(MatchError(apierrors.NewNotFound(schema.GroupResource{Group: networkingv1.SchemeGroupVersion.Group, Resource: "networkpolicies"}, networkPolicyAllowFromShootAPIServer.Name)))
					Expect(kapi.Deploy(ctx)).To(Succeed())
					Expect(c.Get(ctx, client.ObjectKeyFromObject(networkPolicyAllowFromShootAPIServer), networkPolicyAllowFromShootAPIServer)).To(Succeed())
					Expect(networkPolicyAllowFromShootAPIServer).To(DeepEqual(&networkingv1.NetworkPolicy{
						TypeMeta: metav1.TypeMeta{
							APIVersion: networkingv1.SchemeGroupVersion.String(),
							Kind:       "NetworkPolicy",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name:            networkPolicyAllowFromShootAPIServer.Name,
							Namespace:       networkPolicyAllowFromShootAPIServer.Namespace,
							ResourceVersion: "1",
							Annotations: map[string]string{
								"gardener.cloud/description": "Allows Egress from Shoot's Kubernetes API Server to talk to " +
									"pods labeled with 'networking.gardener.cloud/from-shoot-apiserver=allowed'.",
							},
						},
						Spec: networkingv1.NetworkPolicySpec{
							PodSelector: metav1.LabelSelector{
								MatchLabels: map[string]string{"networking.gardener.cloud/from-shoot-apiserver": "allowed"},
							},
							Ingress: []networkingv1.NetworkPolicyIngressRule{{
								From: []networkingv1.NetworkPolicyPeer{{
									PodSelector: &metav1.LabelSelector{
										MatchLabels: map[string]string{
											"app":                 "kubernetes",
											"gardener.cloud/role": "controlplane",
											"role":                "apiserver",
										},
									},
								}},
							}},
							PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress},
						},
					}))
				})

				It("should successfully deploy the allow-to-shoot-apiserver NetworkPolicy resource", func() {
					var (
						protocol = corev1.ProtocolTCP
						port     = intstr.FromInt(443)
					)

					Expect(c.Get(ctx, client.ObjectKeyFromObject(networkPolicyAllowToShootAPIServer), networkPolicyAllowToShootAPIServer)).To(MatchError(apierrors.NewNotFound(schema.GroupResource{Group: networkingv1.SchemeGroupVersion.Group, Resource: "networkpolicies"}, networkPolicyAllowToShootAPIServer.Name)))
					Expect(kapi.Deploy(ctx)).To(Succeed())
					Expect(c.Get(ctx, client.ObjectKeyFromObject(networkPolicyAllowToShootAPIServer), networkPolicyAllowToShootAPIServer)).To(Succeed())
					Expect(networkPolicyAllowToShootAPIServer).To(DeepEqual(&networkingv1.NetworkPolicy{
						TypeMeta: metav1.TypeMeta{
							APIVersion: networkingv1.SchemeGroupVersion.String(),
							Kind:       "NetworkPolicy",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name:            networkPolicyAllowToShootAPIServer.Name,
							Namespace:       networkPolicyAllowToShootAPIServer.Namespace,
							ResourceVersion: "1",
							Annotations: map[string]string{
								"gardener.cloud/description": "Allows Egress from pods labeled with " +
									"'networking.gardener.cloud/to-shoot-apiserver=allowed' to talk to Shoot's Kubernetes " +
									"API Server.",
							},
						},
						Spec: networkingv1.NetworkPolicySpec{
							PodSelector: metav1.LabelSelector{
								MatchLabels: map[string]string{"networking.gardener.cloud/to-shoot-apiserver": "allowed"},
							},
							Egress: []networkingv1.NetworkPolicyEgressRule{{
								To: []networkingv1.NetworkPolicyPeer{{
									PodSelector: &metav1.LabelSelector{
										MatchLabels: map[string]string{
											"app":                 "kubernetes",
											"gardener.cloud/role": "controlplane",
											"role":                "apiserver",
										},
									},
								}},
								Ports: []networkingv1.NetworkPolicyPort{{
									Protocol: &protocol,
									Port:     &port,
								}},
							}},
							PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeEgress},
						},
					}))
				})

				Context("should successfully deploy the allow-kube-apiserver NetworkPolicy resource", func() {
					var (
						protocol             = corev1.ProtocolTCP
						portAPIServer        = intstr.FromInt(443)
						portBlackboxExporter = intstr.FromInt(9115)
						portEtcd             = intstr.FromInt(2379)
						portVPNSeedServer    = intstr.FromInt(9443)
					)

					It("w/o ReversedVPN", func() {
						Expect(c.Get(ctx, client.ObjectKeyFromObject(networkPolicyAllowKubeAPIServer), networkPolicyAllowKubeAPIServer)).To(MatchError(apierrors.NewNotFound(schema.GroupResource{Group: networkingv1.SchemeGroupVersion.Group, Resource: "networkpolicies"}, networkPolicyAllowKubeAPIServer.Name)))
						Expect(kapi.Deploy(ctx)).To(Succeed())
						Expect(c.Get(ctx, client.ObjectKeyFromObject(networkPolicyAllowKubeAPIServer), networkPolicyAllowKubeAPIServer)).To(Succeed())
						Expect(networkPolicyAllowKubeAPIServer).To(DeepEqual(&networkingv1.NetworkPolicy{
							TypeMeta: metav1.TypeMeta{
								APIVersion: networkingv1.SchemeGroupVersion.String(),
								Kind:       "NetworkPolicy",
							},
							ObjectMeta: metav1.ObjectMeta{
								Name:            networkPolicyAllowKubeAPIServer.Name,
								Namespace:       networkPolicyAllowKubeAPIServer.Namespace,
								ResourceVersion: "1",
								Annotations: map[string]string{
									"gardener.cloud/description": "Allows Ingress to the Shoot's Kubernetes API Server from " +
										"pods labeled with 'networking.gardener.cloud/to-shoot-apiserver=allowed' and " +
										"Prometheus, and Egress to etcd pods.",
								},
							},
							Spec: networkingv1.NetworkPolicySpec{
								PodSelector: metav1.LabelSelector{
									MatchLabels: map[string]string{
										"app":                 "kubernetes",
										"gardener.cloud/role": "controlplane",
										"role":                "apiserver",
									},
								},
								Egress: []networkingv1.NetworkPolicyEgressRule{{
									To: []networkingv1.NetworkPolicyPeer{{
										PodSelector: &metav1.LabelSelector{
											MatchLabels: map[string]string{
												"app":                     "etcd-statefulset",
												"garden.sapcloud.io/role": "controlplane",
											},
										},
									}},
									Ports: []networkingv1.NetworkPolicyPort{{
										Protocol: &protocol,
										Port:     &portEtcd,
									}},
								}},
								Ingress: []networkingv1.NetworkPolicyIngressRule{
									{
										From: []networkingv1.NetworkPolicyPeer{
											{PodSelector: &metav1.LabelSelector{}},
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
												MatchLabels: map[string]string{
													"gardener.cloud/role": "monitoring",
													"app":                 "prometheus",
													"role":                "monitoring",
												},
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
							},
						}))
					})

					It("w/ ReversedVPN", func() {
						kapi = New(kubernetesInterface, namespace, Values{VPN: VPNConfig{ReversedVPNEnabled: true}, Version: version})
						kapi.SetSecrets(secrets)

						Expect(c.Get(ctx, client.ObjectKeyFromObject(networkPolicyAllowKubeAPIServer), networkPolicyAllowKubeAPIServer)).To(MatchError(apierrors.NewNotFound(schema.GroupResource{Group: networkingv1.SchemeGroupVersion.Group, Resource: "networkpolicies"}, networkPolicyAllowKubeAPIServer.Name)))
						Expect(kapi.Deploy(ctx)).To(Succeed())
						Expect(c.Get(ctx, client.ObjectKeyFromObject(networkPolicyAllowKubeAPIServer), networkPolicyAllowKubeAPIServer)).To(Succeed())
						Expect(networkPolicyAllowKubeAPIServer).To(DeepEqual(&networkingv1.NetworkPolicy{
							TypeMeta: metav1.TypeMeta{
								APIVersion: networkingv1.SchemeGroupVersion.String(),
								Kind:       "NetworkPolicy",
							},
							ObjectMeta: metav1.ObjectMeta{
								Name:            networkPolicyAllowKubeAPIServer.Name,
								Namespace:       networkPolicyAllowKubeAPIServer.Namespace,
								ResourceVersion: "1",
								Annotations: map[string]string{
									"gardener.cloud/description": "Allows Ingress to the Shoot's Kubernetes API Server from " +
										"pods labeled with 'networking.gardener.cloud/to-shoot-apiserver=allowed' and " +
										"Prometheus, and Egress to etcd pods.",
								},
							},
							Spec: networkingv1.NetworkPolicySpec{
								PodSelector: metav1.LabelSelector{
									MatchLabels: map[string]string{
										"app":                 "kubernetes",
										"gardener.cloud/role": "controlplane",
										"role":                "apiserver",
									},
								},
								Egress: []networkingv1.NetworkPolicyEgressRule{
									{
										To: []networkingv1.NetworkPolicyPeer{{
											PodSelector: &metav1.LabelSelector{
												MatchLabels: map[string]string{
													"app":                     "etcd-statefulset",
													"garden.sapcloud.io/role": "controlplane",
												},
											},
										}},
										Ports: []networkingv1.NetworkPolicyPort{{
											Protocol: &protocol,
											Port:     &portEtcd,
										}},
									},
									{
										To: []networkingv1.NetworkPolicyPeer{{
											PodSelector: &metav1.LabelSelector{
												MatchLabels: map[string]string{
													"gardener.cloud/role": "controlplane",
													"app":                 "vpn-seed-server",
												},
											},
										}},
										Ports: []networkingv1.NetworkPolicyPort{{
											Protocol: &protocol,
											Port:     &portVPNSeedServer,
										}},
									},
								},
								Ingress: []networkingv1.NetworkPolicyIngressRule{
									{
										From: []networkingv1.NetworkPolicyPeer{
											{PodSelector: &metav1.LabelSelector{}},
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
												MatchLabels: map[string]string{
													"gardener.cloud/role": "monitoring",
													"app":                 "prometheus",
													"role":                "monitoring",
												},
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
							},
						}))
					})
				})
			})

			Describe("Shoot Resources", func() {
				It("should successfully deploy the managed resource secret", func() {
					var (
						clusterRoleYAML = `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  creationTimestamp: null
  name: system:apiserver:kubelet
rules:
- apiGroups:
  - ""
  resources:
  - nodes/proxy
  - nodes/stats
  - nodes/log
  - nodes/spec
  - nodes/metrics
  verbs:
  - '*'
- nonResourceURLs:
  - '*'
  verbs:
  - '*'
`
						clusterRoleBindingYAML = `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  annotations:
    resources.gardener.cloud/delete-on-invalid-update: "true"
  creationTimestamp: null
  name: system:apiserver:kubelet
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: system:apiserver:kubelet
subjects:
- kind: User
  name: system:kube-apiserver:kubelet
`
					)

					Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(MatchError(apierrors.NewNotFound(schema.GroupResource{Group: corev1.SchemeGroupVersion.Group, Resource: "secrets"}, managedResourceSecret.Name)))
					Expect(kapi.Deploy(ctx)).To(Succeed())
					Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(Succeed())
					Expect(managedResourceSecret).To(DeepEqual(&corev1.Secret{
						TypeMeta: metav1.TypeMeta{
							APIVersion: corev1.SchemeGroupVersion.String(),
							Kind:       "Secret",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name:            managedResourceSecret.Name,
							Namespace:       managedResourceSecret.Namespace,
							ResourceVersion: "1",
						},
						Type: corev1.SecretTypeOpaque,
						Data: map[string][]byte{
							"clusterrole____system_apiserver_kubelet.yaml":        []byte(clusterRoleYAML),
							"clusterrolebinding____system_apiserver_kubelet.yaml": []byte(clusterRoleBindingYAML),
						},
					}))
				})

				It("should successfully deploy the managed resource", func() {
					Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(MatchError(apierrors.NewNotFound(schema.GroupResource{Group: resourcesv1alpha1.SchemeGroupVersion.Group, Resource: "managedresources"}, managedResource.Name)))
					Expect(kapi.Deploy(ctx)).To(Succeed())
					Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
					Expect(managedResource).To(DeepEqual(&resourcesv1alpha1.ManagedResource{
						TypeMeta: metav1.TypeMeta{
							APIVersion: resourcesv1alpha1.SchemeGroupVersion.String(),
							Kind:       "ManagedResource",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name:            managedResource.Name,
							Namespace:       managedResource.Namespace,
							ResourceVersion: "1",
							Labels: map[string]string{
								"origin": "gardener",
							},
						},
						Spec: resourcesv1alpha1.ManagedResourceSpec{
							InjectLabels: map[string]string{"shoot.gardener.cloud/no-cleanup": "true"},
							KeepObjects:  pointer.Bool(false),
							SecretRefs:   []corev1.LocalObjectReference{{Name: managedResourceSecret.Name}},
						},
					}))
				})
			})

			Describe("Secrets", func() {
				It("should successfully deploy the OIDCCABundle secret resource", func() {
					var (
						caBundle   = "some-ca-bundle"
						oidcConfig = &gardencorev1beta1.OIDCConfig{CABundle: &caBundle}
					)

					kapi = New(kubernetesInterface, namespace, Values{OIDC: oidcConfig, Version: version})
					kapi.SetSecrets(secrets)

					secretOIDCCABundle = &corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{Name: "kube-apiserver-oidc-cabundle", Namespace: namespace},
						Data:       map[string][]byte{"ca.crt": []byte(caBundle)},
					}
					Expect(kutil.MakeUnique(secretOIDCCABundle)).To(Succeed())

					Expect(c.Get(ctx, client.ObjectKeyFromObject(secretOIDCCABundle), secretOIDCCABundle)).To(MatchError(apierrors.NewNotFound(schema.GroupResource{Group: corev1.SchemeGroupVersion.Group, Resource: "secrets"}, secretOIDCCABundle.Name)))
					Expect(kapi.Deploy(ctx)).To(Succeed())
					Expect(c.Get(ctx, client.ObjectKeyFromObject(secretOIDCCABundle), secretOIDCCABundle)).To(Succeed())
					Expect(secretOIDCCABundle).To(DeepEqual(&corev1.Secret{
						TypeMeta: metav1.TypeMeta{
							APIVersion: corev1.SchemeGroupVersion.String(),
							Kind:       "Secret",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name:            secretOIDCCABundle.Name,
							Namespace:       secretOIDCCABundle.Namespace,
							Labels:          map[string]string{"resources.gardener.cloud/garbage-collectable-reference": "true"},
							ResourceVersion: "1",
						},
						Immutable: pointer.Bool(true),
						Data:      secretOIDCCABundle.Data,
					}))
				})

				It("should successfully deploy the ServiceAccountSigningKey secret resource", func() {
					var (
						signingKey           = []byte("some-signingkey")
						serviceAccountConfig = ServiceAccountConfig{SigningKey: signingKey}
					)

					kapi = New(kubernetesInterface, namespace, Values{ServiceAccount: serviceAccountConfig, Version: version})
					kapi.SetSecrets(secrets)

					secretServiceAccountSigningKey = &corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{Name: "kube-apiserver-sa-signing-key", Namespace: namespace},
						Data:       map[string][]byte{"signing-key": signingKey},
					}
					Expect(kutil.MakeUnique(secretServiceAccountSigningKey)).To(Succeed())

					Expect(c.Get(ctx, client.ObjectKeyFromObject(secretServiceAccountSigningKey), secretServiceAccountSigningKey)).To(MatchError(apierrors.NewNotFound(schema.GroupResource{Group: corev1.SchemeGroupVersion.Group, Resource: "secrets"}, secretServiceAccountSigningKey.Name)))
					Expect(kapi.Deploy(ctx)).To(Succeed())
					Expect(c.Get(ctx, client.ObjectKeyFromObject(secretServiceAccountSigningKey), secretServiceAccountSigningKey)).To(Succeed())
					Expect(secretServiceAccountSigningKey).To(DeepEqual(&corev1.Secret{
						TypeMeta: metav1.TypeMeta{
							APIVersion: corev1.SchemeGroupVersion.String(),
							Kind:       "Secret",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name:            secretServiceAccountSigningKey.Name,
							Namespace:       secretServiceAccountSigningKey.Namespace,
							Labels:          map[string]string{"resources.gardener.cloud/garbage-collectable-reference": "true"},
							ResourceVersion: "1",
						},
						Immutable: pointer.Bool(true),
						Data:      secretServiceAccountSigningKey.Data,
					}))
				})
			})

			Describe("ConfigMaps", func() {
				Context("admission", func() {
					It("should successfully deploy the configmap resource w/o admission plugins", func() {
						configMapAdmission = &corev1.ConfigMap{
							ObjectMeta: metav1.ObjectMeta{Name: "kube-apiserver-admission-config", Namespace: namespace},
							Data: map[string]string{"admission-configuration.yaml": `apiVersion: apiserver.k8s.io/v1alpha1
kind: AdmissionConfiguration
plugins: null
`},
						}
						Expect(kutil.MakeUnique(configMapAdmission)).To(Succeed())

						Expect(c.Get(ctx, client.ObjectKeyFromObject(configMapAdmission), configMapAdmission)).To(MatchError(apierrors.NewNotFound(schema.GroupResource{Group: corev1.SchemeGroupVersion.Group, Resource: "configmaps"}, configMapAdmission.Name)))
						Expect(kapi.Deploy(ctx)).To(Succeed())
						Expect(c.Get(ctx, client.ObjectKeyFromObject(configMapAdmission), configMapAdmission)).To(Succeed())
						Expect(configMapAdmission).To(DeepEqual(&corev1.ConfigMap{
							TypeMeta: metav1.TypeMeta{
								APIVersion: corev1.SchemeGroupVersion.String(),
								Kind:       "ConfigMap",
							},
							ObjectMeta: metav1.ObjectMeta{
								Name:            configMapAdmission.Name,
								Namespace:       configMapAdmission.Namespace,
								Labels:          map[string]string{"resources.gardener.cloud/garbage-collectable-reference": "true"},
								ResourceVersion: "1",
							},
							Immutable: pointer.Bool(true),
							Data:      configMapAdmission.Data,
						}))
					})

					It("should successfully deploy the configmap resource w/ admission plugins", func() {
						admissionPlugins := []gardencorev1beta1.AdmissionPlugin{
							{Name: "Foo"},
							{Name: "Bar"},
							{Name: "Baz", Config: &runtime.RawExtension{Raw: []byte("some-config-for-baz")}},
						}

						kapi = New(kubernetesInterface, namespace, Values{AdmissionPlugins: admissionPlugins, Version: version})
						kapi.SetSecrets(secrets)

						configMapAdmission = &corev1.ConfigMap{
							ObjectMeta: metav1.ObjectMeta{Name: "kube-apiserver-admission-config", Namespace: namespace},
							Data: map[string]string{
								"admission-configuration.yaml": `apiVersion: apiserver.k8s.io/v1alpha1
kind: AdmissionConfiguration
plugins:
- configuration: null
  name: Baz
  path: /etc/kubernetes/admission/baz.yaml
`,
								"baz.yaml": "some-config-for-baz",
							},
						}
						Expect(kutil.MakeUnique(configMapAdmission)).To(Succeed())

						Expect(c.Get(ctx, client.ObjectKeyFromObject(configMapAdmission), configMapAdmission)).To(MatchError(apierrors.NewNotFound(schema.GroupResource{Group: corev1.SchemeGroupVersion.Group, Resource: "configmaps"}, configMapAdmission.Name)))
						Expect(kapi.Deploy(ctx)).To(Succeed())
						Expect(c.Get(ctx, client.ObjectKeyFromObject(configMapAdmission), configMapAdmission)).To(Succeed())
						Expect(configMapAdmission).To(DeepEqual(&corev1.ConfigMap{
							TypeMeta: metav1.TypeMeta{
								APIVersion: corev1.SchemeGroupVersion.String(),
								Kind:       "ConfigMap",
							},
							ObjectMeta: metav1.ObjectMeta{
								Name:            configMapAdmission.Name,
								Namespace:       configMapAdmission.Namespace,
								Labels:          map[string]string{"resources.gardener.cloud/garbage-collectable-reference": "true"},
								ResourceVersion: "1",
							},
							Immutable: pointer.Bool(true),
							Data:      configMapAdmission.Data,
						}))
					})
				})

				Context("audit policy", func() {
					It("should successfully deploy the configmap resource w/ default policy", func() {
						configMapAuditPolicy = &corev1.ConfigMap{
							ObjectMeta: metav1.ObjectMeta{Name: "audit-policy-config", Namespace: namespace},
							Data: map[string]string{"audit-policy.yaml": `apiVersion: audit.k8s.io/v1
kind: Policy
metadata:
  creationTimestamp: null
rules:
- level: None
`},
						}
						Expect(kutil.MakeUnique(configMapAuditPolicy)).To(Succeed())

						Expect(c.Get(ctx, client.ObjectKeyFromObject(configMapAuditPolicy), configMapAuditPolicy)).To(MatchError(apierrors.NewNotFound(schema.GroupResource{Group: corev1.SchemeGroupVersion.Group, Resource: "configmaps"}, configMapAuditPolicy.Name)))
						Expect(kapi.Deploy(ctx)).To(Succeed())
						Expect(c.Get(ctx, client.ObjectKeyFromObject(configMapAuditPolicy), configMapAuditPolicy)).To(Succeed())
						Expect(configMapAuditPolicy).To(DeepEqual(&corev1.ConfigMap{
							TypeMeta: metav1.TypeMeta{
								APIVersion: corev1.SchemeGroupVersion.String(),
								Kind:       "ConfigMap",
							},
							ObjectMeta: metav1.ObjectMeta{
								Name:            configMapAuditPolicy.Name,
								Namespace:       configMapAuditPolicy.Namespace,
								Labels:          map[string]string{"resources.gardener.cloud/garbage-collectable-reference": "true"},
								ResourceVersion: "1",
							},
							Immutable: pointer.Bool(true),
							Data:      configMapAuditPolicy.Data,
						}))
					})

					It("should successfully deploy the configmap resource w/o default policy", func() {
						var (
							policy      = "some-audit-policy"
							auditConfig = &AuditConfig{Policy: &policy}
						)

						kapi = New(kubernetesInterface, namespace, Values{Audit: auditConfig, Version: version})
						kapi.SetSecrets(secrets)

						configMapAuditPolicy = &corev1.ConfigMap{
							ObjectMeta: metav1.ObjectMeta{Name: "audit-policy-config", Namespace: namespace},
							Data:       map[string]string{"audit-policy.yaml": policy},
						}
						Expect(kutil.MakeUnique(configMapAuditPolicy)).To(Succeed())

						Expect(c.Get(ctx, client.ObjectKeyFromObject(configMapAuditPolicy), configMapAuditPolicy)).To(MatchError(apierrors.NewNotFound(schema.GroupResource{Group: corev1.SchemeGroupVersion.Group, Resource: "configmaps"}, configMapAuditPolicy.Name)))
						Expect(kapi.Deploy(ctx)).To(Succeed())
						Expect(c.Get(ctx, client.ObjectKeyFromObject(configMapAuditPolicy), configMapAuditPolicy)).To(Succeed())
						Expect(configMapAuditPolicy).To(DeepEqual(&corev1.ConfigMap{
							TypeMeta: metav1.TypeMeta{
								APIVersion: corev1.SchemeGroupVersion.String(),
								Kind:       "ConfigMap",
							},
							ObjectMeta: metav1.ObjectMeta{
								Name:            configMapAuditPolicy.Name,
								Namespace:       configMapAuditPolicy.Namespace,
								Labels:          map[string]string{"resources.gardener.cloud/garbage-collectable-reference": "true"},
								ResourceVersion: "1",
							},
							Immutable: pointer.Bool(true),
							Data:      configMapAuditPolicy.Data,
						}))
					})
				})

				Context("egress selector", func() {
					It("should successfully deploy the configmap resource for K8s < 1.20", func() {
						kapi = New(kubernetesInterface, namespace, Values{VPN: VPNConfig{ReversedVPNEnabled: true}, Version: semver.MustParse("1.19.0")})
						kapi.SetSecrets(secrets)

						configMapEgressSelector = &corev1.ConfigMap{
							ObjectMeta: metav1.ObjectMeta{Name: "kube-apiserver-egress-selector-config", Namespace: namespace},
							Data:       map[string]string{"egress-selector-configuration.yaml": egressSelectorConfigFor("master")},
						}
						Expect(kutil.MakeUnique(configMapEgressSelector)).To(Succeed())

						Expect(c.Get(ctx, client.ObjectKeyFromObject(configMapEgressSelector), configMapEgressSelector)).To(MatchError(apierrors.NewNotFound(schema.GroupResource{Group: corev1.SchemeGroupVersion.Group, Resource: "configmaps"}, configMapEgressSelector.Name)))
						Expect(kapi.Deploy(ctx)).To(Succeed())
						Expect(c.Get(ctx, client.ObjectKeyFromObject(configMapEgressSelector), configMapEgressSelector)).To(Succeed())
						Expect(configMapEgressSelector).To(DeepEqual(&corev1.ConfigMap{
							TypeMeta: metav1.TypeMeta{
								APIVersion: corev1.SchemeGroupVersion.String(),
								Kind:       "ConfigMap",
							},
							ObjectMeta: metav1.ObjectMeta{
								Name:            configMapEgressSelector.Name,
								Namespace:       configMapEgressSelector.Namespace,
								Labels:          map[string]string{"resources.gardener.cloud/garbage-collectable-reference": "true"},
								ResourceVersion: "1",
							},
							Immutable: pointer.Bool(true),
							Data:      configMapEgressSelector.Data,
						}))
					})

					It("should successfully deploy the configmap resource for K8s >= 1.20", func() {
						kapi = New(kubernetesInterface, namespace, Values{VPN: VPNConfig{ReversedVPNEnabled: true}, Version: version})
						kapi.SetSecrets(secrets)

						configMapEgressSelector = &corev1.ConfigMap{
							ObjectMeta: metav1.ObjectMeta{Name: "kube-apiserver-egress-selector-config", Namespace: namespace},
							Data:       map[string]string{"egress-selector-configuration.yaml": egressSelectorConfigFor("controlplane")},
						}
						Expect(kutil.MakeUnique(configMapEgressSelector)).To(Succeed())

						Expect(c.Get(ctx, client.ObjectKeyFromObject(configMapEgressSelector), configMapEgressSelector)).To(MatchError(apierrors.NewNotFound(schema.GroupResource{Group: corev1.SchemeGroupVersion.Group, Resource: "configmaps"}, configMapEgressSelector.Name)))
						Expect(kapi.Deploy(ctx)).To(Succeed())
						Expect(c.Get(ctx, client.ObjectKeyFromObject(configMapEgressSelector), configMapEgressSelector)).To(Succeed())
						Expect(configMapEgressSelector).To(DeepEqual(&corev1.ConfigMap{
							TypeMeta: metav1.TypeMeta{
								APIVersion: corev1.SchemeGroupVersion.String(),
								Kind:       "ConfigMap",
							},
							ObjectMeta: metav1.ObjectMeta{
								Name:            configMapEgressSelector.Name,
								Namespace:       configMapEgressSelector.Namespace,
								Labels:          map[string]string{"resources.gardener.cloud/garbage-collectable-reference": "true"},
								ResourceVersion: "1",
							},
							Immutable: pointer.Bool(true),
							Data:      configMapEgressSelector.Data,
						}))
					})
				})
			})

			Describe("Deployment", func() {
				deployAndRead := func() {
					Expect(c.Get(ctx, client.ObjectKeyFromObject(deployment), deployment)).To(MatchError(apierrors.NewNotFound(schema.GroupResource{Group: appsv1.SchemeGroupVersion.Group, Resource: "deployments"}, deployment.Name)))
					Expect(kapi.Deploy(ctx)).To(Succeed())
					Expect(c.Get(ctx, client.ObjectKeyFromObject(deployment), deployment)).To(Succeed())
				}

				It("should have the expected labels w/o SNI", func() {
					deployAndRead()

					Expect(deployment.Labels).To(Equal(map[string]string{
						"gardener.cloud/role": "controlplane",
						"app":                 "kubernetes",
						"role":                "apiserver",
					}))
				})

				It("should have the expected labels w/ SNI", func() {
					kapi = New(kubernetesInterface, namespace, Values{SNI: SNIConfig{Enabled: true}, Version: version})
					kapi.SetSecrets(secrets)
					deployAndRead()

					Expect(deployment.Labels).To(Equal(map[string]string{
						"gardener.cloud/role":                    "controlplane",
						"app":                                    "kubernetes",
						"role":                                   "apiserver",
						"core.gardener.cloud/apiserver-exposure": "gardener-managed",
					}))
				})

				It("should have the expected annotations", func() {
					deployAndRead()

					Expect(deployment.Annotations).To(Equal(map[string]string{
						"reference.resources.gardener.cloud/secret-7e9b40c7":    secretNameServiceAccountKey,
						"reference.resources.gardener.cloud/secret-71fba891":    secretNameCA,
						"reference.resources.gardener.cloud/secret-91c30740":    secretNameCAEtcd,
						"reference.resources.gardener.cloud/secret-9282d44f":    secretNameCAFrontProxy,
						"reference.resources.gardener.cloud/secret-c16f0542":    secretNameEtcd,
						"reference.resources.gardener.cloud/secret-7e1cfe53":    secretNameEtcdEncryptionConfig,
						"reference.resources.gardener.cloud/secret-274d0dbb":    secretNameKubeAPIServerToKubelet,
						"reference.resources.gardener.cloud/secret-2e310c99":    secretNameKubeAggregator,
						"reference.resources.gardener.cloud/secret-59f1a197":    secretNameStaticToken,
						"reference.resources.gardener.cloud/secret-e2878235":    secretNameServer,
						"reference.resources.gardener.cloud/secret-9f3de87f":    secretNameVPNSeed,
						"reference.resources.gardener.cloud/secret-e638c9f3":    secretNameVPNSeedTLSAuth,
						"reference.resources.gardener.cloud/configmap-130aa219": "kube-apiserver-admission-config-e38ff146",
						"reference.resources.gardener.cloud/configmap-d4419cd4": "audit-policy-config-f5b578b4",
					}))
				})

				It("should have the expected deployment settings", func() {
					var (
						replicas        int32 = 1337
						intStr25Percent       = intstr.FromString("25%")
						intStrZero            = intstr.FromInt(0)
					)

					kapi = New(kubernetesInterface, namespace, Values{Autoscaling: AutoscalingConfig{Replicas: &replicas}, Version: version})
					kapi.SetSecrets(secrets)
					deployAndRead()

					Expect(deployment.Spec.MinReadySeconds).To(Equal(int32(30)))
					Expect(deployment.Spec.RevisionHistoryLimit).To(PointTo(Equal(int32(2))))
					Expect(deployment.Spec.Replicas).To(PointTo(Equal(replicas)))
					Expect(deployment.Spec.Selector).To(Equal(&metav1.LabelSelector{MatchLabels: map[string]string{
						"app":  "kubernetes",
						"role": "apiserver",
					}}))
					Expect(deployment.Spec.Strategy).To(Equal(appsv1.DeploymentStrategy{
						Type: appsv1.RollingUpdateDeploymentStrategyType,
						RollingUpdate: &appsv1.RollingUpdateDeployment{
							MaxSurge:       &intStr25Percent,
							MaxUnavailable: &intStrZero,
						},
					}))
				})

				It("should have the expected pod template metadata", func() {
					deployAndRead()

					Expect(deployment.Spec.Template.Annotations).To(Equal(map[string]string{
						"checksum/secret-" + secretNameCAEtcd:                   secretChecksumCAEtcd,
						"checksum/secret-" + secretNameServiceAccountKey:        secretChecksumServiceAccountKey,
						"checksum/secret-" + secretNameVPNSeedServerTLSAuth:     secretChecksumVPNSeedServerTLSAuth,
						"checksum/secret-" + secretNameVPNSeed:                  secretChecksumVPNSeed,
						"checksum/secret-" + secretNameCA:                       secretChecksumCA,
						"checksum/secret-" + secretNameEtcd:                     secretChecksumEtcd,
						"checksum/secret-" + secretNameHTTPProxy:                secretChecksumHTTPProxy,
						"checksum/secret-" + secretNameStaticToken:              secretChecksumStaticToken,
						"checksum/secret-" + secretNameKubeAggregator:           secretChecksumKubeAggregator,
						"checksum/secret-" + secretNameBasicAuthentication:      secretChecksumBasicAuthentication,
						"checksum/secret-" + secretNameCAFrontProxy:             secretChecksumCAFrontProxy,
						"checksum/secret-" + secretNameKubeAPIServerToKubelet:   secretChecksumKubeAPIServerToKubelet,
						"checksum/secret-" + secretNameEtcdEncryptionConfig:     secretChecksumEtcdEncryptionConfig,
						"checksum/secret-" + secretNameServer:                   secretChecksumServer,
						"checksum/secret-" + secretNameVPNSeedTLSAuth:           secretChecksumVPNSeedTLSAuth,
						"reference.resources.gardener.cloud/secret-7e9b40c7":    secretNameServiceAccountKey,
						"reference.resources.gardener.cloud/secret-71fba891":    secretNameCA,
						"reference.resources.gardener.cloud/secret-91c30740":    secretNameCAEtcd,
						"reference.resources.gardener.cloud/secret-9282d44f":    secretNameCAFrontProxy,
						"reference.resources.gardener.cloud/secret-c16f0542":    secretNameEtcd,
						"reference.resources.gardener.cloud/secret-7e1cfe53":    secretNameEtcdEncryptionConfig,
						"reference.resources.gardener.cloud/secret-274d0dbb":    secretNameKubeAPIServerToKubelet,
						"reference.resources.gardener.cloud/secret-2e310c99":    secretNameKubeAggregator,
						"reference.resources.gardener.cloud/secret-59f1a197":    secretNameStaticToken,
						"reference.resources.gardener.cloud/secret-e2878235":    secretNameServer,
						"reference.resources.gardener.cloud/secret-9f3de87f":    secretNameVPNSeed,
						"reference.resources.gardener.cloud/secret-e638c9f3":    secretNameVPNSeedTLSAuth,
						"reference.resources.gardener.cloud/configmap-130aa219": "kube-apiserver-admission-config-e38ff146",
						"reference.resources.gardener.cloud/configmap-d4419cd4": "audit-policy-config-f5b578b4",
					}))
					Expect(deployment.Spec.Template.Labels).To(Equal(map[string]string{
						"garden.sapcloud.io/role":          "controlplane",
						"gardener.cloud/role":              "controlplane",
						"app":                              "kubernetes",
						"role":                             "apiserver",
						"networking.gardener.cloud/to-dns": "allowed",
						"networking.gardener.cloud/to-private-networks": "allowed",
						"networking.gardener.cloud/to-public-networks":  "allowed",
						"networking.gardener.cloud/to-shoot-networks":   "allowed",
						"networking.gardener.cloud/from-prometheus":     "allowed",
					}))
				})

				It("should have the expected pod settings", func() {
					deployAndRead()

					Expect(deployment.Spec.Template.Spec.Affinity).To(Equal(&corev1.Affinity{
						PodAntiAffinity: &corev1.PodAntiAffinity{
							PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{{
								Weight: 1,
								PodAffinityTerm: corev1.PodAffinityTerm{
									TopologyKey: "kubernetes.io/hostname",
									LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{
										"app":  "kubernetes",
										"role": "apiserver",
									}},
								},
							}},
						},
					}))
					Expect(deployment.Spec.Template.Spec.PriorityClassName).To(Equal("gardener-shoot-controlplane"))
					Expect(deployment.Spec.Template.Spec.DNSPolicy).To(Equal(corev1.DNSClusterFirst))
					Expect(deployment.Spec.Template.Spec.RestartPolicy).To(Equal(corev1.RestartPolicyAlways))
					Expect(deployment.Spec.Template.Spec.SchedulerName).To(Equal("default-scheduler"))
					Expect(deployment.Spec.Template.Spec.TerminationGracePeriodSeconds).To(PointTo(Equal(int64(30))))
				})

				It("should have no init containers when reversed vpn is enabled", func() {
					kapi = New(kubernetesInterface, namespace, Values{VPN: VPNConfig{ReversedVPNEnabled: true}, Version: version})
					kapi.SetSecrets(secrets)
					deployAndRead()

					Expect(deployment.Spec.Template.Spec.InitContainers).To(BeEmpty())
				})

				It("should have one init container and the vpn-seed sidecar container when reversed vpn is disabled", func() {
					var (
						images    = Images{AlpineIPTables: "some-image:latest", VPNSeed: "some-other-image:really-latest"}
						vpnConfig = VPNConfig{
							ReversedVPNEnabled: false,
							PodNetworkCIDR:     "1.2.3.4/5",
							ServiceNetworkCIDR: "6.7.8.9/10",
							NodeNetworkCIDR:    pointer.String("11.12.13.14/15"),
						}
					)

					kapi = New(kubernetesInterface, namespace, Values{VPN: vpnConfig, Images: images, Version: version})
					kapi.SetSecrets(secrets)
					deployAndRead()

					Expect(deployment.Spec.Template.Spec.InitContainers).To(ConsistOf(corev1.Container{
						Name:  "set-iptable-rules",
						Image: images.AlpineIPTables,
						Command: []string{
							"/bin/sh",
							"-c",
							"iptables -A INPUT -i tun0 -p icmp -j ACCEPT && iptables -A INPUT -i tun0 -m state --state NEW -j DROP",
						},
						SecurityContext: &corev1.SecurityContext{
							Capabilities: &corev1.Capabilities{
								Add: []corev1.Capability{"NET_ADMIN"},
							},
							Privileged: pointer.Bool(true),
						},
						VolumeMounts: []corev1.VolumeMount{{
							Name:      "modules",
							MountPath: "/lib/modules",
						}},
					}))
					Expect(deployment.Spec.Template.Spec.Volumes).To(ContainElement(corev1.Volume{
						Name: "modules",
						VolumeSource: corev1.VolumeSource{
							HostPath: &corev1.HostPathVolumeSource{Path: "/lib/modules"},
						},
					}))

					Expect(deployment.Spec.Template.Spec.Containers).To(ContainElement(corev1.Container{
						Name:            "vpn-seed",
						Image:           images.VPNSeed,
						ImagePullPolicy: corev1.PullIfNotPresent,
						Env: []corev1.EnvVar{
							{
								Name:  "MAIN_VPN_SEED",
								Value: "true",
							},
							{
								Name:  "OPENVPN_PORT",
								Value: "4314",
							},
							{
								Name:  "APISERVER_AUTH_MODE",
								Value: "client-cert",
							},
							{
								Name:  "APISERVER_AUTH_MODE_CLIENT_CERT_CA",
								Value: "/srv/secrets/vpn-seed/ca.crt",
							},
							{
								Name:  "APISERVER_AUTH_MODE_CLIENT_CERT_CRT",
								Value: "/srv/secrets/vpn-seed/tls.crt",
							},
							{
								Name:  "APISERVER_AUTH_MODE_CLIENT_CERT_KEY",
								Value: "/srv/secrets/vpn-seed/tls.key",
							},
							{
								Name:  "SERVICE_NETWORK",
								Value: vpnConfig.ServiceNetworkCIDR,
							},
							{
								Name:  "POD_NETWORK",
								Value: vpnConfig.PodNetworkCIDR,
							},
							{
								Name:  "NODE_NETWORK",
								Value: *vpnConfig.NodeNetworkCIDR,
							},
						},
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("100m"),
								corev1.ResourceMemory: resource.MustParse("128Mi"),
							},
							Limits: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("500m"),
								corev1.ResourceMemory: resource.MustParse("1000Mi"),
							},
						},
						SecurityContext: &corev1.SecurityContext{
							Capabilities: &corev1.Capabilities{
								Add: []corev1.Capability{"NET_ADMIN"},
							},
							Privileged: pointer.Bool(true),
						},
						TerminationMessagePath:   "/dev/termination-log",
						TerminationMessagePolicy: corev1.TerminationMessageReadFile,
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      "vpn-seed",
								MountPath: "/srv/secrets/vpn-seed",
							},
							{
								Name:      "vpn-seed-tlsauth",
								MountPath: "/srv/secrets/tlsauth",
							},
						},
					}))
					Expect(deployment.Spec.Template.Spec.Volumes).To(ContainElements(
						corev1.Volume{
							Name: "vpn-seed",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{SecretName: secretNameVPNSeed},
							},
						},
						corev1.Volume{
							Name: "vpn-seed-tlsauth",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{SecretName: secretNameVPNSeedTLSAuth},
							},
						},
					))
				})

				It("should have the mutator sidecar container when enabled", func() {
					var (
						fqdn   = "fqdn.fqdn"
						images = Images{APIServerProxyPodWebhook: "some-image:latest"}
					)

					kapi = New(kubernetesInterface, namespace, Values{Images: images, Version: version, SNI: SNIConfig{
						PodMutatorEnabled: true,
						APIServerFQDN:     fqdn,
					}})
					kapi.SetSecrets(secrets)
					deployAndRead()

					Expect(deployment.Spec.Template.Spec.Containers).To(ContainElement(corev1.Container{
						Name:  "apiserver-proxy-pod-mutator",
						Image: images.APIServerProxyPodWebhook,
						Args: []string{
							"--apiserver-fqdn=" + fqdn,
							"--host=localhost",
							"--port=9443",
							"--cert-dir=/srv/kubernetes/apiserver",
							"--cert-name=kube-apiserver.crt",
							"--key-name=kube-apiserver.key",
						},
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("50m"),
								corev1.ResourceMemory: resource.MustParse("128M"),
							},
							Limits: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("200m"),
								corev1.ResourceMemory: resource.MustParse("500M"),
							},
						},
						VolumeMounts: []corev1.VolumeMount{{
							Name:      "kube-apiserver",
							MountPath: "/srv/kubernetes/apiserver",
						}},
					}))
					Expect(deployment.Spec.Template.Spec.Volumes).To(ContainElement(corev1.Volume{
						Name: "kube-apiserver",
						VolumeSource: corev1.VolumeSource{
							Secret: &corev1.SecretVolumeSource{SecretName: secretNameServer},
						},
					}))
				})

				Context("kube-apiserver container", func() {
					images := Images{KubeAPIServer: "some-kapi-image:latest"}

					It("should have the kube-apiserver container with the expected spec", func() {
						var (
							apiServerResources = corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("1"),
									corev1.ResourceMemory: resource.MustParse("2Gi"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("2"),
									corev1.ResourceMemory: resource.MustParse("4Gi"),
								},
							}
							admissionPlugin1                    = "foo"
							admissionPlugin2                    = "foo"
							admissionPlugins                    = []gardencorev1beta1.AdmissionPlugin{{Name: admissionPlugin1}, {Name: admissionPlugin2}}
							eventTTL                            = 2 * time.Hour
							externalHostname                    = "api.foo.bar.com"
							serviceAccountIssuer                = "issuer"
							serviceAccountMaxTokenExpiration    = time.Hour
							serviceAccountExtendTokenExpiration = false
							serviceNetworkCIDR                  = "1.2.3.4/5"
						)

						kapi = New(kubernetesInterface, namespace, Values{
							AdmissionPlugins: admissionPlugins,
							Autoscaling:      AutoscalingConfig{APIServerResources: apiServerResources},
							EventTTL:         &metav1.Duration{Duration: eventTTL},
							ExternalHostname: externalHostname,
							Images:           images,
							ServiceAccount: ServiceAccountConfig{
								Issuer:                serviceAccountIssuer,
								MaxTokenExpiration:    &metav1.Duration{Duration: serviceAccountMaxTokenExpiration},
								ExtendTokenExpiration: &serviceAccountExtendTokenExpiration,
							},
							Version: version,
							VPN:     VPNConfig{ServiceNetworkCIDR: serviceNetworkCIDR},
						})
						kapi.SetSecrets(secrets)
						deployAndRead()

						Expect(deployment.Spec.Template.Spec.Containers[0].Name).To(Equal("kube-apiserver"))
						Expect(deployment.Spec.Template.Spec.Containers[0].Image).To(Equal(images.KubeAPIServer))
						Expect(deployment.Spec.Template.Spec.Containers[0].ImagePullPolicy).To(Equal(corev1.PullIfNotPresent))
						Expect(deployment.Spec.Template.Spec.Containers[0].Command).To(ContainElements(
							"/usr/local/bin/kube-apiserver",
							"--enable-admission-plugins="+admissionPlugin1+","+admissionPlugin2,
							"--admission-control-config-file=/etc/kubernetes/admission/admission-configuration.yaml",
							"--allow-privileged=true",
							"--anonymous-auth=false",
							"--audit-log-path=/var/lib/audit.log",
							"--audit-policy-file=/etc/kubernetes/audit/audit-policy.yaml",
							"--audit-log-maxsize=100",
							"--audit-log-maxbackup=5",
							"--authorization-mode=Node,RBAC",
							"--client-ca-file=/srv/kubernetes/ca/ca.crt",
							"--enable-aggregator-routing=true",
							"--enable-bootstrap-token-auth=true",
							"--http2-max-streams-per-connection=1000",
							"--etcd-cafile=/srv/kubernetes/etcd/ca/ca.crt",
							"--etcd-certfile=/srv/kubernetes/etcd/client/tls.crt",
							"--etcd-keyfile=/srv/kubernetes/etcd/client/tls.key",
							"--etcd-servers=https://etcd-main-client:2379",
							"--etcd-servers-overrides=/events#https://etcd-events-client:2379",
							"--encryption-provider-config=/etc/kubernetes/etcd-encryption-secret/encryption-configuration.yaml",
							"--event-ttl="+eventTTL.String(),
							"--external-hostname="+externalHostname,
							"--insecure-port=0",
							"--kubelet-preferred-address-types=InternalIP,Hostname,ExternalIP",
							"--kubelet-client-certificate=/srv/kubernetes/apiserver-kubelet/kube-apiserver-kubelet.crt",
							"--kubelet-client-key=/srv/kubernetes/apiserver-kubelet/kube-apiserver-kubelet.key",
							"--livez-grace-period=1m",
							"--shutdown-delay-duration=15s",
							"--profiling=false",
							"--proxy-client-cert-file=/srv/kubernetes/aggregator/kube-aggregator.crt",
							"--proxy-client-key-file=/srv/kubernetes/aggregator/kube-aggregator.key",
							"--requestheader-client-ca-file=/srv/kubernetes/ca-front-proxy/ca.crt",
							"--requestheader-extra-headers-prefix=X-Remote-Extra-",
							"--requestheader-group-headers=X-Remote-Group",
							"--requestheader-username-headers=X-Remote-User",
							"--secure-port=443",
							"--service-cluster-ip-range="+serviceNetworkCIDR,
							"--service-account-issuer="+serviceAccountIssuer,
							"--service-account-max-token-expiration="+serviceAccountMaxTokenExpiration.String(),
							"--service-account-extend-token-expiration="+strconv.FormatBool(serviceAccountExtendTokenExpiration),
							"--service-account-key-file=/srv/kubernetes/service-account-key/id_rsa",
							"--service-account-signing-key-file=/srv/kubernetes/service-account-key/id_rsa",
							"--token-auth-file=/srv/kubernetes/token/static_tokens.csv",
							"--tls-cert-file=/srv/kubernetes/apiserver/kube-apiserver.crt",
							"--tls-private-key-file=/srv/kubernetes/apiserver/kube-apiserver.key",
							"--tls-cipher-suites=TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,TLS_AES_128_GCM_SHA256,TLS_AES_256_GCM_SHA384,TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384",
							"--v=2",
						))
						Expect(deployment.Spec.Template.Spec.Containers[0].TerminationMessagePath).To(Equal("/dev/termination-log"))
						Expect(deployment.Spec.Template.Spec.Containers[0].TerminationMessagePolicy).To(Equal(corev1.TerminationMessageReadFile))
						Expect(deployment.Spec.Template.Spec.Containers[0].Ports).To(ConsistOf(corev1.ContainerPort{
							Name:          "https",
							ContainerPort: int32(443),
							Protocol:      corev1.ProtocolTCP,
						}))
						Expect(deployment.Spec.Template.Spec.Containers[0].Resources).To(Equal(apiServerResources))
						Expect(deployment.Spec.Template.Spec.Containers[0].VolumeMounts).To(ContainElements(
							corev1.VolumeMount{
								Name:      "audit-policy-config",
								MountPath: "/etc/kubernetes/audit",
							},
							corev1.VolumeMount{
								Name:      "kube-apiserver-admission-config",
								MountPath: "/etc/kubernetes/admission",
							},
							corev1.VolumeMount{
								Name:      "ca",
								MountPath: "/srv/kubernetes/ca",
							},
							corev1.VolumeMount{
								Name:      "ca-etcd",
								MountPath: "/srv/kubernetes/etcd/ca",
							},
							corev1.VolumeMount{
								Name:      "ca-front-proxy",
								MountPath: "/srv/kubernetes/ca-front-proxy",
							},
							corev1.VolumeMount{
								Name:      "etcd-client-tls",
								MountPath: "/srv/kubernetes/etcd/client",
							},
							corev1.VolumeMount{
								Name:      "kube-apiserver",
								MountPath: "/srv/kubernetes/apiserver",
							},
							corev1.VolumeMount{
								Name:      "service-account-key",
								MountPath: "/srv/kubernetes/service-account-key",
							},
							corev1.VolumeMount{
								Name:      "static-token",
								MountPath: "/srv/kubernetes/token",
							},
							corev1.VolumeMount{
								Name:      "kube-apiserver-kubelet",
								MountPath: "/srv/kubernetes/apiserver-kubelet",
							},
							corev1.VolumeMount{
								Name:      "kube-aggregator",
								MountPath: "/srv/kubernetes/aggregator",
							},
							corev1.VolumeMount{
								Name:      "etcd-encryption-secret",
								MountPath: "/etc/kubernetes/etcd-encryption-secret",
								ReadOnly:  true,
							},
						))
						Expect(deployment.Spec.Template.Spec.Volumes).To(ContainElements(
							corev1.Volume{
								Name: "audit-policy-config",
								VolumeSource: corev1.VolumeSource{
									ConfigMap: &corev1.ConfigMapVolumeSource{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: "audit-policy-config-f5b578b4",
										},
									},
								},
							},
							corev1.Volume{
								Name: "kube-apiserver-admission-config",
								VolumeSource: corev1.VolumeSource{
									ConfigMap: &corev1.ConfigMapVolumeSource{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: "kube-apiserver-admission-config-e38ff146",
										},
									},
								},
							},
							corev1.Volume{
								Name: "ca",
								VolumeSource: corev1.VolumeSource{
									Secret: &corev1.SecretVolumeSource{
										SecretName: secretNameCA,
									},
								},
							},
							corev1.Volume{
								Name: "ca-etcd",
								VolumeSource: corev1.VolumeSource{
									Secret: &corev1.SecretVolumeSource{
										SecretName: secretNameCAEtcd,
									},
								},
							},
							corev1.Volume{
								Name: "ca-front-proxy",
								VolumeSource: corev1.VolumeSource{
									Secret: &corev1.SecretVolumeSource{
										SecretName: secretNameCAFrontProxy,
									},
								},
							},
							corev1.Volume{
								Name: "etcd-client-tls",
								VolumeSource: corev1.VolumeSource{
									Secret: &corev1.SecretVolumeSource{
										SecretName: secretNameEtcd,
									},
								},
							},
							corev1.Volume{
								Name: "service-account-key",
								VolumeSource: corev1.VolumeSource{
									Secret: &corev1.SecretVolumeSource{
										SecretName: secretNameServiceAccountKey,
									},
								},
							},
							corev1.Volume{
								Name: "static-token",
								VolumeSource: corev1.VolumeSource{
									Secret: &corev1.SecretVolumeSource{
										SecretName: secretNameStaticToken,
									},
								},
							},
							corev1.Volume{
								Name: "kube-apiserver-kubelet",
								VolumeSource: corev1.VolumeSource{
									Secret: &corev1.SecretVolumeSource{
										SecretName: secretNameKubeAPIServerToKubelet,
									},
								},
							},
							corev1.Volume{
								Name: "kube-aggregator",
								VolumeSource: corev1.VolumeSource{
									Secret: &corev1.SecretVolumeSource{
										SecretName: secretNameKubeAggregator,
									},
								},
							},
							corev1.Volume{
								Name: "etcd-encryption-secret",
								VolumeSource: corev1.VolumeSource{
									Secret: &corev1.SecretVolumeSource{
										SecretName: secretNameEtcdEncryptionConfig,
									},
								},
							},
							corev1.Volume{
								Name: "kube-apiserver",
								VolumeSource: corev1.VolumeSource{
									Secret: &corev1.SecretVolumeSource{
										SecretName: secretNameServer,
									},
								},
							},
						))
					})

					It("should use the hyperkube binary if k8s < 1.17", func() {
						kapi = New(kubernetesInterface, namespace, Values{Images: images, Version: semver.MustParse("1.16.9")})
						kapi.SetSecrets(secrets)
						deployAndRead()

						Expect(deployment.Spec.Template.Spec.Containers[0].Command[0]).To(Equal("/hyperkube"))
						Expect(deployment.Spec.Template.Spec.Containers[0].Command[1]).To(Equal("kube-apiserver"))
					})

					It("should properly set the anonymous auth flag if enabled", func() {
						kapi = New(kubernetesInterface, namespace, Values{AnonymousAuthenticationEnabled: true, Images: images, Version: version})
						kapi.SetSecrets(secrets)
						deployAndRead()

						Expect(deployment.Spec.Template.Spec.Containers[0].Command).To(ContainElement(ContainSubstring(
							"--anonymous-auth=true",
						)))
					})

					It("should configure the advertise address if SNI is enabled", func() {
						advertiseAddress := "1.2.3.4"

						kapi = New(kubernetesInterface, namespace, Values{SNI: SNIConfig{Enabled: true, AdvertiseAddress: advertiseAddress}, Images: images, Version: version})
						kapi.SetSecrets(secrets)
						deployAndRead()

						Expect(deployment.Spec.Template.Spec.Containers[0].Command).To(ContainElement(
							"--advertise-address=" + advertiseAddress,
						))
					})

					It("should not configure the advertise address if SNI is enabled", func() {
						kapi = New(kubernetesInterface, namespace, Values{SNI: SNIConfig{Enabled: false, AdvertiseAddress: "foo"}, Images: images, Version: version})
						kapi.SetSecrets(secrets)
						deployAndRead()

						Expect(deployment.Spec.Template.Spec.Containers[0].Command).NotTo(ContainElement(ContainSubstring("--advertise-address=")))
					})

					It("should configure the api audiences if provided", func() {
						var (
							apiAudience1 = "foo"
							apiAudience2 = "bar"
							apiAudiences = []string{apiAudience1, apiAudience2}
						)

						kapi = New(kubernetesInterface, namespace, Values{APIAudiences: apiAudiences, Images: images, Version: version})
						kapi.SetSecrets(secrets)
						deployAndRead()

						Expect(deployment.Spec.Template.Spec.Containers[0].Command).To(ContainElement(
							"--api-audiences=" + apiAudience1 + "," + apiAudience2,
						))
					})

					It("should not configure the api audiences if not provided", func() {
						deployAndRead()

						Expect(deployment.Spec.Template.Spec.Containers[0].Command).NotTo(ContainElement(ContainSubstring("--api-audiences=")))
					})

					It("should configure the feature gates if provided", func() {
						featureGates := map[string]bool{"Foo": true, "Bar": false}

						kapi = New(kubernetesInterface, namespace, Values{FeatureGates: featureGates, Images: images, Version: version})
						kapi.SetSecrets(secrets)
						deployAndRead()

						Expect(deployment.Spec.Template.Spec.Containers[0].Command).To(ContainElement(
							"--feature-gates=Bar=false,Foo=true",
						))
					})

					It("should not configure the feature gates if not provided", func() {
						deployAndRead()

						Expect(deployment.Spec.Template.Spec.Containers[0].Command).NotTo(ContainElement(ContainSubstring("--feature-gates=")))
					})

					It("should not have the flags for improved start-up/shut-down behaviour if k8s < 1.16", func() {
						kapi = New(kubernetesInterface, namespace, Values{Images: images, Version: semver.MustParse("1.15.8")})
						kapi.SetSecrets(secrets)
						deployAndRead()

						Expect(deployment.Spec.Template.Spec.Containers[0].Command).NotTo(ContainElements(
							ContainSubstring("--livez-grace-period="),
							ContainSubstring("--shutdown-delay-duration="),
						))
					})

					It("should configure the request settings if provided", func() {
						requests := &gardencorev1beta1.KubeAPIServerRequests{
							MaxNonMutatingInflight: pointer.Int32(123),
							MaxMutatingInflight:    pointer.Int32(456),
						}

						kapi = New(kubernetesInterface, namespace, Values{Requests: requests, Images: images, Version: version})
						kapi.SetSecrets(secrets)
						deployAndRead()

						Expect(deployment.Spec.Template.Spec.Containers[0].Command).To(ContainElements(
							"--max-requests-inflight=123",
							"--max-mutating-requests-inflight=456",
						))
					})

					It("should not configure the request settings if not provided", func() {
						deployAndRead()

						Expect(deployment.Spec.Template.Spec.Containers[0].Command).NotTo(ContainElements(
							ContainSubstring("--max-requests-inflight="),
							ContainSubstring("--max-mutating-requests-inflight="),
						))
					})

					It("should configure the runtime config if provided", func() {
						runtimeConfig := map[string]bool{"foo": true, "bar": false}

						kapi = New(kubernetesInterface, namespace, Values{RuntimeConfig: runtimeConfig, Images: images, Version: version})
						kapi.SetSecrets(secrets)
						deployAndRead()

						Expect(deployment.Spec.Template.Spec.Containers[0].Command).To(ContainElement(
							"--runtime-config=bar=false,foo=true",
						))
					})

					It("should not configure the runtime config if not provided", func() {
						deployAndRead()

						Expect(deployment.Spec.Template.Spec.Containers[0].Command).NotTo(ContainElement(ContainSubstring("--runtime-config=")))
					})

					It("should configure the watch cache settings if provided", func() {
						watchCacheSizes := &gardencorev1beta1.WatchCacheSizes{
							Default: pointer.Int32(123),
							Resources: []gardencorev1beta1.ResourceWatchCacheSize{
								{Resource: "foo", CacheSize: 456},
								{Resource: "bar", CacheSize: 789, APIGroup: pointer.String("baz")},
							},
						}

						kapi = New(kubernetesInterface, namespace, Values{WatchCacheSizes: watchCacheSizes, Images: images, Version: version})
						kapi.SetSecrets(secrets)
						deployAndRead()

						Expect(deployment.Spec.Template.Spec.Containers[0].Command).To(ContainElements(
							"--default-watch-cache-size=123",
							"--watch-cache-sizes=foo#456,bar.baz#789",
						))
					})

					It("should not configure the watch cache settings if not provided", func() {
						deployAndRead()

						Expect(deployment.Spec.Template.Spec.Containers[0].Command).NotTo(ContainElements(
							ContainSubstring("--default-watch-cache-size="),
							ContainSubstring("--watch-cache-sizes="),
						))
					})

					It("should mount the host pki directories if k8s >= 1.17", func() {
						directoryOrCreate := corev1.HostPathDirectoryOrCreate

						kapi = New(kubernetesInterface, namespace, Values{Images: images, Version: version})
						kapi.SetSecrets(secrets)
						deployAndRead()

						Expect(deployment.Spec.Template.Spec.Containers[0].VolumeMounts).To(ContainElements(
							corev1.VolumeMount{
								Name:      "fedora-rhel6-openelec-cabundle",
								MountPath: "/etc/pki/tls",
								ReadOnly:  true,
							},
							corev1.VolumeMount{
								Name:      "centos-rhel7-cabundle",
								MountPath: "/etc/pki/ca-trust/extracted/pem",
								ReadOnly:  true,
							},
							corev1.VolumeMount{
								Name:      "etc-ssl",
								MountPath: "/etc/ssl",
								ReadOnly:  true,
							},
							corev1.VolumeMount{
								Name:      "usr-share-cacerts",
								MountPath: "/usr/share/ca-certificates",
								ReadOnly:  true,
							},
						))

						Expect(deployment.Spec.Template.Spec.Volumes).To(ContainElements(
							corev1.Volume{
								Name: "fedora-rhel6-openelec-cabundle",
								VolumeSource: corev1.VolumeSource{
									HostPath: &corev1.HostPathVolumeSource{
										Path: "/etc/pki/tls",
										Type: &directoryOrCreate,
									},
								},
							},
							corev1.Volume{
								Name: "centos-rhel7-cabundle",
								VolumeSource: corev1.VolumeSource{
									HostPath: &corev1.HostPathVolumeSource{
										Path: "/etc/pki/ca-trust/extracted/pem",
										Type: &directoryOrCreate,
									},
								},
							},
							corev1.Volume{
								Name: "etc-ssl",
								VolumeSource: corev1.VolumeSource{
									HostPath: &corev1.HostPathVolumeSource{
										Path: "/etc/ssl",
										Type: &directoryOrCreate,
									},
								},
							},
							corev1.Volume{
								Name: "usr-share-cacerts",
								VolumeSource: corev1.VolumeSource{
									HostPath: &corev1.HostPathVolumeSource{
										Path: "/usr/share/ca-certificates",
										Type: &directoryOrCreate,
									},
								},
							},
						))
					})

					It("should not mount the host pki directories if k8s < 1.17", func() {
						kapi = New(kubernetesInterface, namespace, Values{Images: images, Version: semver.MustParse("1.16.9")})
						kapi.SetSecrets(secrets)
						deployAndRead()

						for _, name := range []string{"fedora-rhel6-openelec-cabundle", "centos-rhel7-cabundle", "etc-ssl", "usr-share-cacerts"} {
							Expect(deployment.Spec.Template.Spec.Containers[0].VolumeMounts).NotTo(ContainElement(MatchFields(IgnoreExtras, Fields{"Name": Equal(name)})))
							Expect(deployment.Spec.Template.Spec.Volumes).NotTo(ContainElement(MatchFields(IgnoreExtras, Fields{"Name": Equal(name)})))
						}
					})

					It("should properly configure the settings related to the basic auth secret if enabled", func() {
						kapi = New(kubernetesInterface, namespace, Values{Images: images, Version: version, BasicAuthenticationEnabled: true})
						kapi.SetSecrets(secrets)
						deployAndRead()

						Expect(deployment.Spec.Template.Spec.Containers[0].Command).To(ContainElement(
							"--basic-auth-file=/srv/kubernetes/auth/basic_auth.csv",
						))

						Expect(deployment.Spec.Template.Spec.Containers[0].VolumeMounts).To(ContainElement(corev1.VolumeMount{
							Name:      "kube-apiserver-basic-auth",
							MountPath: "/srv/kubernetes/auth",
						}))

						Expect(deployment.Spec.Template.Spec.Volumes).To(ContainElement(corev1.Volume{
							Name: "kube-apiserver-basic-auth",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: secretNameBasicAuthentication,
								},
							},
						}))
					})

					It("should not configure the settings related to the basic auth secret if disabled", func() {
						kapi = New(kubernetesInterface, namespace, Values{Images: images, Version: version, BasicAuthenticationEnabled: false})
						kapi.SetSecrets(secrets)
						deployAndRead()

						Expect(deployment.Spec.Template.Spec.Containers[0].Command).NotTo(ContainElement(ContainSubstring("--basic-auth-file=")))
						Expect(deployment.Spec.Template.Spec.Containers[0].VolumeMounts).NotTo(ContainElement(MatchFields(IgnoreExtras, Fields{"Name": Equal("kube-apiserver-basic-auth")})))
						Expect(deployment.Spec.Template.Spec.Volumes).NotTo(ContainElement(MatchFields(IgnoreExtras, Fields{"Name": Equal("kube-apiserver-basic-auth")})))
					})

					It("should properly configure the settings related to reversed vpn if enabled", func() {
						kapi = New(kubernetesInterface, namespace, Values{Images: images, Version: version, VPN: VPNConfig{ReversedVPNEnabled: true}})
						kapi.SetSecrets(secrets)
						deployAndRead()

						Expect(deployment.Spec.Template.Spec.Containers[0].Command).To(ContainElement(
							"--egress-selector-config-file=/etc/kubernetes/egress/egress-selector-configuration.yaml",
						))

						Expect(deployment.Spec.Template.Spec.Containers[0].VolumeMounts).To(ContainElements(
							corev1.VolumeMount{
								Name:      "kube-apiserver-http-proxy",
								MountPath: "/etc/srv/kubernetes/envoy",
							},
							corev1.VolumeMount{
								Name:      "egress-selection-config",
								MountPath: "/etc/kubernetes/egress",
							},
						))

						Expect(deployment.Spec.Template.Spec.Volumes).To(ContainElements(
							corev1.Volume{
								Name: "kube-apiserver-http-proxy",
								VolumeSource: corev1.VolumeSource{
									Secret: &corev1.SecretVolumeSource{
										SecretName: secretNameHTTPProxy,
									},
								},
							},
							corev1.Volume{
								Name: "egress-selection-config",
								VolumeSource: corev1.VolumeSource{
									ConfigMap: &corev1.ConfigMapVolumeSource{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: "kube-apiserver-egress-selector-config-6a97058a",
										},
									},
								},
							},
						))
					})

					It("should not configure the settings related to reversed vpn if disabled", func() {
						kapi = New(kubernetesInterface, namespace, Values{Images: images, Version: version, VPN: VPNConfig{ReversedVPNEnabled: false}})
						kapi.SetSecrets(secrets)
						deployAndRead()

						Expect(deployment.Spec.Template.Spec.Containers[0].Command).NotTo(ContainElement(ContainSubstring("--egress-selector-config-file=")))
						Expect(deployment.Spec.Template.Spec.Containers[0].VolumeMounts).NotTo(ContainElement(MatchFields(IgnoreExtras, Fields{"Name": Equal("kube-apiserver-http-proxy")})))
						Expect(deployment.Spec.Template.Spec.Volumes).NotTo(ContainElement(MatchFields(IgnoreExtras, Fields{"Name": Equal("kube-apiserver-http-proxy")})))
					})

					It("should properly configure the settings related to oidc if enabled", func() {
						oidc := &gardencorev1beta1.OIDCConfig{
							IssuerURL:      pointer.String("someurl"),
							ClientID:       pointer.String("clientid"),
							CABundle:       pointer.String(""),
							UsernameClaim:  pointer.String("usernameclaim"),
							GroupsClaim:    pointer.String("groupsclaim"),
							UsernamePrefix: pointer.String("usernameprefix"),
							GroupsPrefix:   pointer.String("groupsprefix"),
							SigningAlgs:    []string{"foo", "bar"},
							RequiredClaims: map[string]string{"one": "two", "three": "four"},
						}

						kapi = New(kubernetesInterface, namespace, Values{Images: images, Version: version, OIDC: oidc})
						kapi.SetSecrets(secrets)
						deployAndRead()

						Expect(deployment.Spec.Template.Spec.Containers[0].Command).To(ContainElements(
							"--oidc-issuer-url="+*oidc.IssuerURL,
							"--oidc-client-id="+*oidc.ClientID,
							"--oidc-ca-file=/srv/kubernetes/oidc/ca.crt",
							"--oidc-username-claim="+*oidc.UsernameClaim,
							"--oidc-groups-claim="+*oidc.GroupsClaim,
							"--oidc-username-prefix="+*oidc.UsernamePrefix,
							"--oidc-groups-prefix="+*oidc.GroupsPrefix,
							"--oidc-signing-algs=foo,bar",
							"--oidc-required-claim=one=two",
							"--oidc-required-claim=three=four",
						))

						Expect(deployment.Spec.Template.Spec.Containers[0].VolumeMounts).To(ContainElement(corev1.VolumeMount{
							Name:      "kube-apiserver-oidc-cabundle",
							MountPath: "/srv/kubernetes/oidc",
						}))

						Expect(deployment.Spec.Template.Spec.Volumes).To(ContainElement(corev1.Volume{
							Name: "kube-apiserver-oidc-cabundle",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: "kube-apiserver-oidc-cabundle-cd372fb8",
								},
							},
						}))
					})

					It("should not configure the settings related to oidc if disabled", func() {
						deployAndRead()

						Expect(deployment.Spec.Template.Spec.Containers[0].VolumeMounts).NotTo(ContainElement(MatchFields(IgnoreExtras, Fields{"Name": Equal("kube-apiserver-oidc-cabundle")})))
						Expect(deployment.Spec.Template.Spec.Volumes).NotTo(ContainElement(MatchFields(IgnoreExtras, Fields{"Name": Equal("kube-apiserver-oidc-cabundle")})))
					})

					It("should properly configure the settings related ot the service account signing key if provided", func() {
						kapi = New(kubernetesInterface, namespace, Values{Images: images, Version: version, ServiceAccount: ServiceAccountConfig{SigningKey: []byte("")}})
						kapi.SetSecrets(secrets)
						deployAndRead()

						Expect(deployment.Spec.Template.Spec.Containers[0].Command).To(ContainElements(
							"--service-account-signing-key-file=/srv/kubernetes/service-account-signing-key/signing-key",
							"--service-account-key-file=/srv/kubernetes/service-account-signing-key/signing-key",
						))

						Expect(deployment.Spec.Template.Spec.Containers[0].VolumeMounts).To(ContainElement(corev1.VolumeMount{
							Name:      "kube-apiserver-service-account-signing-key",
							MountPath: "/srv/kubernetes/service-account-signing-key",
						}))

						Expect(deployment.Spec.Template.Spec.Volumes).To(ContainElement(corev1.Volume{
							Name: "kube-apiserver-service-account-signing-key",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: "kube-apiserver-sa-signing-key-cd372fb8",
								},
							},
						}))
					})

					It("should not configure the settings related to the service account signing key if not provided", func() {
						deployAndRead()

						Expect(deployment.Spec.Template.Spec.Containers[0].VolumeMounts).NotTo(ContainElement(MatchFields(IgnoreExtras, Fields{"Name": Equal("kube-apiserver-oidc-cabundle")})))
						Expect(deployment.Spec.Template.Spec.Volumes).NotTo(ContainElement(MatchFields(IgnoreExtras, Fields{"Name": Equal("kube-apiserver-oidc-cabundle")})))
					})

					It("should have the proper probes for k8s >= 1.16", func() {
						probeToken := "1234"

						kapi = New(kubernetesInterface, namespace, Values{Images: images, Version: semver.MustParse("1.16.9"), ProbeToken: probeToken})
						kapi.SetSecrets(secrets)
						deployAndRead()

						Expect(deployment.Spec.Template.Spec.Containers[0].LivenessProbe).To(Equal(&corev1.Probe{
							Handler: corev1.Handler{
								HTTPGet: &corev1.HTTPGetAction{
									Path:   "/livez",
									Scheme: corev1.URISchemeHTTPS,
									Port:   intstr.FromInt(Port),
									HTTPHeaders: []corev1.HTTPHeader{{
										Name:  "Authorization",
										Value: "Bearer " + probeToken,
									}},
								},
							},
							SuccessThreshold:    1,
							FailureThreshold:    3,
							InitialDelaySeconds: 15,
							PeriodSeconds:       10,
							TimeoutSeconds:      15,
						}))
						Expect(deployment.Spec.Template.Spec.Containers[0].ReadinessProbe).To(Equal(&corev1.Probe{
							Handler: corev1.Handler{
								HTTPGet: &corev1.HTTPGetAction{
									Path:   "/readyz",
									Scheme: corev1.URISchemeHTTPS,
									Port:   intstr.FromInt(Port),
									HTTPHeaders: []corev1.HTTPHeader{{
										Name:  "Authorization",
										Value: "Bearer " + probeToken,
									}},
								},
							},
							SuccessThreshold:    1,
							FailureThreshold:    3,
							InitialDelaySeconds: 10,
							PeriodSeconds:       10,
							TimeoutSeconds:      15,
						}))
					})

					It("should have the proper probes for k8s < 1.16", func() {
						probeToken := "1234"

						kapi = New(kubernetesInterface, namespace, Values{Images: images, Version: semver.MustParse("1.15.9"), ProbeToken: probeToken})
						kapi.SetSecrets(secrets)
						deployAndRead()

						Expect(deployment.Spec.Template.Spec.Containers[0].LivenessProbe).To(Equal(&corev1.Probe{
							Handler: corev1.Handler{
								HTTPGet: &corev1.HTTPGetAction{
									Path:   "/healthz",
									Scheme: corev1.URISchemeHTTPS,
									Port:   intstr.FromInt(Port),
									HTTPHeaders: []corev1.HTTPHeader{{
										Name:  "Authorization",
										Value: "Bearer " + probeToken,
									}},
								},
							},
							SuccessThreshold:    1,
							FailureThreshold:    3,
							InitialDelaySeconds: 15,
							PeriodSeconds:       10,
							TimeoutSeconds:      15,
						}))
						Expect(deployment.Spec.Template.Spec.Containers[0].ReadinessProbe).To(Equal(&corev1.Probe{
							Handler: corev1.Handler{
								HTTPGet: &corev1.HTTPGetAction{
									Path:   "/healthz",
									Scheme: corev1.URISchemeHTTPS,
									Port:   intstr.FromInt(Port),
									HTTPHeaders: []corev1.HTTPHeader{{
										Name:  "Authorization",
										Value: "Bearer " + probeToken,
									}},
								},
							},
							SuccessThreshold:    1,
							FailureThreshold:    3,
							InitialDelaySeconds: 10,
							PeriodSeconds:       10,
							TimeoutSeconds:      15,
						}))
					})

					It("should have no lifecycle settings if k8s >= 1.16", func() {
						deployAndRead()

						Expect(deployment.Spec.Template.Spec.Containers[0].Lifecycle).To(BeNil())
					})

					It("should have no lifecycle settings if k8s < 1.16", func() {
						kapi = New(kubernetesInterface, namespace, Values{Images: images, Version: semver.MustParse("1.15.9")})
						kapi.SetSecrets(secrets)
						deployAndRead()

						Expect(deployment.Spec.Template.Spec.Containers[0].Lifecycle).To(Equal(&corev1.Lifecycle{
							PreStop: &corev1.Handler{
								Exec: &corev1.ExecAction{
									Command: []string{"sh", "-c", "sleep 5"},
								},
							},
						}))
					})
				})
			})
		})
	})

	Describe("#Destroy", func() {
		It("should successfully delete all expected resources", func() {
			Expect(c.Create(ctx, deployment)).To(Succeed())
			Expect(c.Create(ctx, horizontalPodAutoscaler)).To(Succeed())
			Expect(c.Create(ctx, verticalPodAutoscaler)).To(Succeed())
			Expect(c.Create(ctx, hvpa)).To(Succeed())
			Expect(c.Create(ctx, podDisruptionBudget)).To(Succeed())
			Expect(c.Create(ctx, networkPolicyAllowFromShootAPIServer)).To(Succeed())
			Expect(c.Create(ctx, networkPolicyAllowToShootAPIServer)).To(Succeed())
			Expect(c.Create(ctx, networkPolicyAllowKubeAPIServer)).To(Succeed())
			Expect(c.Create(ctx, managedResourceSecret)).To(Succeed())
			Expect(c.Create(ctx, managedResource)).To(Succeed())

			Expect(c.Get(ctx, client.ObjectKeyFromObject(deployment), deployment)).To(Succeed())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(horizontalPodAutoscaler), horizontalPodAutoscaler)).To(Succeed())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(verticalPodAutoscaler), verticalPodAutoscaler)).To(Succeed())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(hvpa), hvpa)).To(Succeed())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(podDisruptionBudget), podDisruptionBudget)).To(Succeed())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(networkPolicyAllowFromShootAPIServer), networkPolicyAllowFromShootAPIServer)).To(Succeed())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(networkPolicyAllowToShootAPIServer), networkPolicyAllowToShootAPIServer)).To(Succeed())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(networkPolicyAllowKubeAPIServer), networkPolicyAllowKubeAPIServer)).To(Succeed())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(Succeed())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())

			Expect(kapi.Destroy(ctx)).To(Succeed())

			Expect(c.Get(ctx, client.ObjectKeyFromObject(deployment), deployment)).To(MatchError(apierrors.NewNotFound(schema.GroupResource{Group: appsv1.SchemeGroupVersion.Group, Resource: "deployments"}, deployment.Name)))
			Expect(c.Get(ctx, client.ObjectKeyFromObject(horizontalPodAutoscaler), horizontalPodAutoscaler)).To(MatchError(apierrors.NewNotFound(schema.GroupResource{Group: autoscalingv2beta1.SchemeGroupVersion.Group, Resource: "horizontalpodautoscalers"}, horizontalPodAutoscaler.Name)))
			Expect(c.Get(ctx, client.ObjectKeyFromObject(verticalPodAutoscaler), verticalPodAutoscaler)).To(MatchError(apierrors.NewNotFound(schema.GroupResource{Group: autoscalingv1beta2.SchemeGroupVersion.Group, Resource: "verticalpodautoscalers"}, verticalPodAutoscaler.Name)))
			Expect(c.Get(ctx, client.ObjectKeyFromObject(hvpa), hvpa)).To(MatchError(apierrors.NewNotFound(schema.GroupResource{Group: hvpav1alpha1.SchemeGroupVersionHvpa.Group, Resource: "hvpas"}, hvpa.Name)))
			Expect(c.Get(ctx, client.ObjectKeyFromObject(podDisruptionBudget), podDisruptionBudget)).To(MatchError(apierrors.NewNotFound(schema.GroupResource{Group: policyv1beta1.SchemeGroupVersion.Group, Resource: "poddisruptionbudgets"}, podDisruptionBudget.Name)))
			Expect(c.Get(ctx, client.ObjectKeyFromObject(networkPolicyAllowFromShootAPIServer), networkPolicyAllowFromShootAPIServer)).To(MatchError(apierrors.NewNotFound(schema.GroupResource{Group: networkingv1.SchemeGroupVersion.Group, Resource: "networkpolicies"}, networkPolicyAllowFromShootAPIServer.Name)))
			Expect(c.Get(ctx, client.ObjectKeyFromObject(networkPolicyAllowToShootAPIServer), networkPolicyAllowToShootAPIServer)).To(MatchError(apierrors.NewNotFound(schema.GroupResource{Group: networkingv1.SchemeGroupVersion.Group, Resource: "networkpolicies"}, networkPolicyAllowToShootAPIServer.Name)))
			Expect(c.Get(ctx, client.ObjectKeyFromObject(networkPolicyAllowKubeAPIServer), networkPolicyAllowKubeAPIServer)).To(MatchError(apierrors.NewNotFound(schema.GroupResource{Group: networkingv1.SchemeGroupVersion.Group, Resource: "networkpolicies"}, networkPolicyAllowKubeAPIServer.Name)))
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(MatchError(apierrors.NewNotFound(schema.GroupResource{Group: corev1.SchemeGroupVersion.Group, Resource: "secrets"}, managedResourceSecret.Name)))
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(MatchError(apierrors.NewNotFound(schema.GroupResource{Group: resourcesv1alpha1.SchemeGroupVersion.Group, Resource: "managedresources"}, managedResource.Name)))
		})
	})

	Describe("#Wait", func() {
		It("should successfully wait for the deployment to be ready", func() {
			fakeClient := fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
			fakeKubernetesInterface := fakekubernetes.NewClientSetBuilder().WithAPIReader(fakeClient).WithClient(fakeClient).Build()
			kapi = New(fakeKubernetesInterface, namespace, Values{})
			deploy := deployment.DeepCopy()

			defer test.WithVars(&IntervalWaitForDeployment, time.Millisecond)()
			defer test.WithVars(&TimeoutWaitForDeployment, 100*time.Millisecond)()

			Expect(fakeClient.Create(ctx, deploy)).To(Succeed())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(deploy), deploy)).To(Succeed())

			timer := time.AfterFunc(10*time.Millisecond, func() {
				deploy.Generation = 24
				deploy.Status.ObservedGeneration = deploy.Generation
				deploy.Spec.Replicas = pointer.Int32(4)
				deploy.Status.Replicas = *deploy.Spec.Replicas
				deploy.Status.UpdatedReplicas = *deploy.Spec.Replicas
				deploy.Status.AvailableReplicas = *deploy.Spec.Replicas
				Expect(fakeClient.Update(ctx, deploy)).To(Succeed())
			})
			defer timer.Stop()

			Expect(kapi.Wait(ctx)).To(Succeed())
		})

		It("should fail while waiting for the deployment to be ready due to outdated generation", func() {
			defer test.WithVars(&IntervalWaitForDeployment, time.Millisecond)()
			defer test.WithVars(&TimeoutWaitForDeployment, 10*time.Millisecond)()

			deployment.Generation = 24
			deployment.Status.ObservedGeneration = deployment.Generation - 1
			Expect(c.Create(ctx, deployment)).To(Succeed())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(deployment), deployment)).To(Succeed())

			Expect(kapi.Wait(ctx)).To(MatchError(ContainSubstring("not observed at latest generation")))
		})

		It("should fail while waiting for the deployment to be ready due to outdated replicas field", func() {
			defer test.WithVars(&IntervalWaitForDeployment, time.Millisecond)()
			defer test.WithVars(&TimeoutWaitForDeployment, 10*time.Millisecond)()

			deployment.Generation = 24
			deployment.Status.ObservedGeneration = deployment.Generation
			deployment.Spec.Replicas = pointer.Int32(4)
			deployment.Status.Replicas = *deployment.Spec.Replicas - 1
			deployment.Status.UpdatedReplicas = *deployment.Spec.Replicas
			Expect(c.Create(ctx, deployment)).To(Succeed())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(deployment), deployment)).To(Succeed())

			Expect(kapi.Wait(ctx)).To(MatchError(ContainSubstring("has outdated replicas")))
		})

		It("should fail while waiting for the deployment to be ready due to outdated updatedReplicas field", func() {
			defer test.WithVars(&IntervalWaitForDeployment, time.Millisecond)()
			defer test.WithVars(&TimeoutWaitForDeployment, 10*time.Millisecond)()

			deployment.Generation = 24
			deployment.Status.ObservedGeneration = deployment.Generation
			deployment.Spec.Replicas = pointer.Int32(4)
			deployment.Status.Replicas = *deployment.Spec.Replicas
			deployment.Status.UpdatedReplicas = *deployment.Spec.Replicas - 1
			Expect(c.Create(ctx, deployment)).To(Succeed())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(deployment), deployment)).To(Succeed())

			Expect(kapi.Wait(ctx)).To(MatchError(ContainSubstring("does not have enough updated replicas")))
		})

		It("should fail while waiting for the deployment to be ready due to outdated updatedReplicas field", func() {
			defer test.WithVars(&IntervalWaitForDeployment, time.Millisecond)()
			defer test.WithVars(&TimeoutWaitForDeployment, 10*time.Millisecond)()

			deployment.Generation = 24
			deployment.Status.ObservedGeneration = deployment.Generation
			deployment.Spec.Replicas = pointer.Int32(4)
			deployment.Status.Replicas = *deployment.Spec.Replicas
			deployment.Status.UpdatedReplicas = *deployment.Spec.Replicas
			deployment.Status.AvailableReplicas = *deployment.Spec.Replicas - 1
			Expect(c.Create(ctx, deployment)).To(Succeed())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(deployment), deployment)).To(Succeed())

			Expect(kapi.Wait(ctx)).To(MatchError(ContainSubstring("does not have enough available replicas")))
		})
	})

	Describe("#WaitCleanup", func() {
		It("should successfully wait for the deployment to be deleted", func() {
			fakeClient := fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
			fakeKubernetesInterface := fakekubernetes.NewClientSetBuilder().WithAPIReader(fakeClient).WithClient(fakeClient).Build()
			kapi = New(fakeKubernetesInterface, namespace, Values{})
			deploy := deployment.DeepCopy()

			defer test.WithVars(&IntervalWaitForDeployment, time.Millisecond)()
			defer test.WithVars(&TimeoutWaitForDeployment, 100*time.Millisecond)()

			Expect(fakeClient.Create(ctx, deploy)).To(Succeed())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(deploy), deploy)).To(Succeed())

			timer := time.AfterFunc(10*time.Millisecond, func() {
				Expect(fakeClient.Delete(ctx, deploy)).To(Succeed())
				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(deploy), deploy)).To(MatchError(apierrors.NewNotFound(schema.GroupResource{Group: appsv1.SchemeGroupVersion.Group, Resource: "deployments"}, deploy.Name)))
			})
			defer timer.Stop()

			Expect(kapi.WaitCleanup(ctx)).To(Succeed())
		})

		It("should time out while waiting for the deployment to be deleted", func() {
			defer test.WithVars(&IntervalWaitForDeployment, time.Millisecond)()
			defer test.WithVars(&TimeoutWaitForDeployment, 100*time.Millisecond)()

			Expect(c.Create(ctx, deployment)).To(Succeed())

			Expect(kapi.WaitCleanup(ctx)).To(MatchError(ContainSubstring("context deadline exceeded")))
		})

		It("should abort due to a severe error while waiting for the deployment to be deleted", func() {
			defer test.WithVars(&IntervalWaitForDeployment, time.Millisecond)()

			Expect(c.Create(ctx, deployment)).To(Succeed())

			scheme := runtime.NewScheme()
			clientWithoutScheme := fakeclient.NewClientBuilder().WithScheme(scheme).Build()
			kubernetesInterface2 := fakekubernetes.NewClientSetBuilder().WithClient(clientWithoutScheme).Build()
			kapi = New(kubernetesInterface2, namespace, Values{})

			Expect(runtime.IsNotRegisteredError(kapi.WaitCleanup(ctx))).To(BeTrue())
		})
	})

	Describe("#SetAutoscalingAPIServerResources", func() {
		It("should properly set the field", func() {
			v := corev1.ResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("10Mi")}}
			kapi.SetAutoscalingAPIServerResources(v)
			Expect(kapi.GetValues().Autoscaling.APIServerResources).To(Equal(v))
		})
	})

	Describe("#SetAutoscalingReplicas", func() {
		It("should properly set the field", func() {
			v := pointer.Int32(2)
			kapi.SetAutoscalingReplicas(v)
			Expect(kapi.GetValues().Autoscaling.Replicas).To(Equal(v))
		})
	})

	Describe("#SetServiceAccountConfig", func() {
		It("should properly set the field", func() {
			v := ServiceAccountConfig{Issuer: "foo"}
			kapi.SetServiceAccountConfig(v)
			Expect(kapi.GetValues().ServiceAccount).To(Equal(v))
		})
	})

	Describe("#SetSNIConfig", func() {
		It("should properly set the field", func() {
			v := SNIConfig{AdvertiseAddress: "foo"}
			kapi.SetSNIConfig(v)
			Expect(kapi.GetValues().SNI).To(Equal(v))
		})
	})

	Describe("#SetProbeToken", func() {
		It("should properly set the field", func() {
			v := "bar"
			kapi.SetProbeToken(v)
			Expect(kapi.GetValues().ProbeToken).To(Equal(v))
		})
	})

	Describe("#SetExternalHostname", func() {
		It("should properly set the field", func() {
			v := "bar"
			kapi.SetExternalHostname(v)
			Expect(kapi.GetValues().ExternalHostname).To(Equal(v))
		})
	})
})

func intOrStrPtr(intOrStr intstr.IntOrString) *intstr.IntOrString {
	return &intOrStr
}

func egressSelectorConfigFor(controlPlaneName string) string {
	return `apiVersion: apiserver.k8s.io/v1alpha1
egressSelections:
- connection:
    proxyProtocol: HTTPConnect
    transport:
      tcp:
        tlsConfig:
          caBundle: /etc/srv/kubernetes/envoy/ca.crt
          clientCert: /etc/srv/kubernetes/envoy/tls.crt
          clientKey: /etc/srv/kubernetes/envoy/tls.key
        url: https://vpn-seed-server:9443
  name: cluster
- connection:
    proxyProtocol: Direct
  name: ` + controlPlaneName + `
- connection:
    proxyProtocol: Direct
  name: etcd
kind: EgressSelectorConfiguration
`
}
