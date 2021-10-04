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

package botanist

import (
	"context"
	"net"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	fakeclientmap "github.com/gardener/gardener/pkg/client/kubernetes/clientmap/fake"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	"github.com/gardener/gardener/pkg/client/kubernetes/fake"
	"github.com/gardener/gardener/pkg/features"
	gardenletfeatures "github.com/gardener/gardener/pkg/gardenlet/features"
	"github.com/gardener/gardener/pkg/operation"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	"github.com/gardener/gardener/pkg/operation/botanist/component/kubeapiserver"
	mockkubeapiserver "github.com/gardener/gardener/pkg/operation/botanist/component/kubeapiserver/mock"
	gardenpkg "github.com/gardener/gardener/pkg/operation/garden"
	shootpkg "github.com/gardener/gardener/pkg/operation/shoot"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	"github.com/gardener/gardener/pkg/utils/test"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/component-base/featuregate"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("KubeAPIServer", func() {
	var (
		ctrl *gomock.Controller

		gc              client.Client
		k8sGardenClient kubernetes.Interface
		c               client.Client
		k8sSeedClient   kubernetes.Interface
		botanist        *Botanist
		kubeAPIServer   *mockkubeapiserver.MockInterface

		ctx                   = context.TODO()
		projectNamespace      = "garden-my-project"
		shootNamespace        = "shoot--foo--bar"
		internalClusterDomain = "foo.bar.com"
		podNetwork            *net.IPNet
		serviceNetwork        *net.IPNet
		podNetworkCIDR        = "10.0.1.0/24"
		serviceNetworkCIDR    = "10.0.2.0/24"
		nodeNetworkCIDR       = "10.0.3.0/24"
		healthCheckToken      = "some-token"
		apiServerClusterIP    = "1.2.3.4"
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())

		gc = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).Build()
		k8sGardenClient = fake.NewClientSetBuilder().WithClient(gc).Build()

		c = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		k8sSeedClient = fake.NewClientSetBuilder().WithClient(c).Build()

		var err error
		_, podNetwork, err = net.ParseCIDR(podNetworkCIDR)
		Expect(err).NotTo(HaveOccurred())
		_, serviceNetwork, err = net.ParseCIDR(serviceNetworkCIDR)
		Expect(err).NotTo(HaveOccurred())

		kubeAPIServer = mockkubeapiserver.NewMockInterface(ctrl)
		botanist = &Botanist{
			Operation: &operation.Operation{
				K8sGardenClient: k8sGardenClient,
				K8sSeedClient:   k8sSeedClient,
				Garden:          &gardenpkg.Garden{},
				Shoot: &shootpkg.Shoot{
					SeedNamespace: shootNamespace,
					Components: &shootpkg.Components{
						ControlPlane: &shootpkg.ControlPlane{
							KubeAPIServer: kubeAPIServer,
						},
					},
					InternalClusterDomain: internalClusterDomain,
					Networks: &shootpkg.Networks{
						Pods:     podNetwork,
						Services: serviceNetwork,
					},
				},
				ImageVector: imagevector.ImageVector{
					{Name: "alpine-iptables"},
					{Name: "apiserver-proxy-pod-webhook"},
					{Name: "kube-apiserver"},
					{Name: "vpn-seed"},
				},
				APIServerHealthCheckToken: healthCheckToken,
				APIServerClusterIP:        apiServerClusterIP,
			},
		}
		botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{
			Spec: gardencorev1beta1.ShootSpec{
				Networking: gardencorev1beta1.Networking{
					Nodes: &nodeNetworkCIDR,
				},
			},
		})
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#DefaultKubeAPIServer", func() {
		It("should return an error because the alpine-iptables image cannot be found", func() {
			botanist.ImageVector = imagevector.ImageVector{}

			kubeAPIServer, err := botanist.DefaultKubeAPIServer(ctx)
			Expect(kubeAPIServer).To(BeNil())
			Expect(err).To(MatchError(ContainSubstring("could not find image \"alpine-iptables\"")))
		})

		It("should return an error because the apiserver-proxy-pod-webhook cannot be found", func() {
			botanist.ImageVector = imagevector.ImageVector{{Name: "alpine-iptables"}}

			kubeAPIServer, err := botanist.DefaultKubeAPIServer(ctx)
			Expect(kubeAPIServer).To(BeNil())
			Expect(err).To(MatchError(ContainSubstring("could not find image \"apiserver-proxy-pod-webhook\"")))
		})

		It("should return an error because the vpn-seed cannot be found", func() {
			botanist.ImageVector = imagevector.ImageVector{{Name: "alpine-iptables"}, {Name: "apiserver-proxy-pod-webhook"}, {Name: "kube-apiserver"}}

			kubeAPIServer, err := botanist.DefaultKubeAPIServer(ctx)
			Expect(kubeAPIServer).To(BeNil())
			Expect(err).To(MatchError(ContainSubstring("could not find image \"vpn-seed\"")))
		})

		It("should return an error because the kube-apiserver cannot be found", func() {
			botanist.ImageVector = imagevector.ImageVector{{Name: "alpine-iptables"}, {Name: "apiserver-proxy-pod-webhook"}, {Name: "vpn-seed"}}

			kubeAPIServer, err := botanist.DefaultKubeAPIServer(ctx)
			Expect(kubeAPIServer).To(BeNil())
			Expect(err).To(MatchError(ContainSubstring("could not find image \"kube-apiserver\"")))
		})

		Describe("AnonymousAuthenticationEnabled", func() {
			It("should set the field to false by default", func() {
				kubeAPIServer, err := botanist.DefaultKubeAPIServer(ctx)
				Expect(err).NotTo(HaveOccurred())
				Expect(kubeAPIServer.GetValues().AnonymousAuthenticationEnabled).To(BeFalse())
			})

			It("should set the field to true if explicitly enabled", func() {
				botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{
					Spec: gardencorev1beta1.ShootSpec{
						Kubernetes: gardencorev1beta1.Kubernetes{
							KubeAPIServer: &gardencorev1beta1.KubeAPIServerConfig{
								EnableAnonymousAuthentication: pointer.Bool(true),
							},
						},
					},
				})

				kubeAPIServer, err := botanist.DefaultKubeAPIServer(ctx)
				Expect(err).NotTo(HaveOccurred())
				Expect(kubeAPIServer.GetValues().AnonymousAuthenticationEnabled).To(BeTrue())
			})
		})

		Describe("APIAudiences", func() {
			It("should set the field to 'kubernetes' by default", func() {
				kubeAPIServer, err := botanist.DefaultKubeAPIServer(ctx)
				Expect(err).NotTo(HaveOccurred())
				Expect(kubeAPIServer.GetValues().APIAudiences).To(ConsistOf("kubernetes"))
			})

			It("should set the field to the configured values", func() {
				apiAudiences := []string{"foo", "bar"}

				botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{
					Spec: gardencorev1beta1.ShootSpec{
						Kubernetes: gardencorev1beta1.Kubernetes{
							KubeAPIServer: &gardencorev1beta1.KubeAPIServerConfig{
								APIAudiences: apiAudiences,
							},
						},
					},
				})

				kubeAPIServer, err := botanist.DefaultKubeAPIServer(ctx)
				Expect(err).NotTo(HaveOccurred())
				Expect(kubeAPIServer.GetValues().APIAudiences).To(Equal(apiAudiences))
			})
		})

		Describe("AdmissionPlugins", func() {
			DescribeTable("should have the expected admission plugins config",
				func(configuredPlugins, expectedPlugins []gardencorev1beta1.AdmissionPlugin) {
					botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{
						Spec: gardencorev1beta1.ShootSpec{
							Kubernetes: gardencorev1beta1.Kubernetes{
								KubeAPIServer: &gardencorev1beta1.KubeAPIServerConfig{
									AdmissionPlugins: configuredPlugins,
								},
								Version: "1.22.1",
							},
						},
					})

					kubeAPIServer, err := botanist.DefaultKubeAPIServer(ctx)
					Expect(err).NotTo(HaveOccurred())
					Expect(kubeAPIServer.GetValues().AdmissionPlugins).To(Equal(expectedPlugins))
				},

				Entry("only default plugins",
					nil,
					[]gardencorev1beta1.AdmissionPlugin{
						{Name: "Priority"},
						{Name: "NamespaceLifecycle"},
						{Name: "LimitRanger"},
						{Name: "PodSecurityPolicy"},
						{Name: "ServiceAccount"},
						{Name: "NodeRestriction"},
						{Name: "DefaultStorageClass"},
						{Name: "DefaultTolerationSeconds"},
						{Name: "ResourceQuota"},
						{Name: "StorageObjectInUseProtection"},
						{Name: "MutatingAdmissionWebhook"},
						{Name: "ValidatingAdmissionWebhook"},
					},
				),
				Entry("default plugins with overrides",
					[]gardencorev1beta1.AdmissionPlugin{
						{Name: "NamespaceLifecycle", Config: &runtime.RawExtension{Raw: []byte("namespace-lifecycle-config")}},
					},
					[]gardencorev1beta1.AdmissionPlugin{
						{Name: "Priority"},
						{Name: "NamespaceLifecycle", Config: &runtime.RawExtension{Raw: []byte("namespace-lifecycle-config")}},
						{Name: "LimitRanger"},
						{Name: "PodSecurityPolicy"},
						{Name: "ServiceAccount"},
						{Name: "NodeRestriction"},
						{Name: "DefaultStorageClass"},
						{Name: "DefaultTolerationSeconds"},
						{Name: "ResourceQuota"},
						{Name: "StorageObjectInUseProtection"},
						{Name: "MutatingAdmissionWebhook"},
						{Name: "ValidatingAdmissionWebhook"},
					},
				),
				Entry("default plugins with overrides and other plugins",
					[]gardencorev1beta1.AdmissionPlugin{
						{Name: "NamespaceLifecycle", Config: &runtime.RawExtension{Raw: []byte("namespace-lifecycle-config")}},
						{Name: "Foo"},
						{Name: "Bar"},
						{Name: "Baz", Config: &runtime.RawExtension{Raw: []byte("baz-config")}},
					},
					[]gardencorev1beta1.AdmissionPlugin{
						{Name: "Priority"},
						{Name: "NamespaceLifecycle", Config: &runtime.RawExtension{Raw: []byte("namespace-lifecycle-config")}},
						{Name: "LimitRanger"},
						{Name: "PodSecurityPolicy"},
						{Name: "ServiceAccount"},
						{Name: "NodeRestriction"},
						{Name: "DefaultStorageClass"},
						{Name: "DefaultTolerationSeconds"},
						{Name: "ResourceQuota"},
						{Name: "StorageObjectInUseProtection"},
						{Name: "MutatingAdmissionWebhook"},
						{Name: "ValidatingAdmissionWebhook"},
						{Name: "Foo"},
						{Name: "Bar"},
						{Name: "Baz", Config: &runtime.RawExtension{Raw: []byte("baz-config")}},
					},
				),
			)
		})

		Describe("AuditConfig", func() {
			var (
				policy               = "some-policy"
				auditPolicyConfigMap *corev1.ConfigMap
			)

			BeforeEach(func() {
				auditPolicyConfigMap = &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-audit-policy",
						Namespace: projectNamespace,
					},
					Data: map[string]string{"policy": policy},
				}
			})

			DescribeTable("should have the expected audit config",
				func(prepTest func(), expectedConfig *kubeapiserver.AuditConfig, errMatcher gomegatypes.GomegaMatcher) {
					if prepTest != nil {
						prepTest()
					}

					kubeAPIServer, err := botanist.DefaultKubeAPIServer(ctx)
					Expect(err).To(errMatcher)
					if kubeAPIServer != nil {
						Expect(kubeAPIServer.GetValues().Audit).To(Equal(expectedConfig))
					}
				},

				Entry("KubeAPIServerConfig is nil",
					nil,
					nil,
					Not(HaveOccurred()),
				),
				Entry("AuditConfig is nil",
					func() {
						botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{
							Spec: gardencorev1beta1.ShootSpec{
								Kubernetes: gardencorev1beta1.Kubernetes{
									KubeAPIServer: &gardencorev1beta1.KubeAPIServerConfig{},
								},
							},
						})
					},
					nil,
					Not(HaveOccurred()),
				),
				Entry("AuditPolicy is nil",
					func() {
						botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{
							Spec: gardencorev1beta1.ShootSpec{
								Kubernetes: gardencorev1beta1.Kubernetes{
									KubeAPIServer: &gardencorev1beta1.KubeAPIServerConfig{
										AuditConfig: &gardencorev1beta1.AuditConfig{},
									},
								},
							},
						})
					},
					nil,
					Not(HaveOccurred()),
				),
				Entry("ConfigMapRef is nil",
					func() {
						botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{
							Spec: gardencorev1beta1.ShootSpec{
								Kubernetes: gardencorev1beta1.Kubernetes{
									KubeAPIServer: &gardencorev1beta1.KubeAPIServerConfig{
										AuditConfig: &gardencorev1beta1.AuditConfig{
											AuditPolicy: &gardencorev1beta1.AuditPolicy{},
										},
									},
								},
							},
						})
					},
					nil,
					Not(HaveOccurred()),
				),
				Entry("ConfigMapRef is provided but configmap is missing",
					func() {
						botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{
							ObjectMeta: metav1.ObjectMeta{
								Namespace: projectNamespace,
							},
							Spec: gardencorev1beta1.ShootSpec{
								Kubernetes: gardencorev1beta1.Kubernetes{
									KubeAPIServer: &gardencorev1beta1.KubeAPIServerConfig{
										AuditConfig: &gardencorev1beta1.AuditConfig{
											AuditPolicy: &gardencorev1beta1.AuditPolicy{
												ConfigMapRef: &corev1.ObjectReference{
													Name: auditPolicyConfigMap.Name,
												},
											},
										},
									},
								},
							},
						})
					},
					nil,
					MatchError(ContainSubstring("not found")),
				),
				Entry("ConfigMapRef is provided but configmap is missing while shoot has a deletion timestamp",
					func() {
						botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{
							ObjectMeta: metav1.ObjectMeta{
								Namespace:         projectNamespace,
								DeletionTimestamp: &metav1.Time{},
							},
							Spec: gardencorev1beta1.ShootSpec{
								Kubernetes: gardencorev1beta1.Kubernetes{
									KubeAPIServer: &gardencorev1beta1.KubeAPIServerConfig{
										AuditConfig: &gardencorev1beta1.AuditConfig{
											AuditPolicy: &gardencorev1beta1.AuditPolicy{
												ConfigMapRef: &corev1.ObjectReference{
													Name: auditPolicyConfigMap.Name,
												},
											},
										},
									},
								},
							},
						})
					},
					&kubeapiserver.AuditConfig{},
					Not(HaveOccurred()),
				),
				Entry("ConfigMapRef is provided but configmap does not have correct data field",
					func() {
						auditPolicyConfigMap.Data = nil
						Expect(gc.Create(ctx, auditPolicyConfigMap)).To(Succeed())

						botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{
							ObjectMeta: metav1.ObjectMeta{
								Namespace: projectNamespace,
							},
							Spec: gardencorev1beta1.ShootSpec{
								Kubernetes: gardencorev1beta1.Kubernetes{
									KubeAPIServer: &gardencorev1beta1.KubeAPIServerConfig{
										AuditConfig: &gardencorev1beta1.AuditConfig{
											AuditPolicy: &gardencorev1beta1.AuditPolicy{
												ConfigMapRef: &corev1.ObjectReference{
													Name: auditPolicyConfigMap.Name,
												},
											},
										},
									},
								},
							},
						})
					},
					nil,
					MatchError(ContainSubstring("missing '.data.policy' in audit policy configmap")),
				),
				Entry("ConfigMapRef is provided and configmap is compliant",
					func() {
						Expect(gc.Create(ctx, auditPolicyConfigMap)).To(Succeed())

						botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{
							ObjectMeta: metav1.ObjectMeta{
								Namespace: projectNamespace,
							},
							Spec: gardencorev1beta1.ShootSpec{
								Kubernetes: gardencorev1beta1.Kubernetes{
									KubeAPIServer: &gardencorev1beta1.KubeAPIServerConfig{
										AuditConfig: &gardencorev1beta1.AuditConfig{
											AuditPolicy: &gardencorev1beta1.AuditPolicy{
												ConfigMapRef: &corev1.ObjectReference{
													Name: auditPolicyConfigMap.Name,
												},
											},
										},
									},
								},
							},
						})
					},
					&kubeapiserver.AuditConfig{
						Policy: &policy,
					},
					Not(HaveOccurred()),
				),
			)
		})

		Describe("AutoscalingConfig", func() {
			DescribeTable("should have the expected autoscaling config",
				func(prepTest func(), featureGate *featuregate.Feature, value *bool, expectedConfig kubeapiserver.AutoscalingConfig) {
					if prepTest != nil {
						prepTest()
					}

					if featureGate != nil && value != nil {
						defer test.WithFeatureGate(gardenletfeatures.FeatureGate, *featureGate, *value)()
					}

					kubeAPIServer, err := botanist.DefaultKubeAPIServer(ctx)
					Expect(err).NotTo(HaveOccurred())
					Expect(kubeAPIServer.GetValues().Autoscaling).To(Equal(expectedConfig))
				},

				Entry("default behaviour, HVPA is disabled",
					nil,
					featureGatePtr(features.HVPA), pointer.Bool(false),
					kubeapiserver.AutoscalingConfig{
						APIServerResources:        resourcesRequirementsForKubeAPIServer(0, ""),
						HVPAEnabled:               false,
						MinReplicas:               1,
						MaxReplicas:               4,
						UseMemoryMetricForHvpaHPA: false,
						ScaleDownDisabledForHvpa:  false,
					},
				),
				Entry("default behaviour, HVPA is enabled",
					nil,
					featureGatePtr(features.HVPA), pointer.Bool(true),
					kubeapiserver.AutoscalingConfig{
						APIServerResources:        resourcesRequirementsForKubeAPIServer(0, ""),
						HVPAEnabled:               true,
						MinReplicas:               1,
						MaxReplicas:               4,
						UseMemoryMetricForHvpaHPA: false,
						ScaleDownDisabledForHvpa:  false,
					},
				),
				Entry("shoot purpose production",
					func() {
						botanist.Shoot.Purpose = gardencorev1beta1.ShootPurposeProduction
					},
					nil, nil,
					kubeapiserver.AutoscalingConfig{
						APIServerResources:        resourcesRequirementsForKubeAPIServer(0, ""),
						HVPAEnabled:               false,
						MinReplicas:               2,
						MaxReplicas:               4,
						UseMemoryMetricForHvpaHPA: false,
						ScaleDownDisabledForHvpa:  false,
					},
				),
				Entry("shoot disables scale down",
					func() {
						botanist.Shoot.GetInfo().Annotations = map[string]string{"alpha.control-plane.scaling.shoot.gardener.cloud/scale-down-disabled": "true"}
					},
					nil, nil,
					kubeapiserver.AutoscalingConfig{
						APIServerResources:        resourcesRequirementsForKubeAPIServer(0, ""),
						HVPAEnabled:               false,
						MinReplicas:               4,
						MaxReplicas:               4,
						UseMemoryMetricForHvpaHPA: false,
						ScaleDownDisabledForHvpa:  true,
					},
				),
				Entry("shoot is a managed seed and HVPAForShootedSeed is disabled",
					func() {
						botanist.ManagedSeed = &seedmanagementv1alpha1.ManagedSeed{}
					},
					featureGatePtr(features.HVPAForShootedSeed), pointer.Bool(false),
					kubeapiserver.AutoscalingConfig{
						APIServerResources:        resourcesRequirementsForKubeAPIServer(0, ""),
						HVPAEnabled:               false,
						MinReplicas:               1,
						MaxReplicas:               4,
						UseMemoryMetricForHvpaHPA: true,
						ScaleDownDisabledForHvpa:  false,
					},
				),
				Entry("shoot is a managed seed and HVPAForShootedSeed is enabled",
					func() {
						botanist.ManagedSeed = &seedmanagementv1alpha1.ManagedSeed{}
					},
					featureGatePtr(features.HVPAForShootedSeed), pointer.Bool(true),
					kubeapiserver.AutoscalingConfig{
						APIServerResources:        resourcesRequirementsForKubeAPIServer(0, ""),
						HVPAEnabled:               true,
						MinReplicas:               1,
						MaxReplicas:               4,
						UseMemoryMetricForHvpaHPA: true,
						ScaleDownDisabledForHvpa:  false,
					},
				),
				Entry("shoot is a managed seed w/ APIServer settings and HVPAForShootedSeed is enabled",
					func() {
						botanist.ManagedSeed = &seedmanagementv1alpha1.ManagedSeed{}
						botanist.ManagedSeedAPIServer = &helper.ShootedSeedAPIServer{
							Autoscaler: &helper.ShootedSeedAPIServerAutoscaler{
								MinReplicas: pointer.Int32(16),
								MaxReplicas: 32,
							},
							Replicas: pointer.Int32(24),
						}
					},
					featureGatePtr(features.HVPAForShootedSeed), pointer.Bool(true),
					kubeapiserver.AutoscalingConfig{
						APIServerResources:        resourcesRequirementsForKubeAPIServer(0, ""),
						HVPAEnabled:               true,
						MinReplicas:               16,
						MaxReplicas:               32,
						UseMemoryMetricForHvpaHPA: true,
						ScaleDownDisabledForHvpa:  false,
					},
				),
				Entry("shoot is a managed seed w/ APIServer settings and HVPAForShootedSeed is disabled",
					func() {
						botanist.ManagedSeed = &seedmanagementv1alpha1.ManagedSeed{}
						botanist.ManagedSeedAPIServer = &helper.ShootedSeedAPIServer{
							Autoscaler: &helper.ShootedSeedAPIServerAutoscaler{
								MinReplicas: pointer.Int32(16),
								MaxReplicas: 32,
							},
							Replicas: pointer.Int32(24),
						}
					},
					featureGatePtr(features.HVPAForShootedSeed), pointer.Bool(false),
					kubeapiserver.AutoscalingConfig{
						APIServerResources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("1750m"),
								corev1.ResourceMemory: resource.MustParse("2Gi"),
							},
							Limits: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("4000m"),
								corev1.ResourceMemory: resource.MustParse("8Gi"),
							},
						},
						HVPAEnabled:               false,
						MinReplicas:               16,
						MaxReplicas:               32,
						Replicas:                  pointer.Int32(24),
						UseMemoryMetricForHvpaHPA: true,
						ScaleDownDisabledForHvpa:  false,
					},
				),
			)
		})

		Describe("EventTTL", func() {
			It("should not set the event ttl field", func() {
				kubeAPIServer, err := botanist.DefaultKubeAPIServer(ctx)
				Expect(err).NotTo(HaveOccurred())
				Expect(kubeAPIServer.GetValues().EventTTL).To(BeNil())
			})

			It("should set the field to the configured values", func() {
				eventTTL := &metav1.Duration{Duration: 2 * time.Hour}

				botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{
					Spec: gardencorev1beta1.ShootSpec{
						Kubernetes: gardencorev1beta1.Kubernetes{
							KubeAPIServer: &gardencorev1beta1.KubeAPIServerConfig{
								EventTTL: eventTTL,
							},
						},
					},
				})

				kubeAPIServer, err := botanist.DefaultKubeAPIServer(ctx)
				Expect(err).NotTo(HaveOccurred())
				Expect(kubeAPIServer.GetValues().EventTTL).To(Equal(eventTTL))
			})
		})

		Describe("FeatureGates", func() {
			It("should set the field to nil by default", func() {
				kubeAPIServer, err := botanist.DefaultKubeAPIServer(ctx)
				Expect(err).NotTo(HaveOccurred())
				Expect(kubeAPIServer.GetValues().FeatureGates).To(BeNil())
			})

			It("should set the field to the configured values", func() {
				featureGates := map[string]bool{"foo": true, "bar": false}

				botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{
					Spec: gardencorev1beta1.ShootSpec{
						Kubernetes: gardencorev1beta1.Kubernetes{
							KubeAPIServer: &gardencorev1beta1.KubeAPIServerConfig{
								KubernetesConfig: gardencorev1beta1.KubernetesConfig{
									FeatureGates: featureGates,
								},
							},
						},
					},
				})

				kubeAPIServer, err := botanist.DefaultKubeAPIServer(ctx)
				Expect(err).NotTo(HaveOccurred())
				Expect(kubeAPIServer.GetValues().FeatureGates).To(Equal(featureGates))
			})
		})

		Describe("OIDCConfig", func() {
			DescribeTable("should have the expected OIDC config",
				func(prepTest func(), expectedConfig *gardencorev1beta1.OIDCConfig) {
					if prepTest != nil {
						prepTest()
					}

					kubeAPIServer, err := botanist.DefaultKubeAPIServer(ctx)
					Expect(err).NotTo(HaveOccurred())
					Expect(kubeAPIServer.GetValues().OIDC).To(Equal(expectedConfig))
				},

				Entry("KubeAPIServerConfig is nil",
					nil,
					nil,
				),
				Entry("OIDCConfig is nil",
					func() {
						botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{
							Spec: gardencorev1beta1.ShootSpec{
								Kubernetes: gardencorev1beta1.Kubernetes{
									KubeAPIServer: &gardencorev1beta1.KubeAPIServerConfig{},
								},
							},
						})
					},
					nil,
				),
				Entry("OIDCConfig is not nil",
					func() {
						botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{
							Spec: gardencorev1beta1.ShootSpec{
								Kubernetes: gardencorev1beta1.Kubernetes{
									KubeAPIServer: &gardencorev1beta1.KubeAPIServerConfig{
										OIDCConfig: &gardencorev1beta1.OIDCConfig{},
									},
								},
							},
						})
					},
					&gardencorev1beta1.OIDCConfig{},
				),
			)
		})

		Describe("Requests", func() {
			It("should set the field to nil by default", func() {
				kubeAPIServer, err := botanist.DefaultKubeAPIServer(ctx)
				Expect(err).NotTo(HaveOccurred())
				Expect(kubeAPIServer.GetValues().Requests).To(BeNil())
			})

			It("should set the field to the configured values", func() {
				requests := &gardencorev1beta1.KubeAPIServerRequests{
					MaxMutatingInflight:    pointer.Int32(1),
					MaxNonMutatingInflight: pointer.Int32(2),
				}

				botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{
					Spec: gardencorev1beta1.ShootSpec{
						Kubernetes: gardencorev1beta1.Kubernetes{
							KubeAPIServer: &gardencorev1beta1.KubeAPIServerConfig{
								Requests: requests,
							},
						},
					},
				})

				kubeAPIServer, err := botanist.DefaultKubeAPIServer(ctx)
				Expect(err).NotTo(HaveOccurred())
				Expect(kubeAPIServer.GetValues().Requests).To(Equal(requests))
			})
		})

		Describe("RuntimeConfig", func() {
			It("should set the field to nil by default", func() {
				kubeAPIServer, err := botanist.DefaultKubeAPIServer(ctx)
				Expect(err).NotTo(HaveOccurred())
				Expect(kubeAPIServer.GetValues().RuntimeConfig).To(BeNil())
			})

			It("should set the field to the configured values", func() {
				runtimeConfig := map[string]bool{"foo": true, "bar": false}

				botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{
					Spec: gardencorev1beta1.ShootSpec{
						Kubernetes: gardencorev1beta1.Kubernetes{
							KubeAPIServer: &gardencorev1beta1.KubeAPIServerConfig{
								RuntimeConfig: runtimeConfig,
							},
						},
					},
				})

				kubeAPIServer, err := botanist.DefaultKubeAPIServer(ctx)
				Expect(err).NotTo(HaveOccurred())
				Expect(kubeAPIServer.GetValues().RuntimeConfig).To(Equal(runtimeConfig))
			})
		})

		Describe("VPNConfig", func() {
			DescribeTable("should have the expected VPN config",
				func(prepTest func(), expectedConfig kubeapiserver.VPNConfig) {
					if prepTest != nil {
						prepTest()
					}

					kubeAPIServer, err := botanist.DefaultKubeAPIServer(ctx)
					Expect(err).NotTo(HaveOccurred())
					Expect(kubeAPIServer.GetValues().VPN).To(Equal(expectedConfig))
				},

				Entry("ReversedVPN disabled",
					nil,
					kubeapiserver.VPNConfig{
						ReversedVPNEnabled: false,
						PodNetworkCIDR:     podNetworkCIDR,
						ServiceNetworkCIDR: serviceNetworkCIDR,
						NodeNetworkCIDR:    &nodeNetworkCIDR,
					},
				),
				Entry("ReversedVPN enabled",
					func() {
						botanist.Shoot.ReversedVPNEnabled = true
					},
					kubeapiserver.VPNConfig{
						ReversedVPNEnabled: true,
						PodNetworkCIDR:     podNetworkCIDR,
						ServiceNetworkCIDR: serviceNetworkCIDR,
						NodeNetworkCIDR:    &nodeNetworkCIDR,
					},
				),
				Entry("no node network",
					func() {
						botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{})
					},
					kubeapiserver.VPNConfig{
						ReversedVPNEnabled: false,
						PodNetworkCIDR:     podNetworkCIDR,
						ServiceNetworkCIDR: serviceNetworkCIDR,
					},
				),
			)
		})

		Describe("WatchCacheSizes", func() {
			It("should set the field to nil by default", func() {
				kubeAPIServer, err := botanist.DefaultKubeAPIServer(ctx)
				Expect(err).NotTo(HaveOccurred())
				Expect(kubeAPIServer.GetValues().WatchCacheSizes).To(BeNil())
			})

			It("should set the field to the configured values", func() {
				watchCacheSizes := &gardencorev1beta1.WatchCacheSizes{
					Default:   pointer.Int32(1),
					Resources: []gardencorev1beta1.ResourceWatchCacheSize{{Resource: "foo"}},
				}

				botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{
					Spec: gardencorev1beta1.ShootSpec{
						Kubernetes: gardencorev1beta1.Kubernetes{
							KubeAPIServer: &gardencorev1beta1.KubeAPIServerConfig{
								WatchCacheSizes: watchCacheSizes,
							},
						},
					},
				})

				kubeAPIServer, err := botanist.DefaultKubeAPIServer(ctx)
				Expect(err).NotTo(HaveOccurred())
				Expect(kubeAPIServer.GetValues().WatchCacheSizes).To(Equal(watchCacheSizes))
			})
		})

	})

	DescribeTable("#resourcesRequirementsForKubeAPIServer",
		func(nodes int, storageClass, expectedCPURequest, expectedMemoryRequest, expectedCPULimit, expectedMemoryLimit string) {
			Expect(resourcesRequirementsForKubeAPIServer(int32(nodes), storageClass)).To(Equal(
				corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse(expectedCPURequest),
						corev1.ResourceMemory: resource.MustParse(expectedMemoryRequest),
					},
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse(expectedCPULimit),
						corev1.ResourceMemory: resource.MustParse(expectedMemoryLimit),
					},
				}))
		},

		// nodes tests
		Entry("nodes <= 2", 2, "", "800m", "800Mi", "1000m", "1200Mi"),
		Entry("nodes <= 10", 10, "", "1000m", "1100Mi", "1200m", "1900Mi"),
		Entry("nodes <= 50", 50, "", "1200m", "1600Mi", "1500m", "3900Mi"),
		Entry("nodes <= 100", 100, "", "2500m", "5200Mi", "3000m", "5900Mi"),
		Entry("nodes > 100", 1000, "", "3000m", "5200Mi", "4000m", "7800Mi"),

		// scaling class tests
		Entry("scaling class small", -1, "small", "800m", "800Mi", "1000m", "1200Mi"),
		Entry("scaling class medium", -1, "medium", "1000m", "1100Mi", "1200m", "1900Mi"),
		Entry("scaling class large", -1, "large", "1200m", "1600Mi", "1500m", "3900Mi"),
		Entry("scaling class xlarge", -1, "xlarge", "2500m", "5200Mi", "3000m", "5900Mi"),
		Entry("scaling class 2xlarge", -1, "2xlarge", "3000m", "5200Mi", "4000m", "7800Mi"),

		// scaling class always decides if provided
		Entry("nodes > 100, scaling class small", 100, "small", "800m", "800Mi", "1000m", "1200Mi"),
		Entry("nodes <= 100, scaling class medium", 100, "medium", "1000m", "1100Mi", "1200m", "1900Mi"),
		Entry("nodes <= 50, scaling class large", 50, "large", "1200m", "1600Mi", "1500m", "3900Mi"),
		Entry("nodes <= 10, scaling class xlarge", 10, "xlarge", "2500m", "5200Mi", "3000m", "5900Mi"),
		Entry("nodes <= 2, scaling class 2xlarge", 2, "2xlarge", "3000m", "5200Mi", "4000m", "7800Mi"),
	)

	Describe("#DeployKubeAPIServer", func() {
		DescribeTable("should correctly set the autoscaling replicas",
			func(prepTest func(), autoscalingConfig kubeapiserver.AutoscalingConfig, expectedReplicas int32) {
				if prepTest != nil {
					prepTest()
				}

				kubeAPIServer.EXPECT().GetValues().Return(kubeapiserver.Values{Autoscaling: autoscalingConfig})
				kubeAPIServer.EXPECT().SetAutoscalingReplicas(&expectedReplicas)
				kubeAPIServer.EXPECT().SetSecrets(gomock.Any())
				kubeAPIServer.EXPECT().SetSNIConfig(gomock.Any())
				kubeAPIServer.EXPECT().SetProbeToken(gomock.Any())
				kubeAPIServer.EXPECT().SetExternalHostname(gomock.Any())
				kubeAPIServer.EXPECT().SetServiceAccountConfig(gomock.Any())
				kubeAPIServer.EXPECT().Deploy(ctx)

				Expect(botanist.DeployKubeAPIServer(ctx)).To(Succeed())
			},

			Entry("no change due to already set",
				nil,
				kubeapiserver.AutoscalingConfig{Replicas: pointer.Int32(1)},
				int32(1),
			),
			Entry("use minReplicas because deployment does not exist",
				nil,
				kubeapiserver.AutoscalingConfig{MinReplicas: 2},
				int32(2),
			),
			Entry("use 0 because shoot is hibernated, even  if deployment does not exist",
				func() {
					botanist.Shoot.HibernationEnabled = true
				},
				kubeapiserver.AutoscalingConfig{MinReplicas: 2},
				int32(0),
			),
			Entry("use deployment replicas because they are greater than 0",
				func() {
					Expect(c.Create(ctx, &appsv1.Deployment{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "kube-apiserver",
							Namespace: shootNamespace,
						},
						Spec: appsv1.DeploymentSpec{
							Replicas: pointer.Int32(3),
						},
					})).To(Succeed())
				},
				kubeapiserver.AutoscalingConfig{},
				int32(3),
			),
			Entry("use 0 because shoot is hibernated and deployment is already scaled down",
				func() {
					botanist.Shoot.HibernationEnabled = true
					Expect(c.Create(ctx, &appsv1.Deployment{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "kube-apiserver",
							Namespace: shootNamespace,
						},
						Spec: appsv1.DeploymentSpec{
							Replicas: pointer.Int32(0),
						},
					})).To(Succeed())
				},
				kubeapiserver.AutoscalingConfig{},
				int32(0),
			),
		)

		var apiServerResources = corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("1"),
				corev1.ResourceMemory: resource.MustParse("2"),
			},
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("3"),
				corev1.ResourceMemory: resource.MustParse("4"),
			},
		}

		DescribeTable("should correctly set the autoscaling apiserver resources",
			func(prepTest func(), autoscalingConfig kubeapiserver.AutoscalingConfig, expectedResources *corev1.ResourceRequirements) {
				if prepTest != nil {
					prepTest()
				}

				kubeAPIServer.EXPECT().GetValues().Return(kubeapiserver.Values{Autoscaling: autoscalingConfig})
				kubeAPIServer.EXPECT().SetAutoscalingReplicas(gomock.Any())
				if expectedResources != nil {
					kubeAPIServer.EXPECT().SetAutoscalingAPIServerResources(*expectedResources)
				}
				kubeAPIServer.EXPECT().SetSecrets(gomock.Any())
				kubeAPIServer.EXPECT().SetSNIConfig(gomock.Any())
				kubeAPIServer.EXPECT().SetProbeToken(gomock.Any())
				kubeAPIServer.EXPECT().SetExternalHostname(gomock.Any())
				kubeAPIServer.EXPECT().SetServiceAccountConfig(gomock.Any())
				kubeAPIServer.EXPECT().Deploy(ctx)

				Expect(botanist.DeployKubeAPIServer(ctx)).To(Succeed())
			},

			Entry("nothing is set because deployment is not found",
				nil,
				kubeapiserver.AutoscalingConfig{},
				nil,
			),
			Entry("nothing is set because HVPA is disabled",
				func() {
					Expect(c.Create(ctx, &appsv1.Deployment{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "kube-apiserver",
							Namespace: shootNamespace,
						},
						Spec: appsv1.DeploymentSpec{
							Template: corev1.PodTemplateSpec{
								Spec: corev1.PodSpec{
									Containers: []corev1.Container{{
										Name:      "kube-apiserver",
										Resources: apiServerResources,
									}},
								},
							},
						},
					})).To(Succeed())
				},
				kubeapiserver.AutoscalingConfig{HVPAEnabled: false},
				nil,
			),
			Entry("set the existing requirements because deployment found and HVPA enabled",
				func() {
					Expect(c.Create(ctx, &appsv1.Deployment{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "kube-apiserver",
							Namespace: shootNamespace,
						},
						Spec: appsv1.DeploymentSpec{
							Template: corev1.PodTemplateSpec{
								Spec: corev1.PodSpec{
									Containers: []corev1.Container{{
										Name:      "kube-apiserver",
										Resources: apiServerResources,
									}},
								},
							},
						},
					})).To(Succeed())
				},
				kubeapiserver.AutoscalingConfig{HVPAEnabled: true},
				&apiServerResources,
			),
		)

		DescribeTable("should correctly set the secrets",
			func(values kubeapiserver.Values, mutateSecrets func(*kubeapiserver.Secrets)) {
				secrets := kubeapiserver.Secrets{
					CA:                     component.Secret{Name: "ca", Checksum: botanist.LoadCheckSum("ca")},
					CAEtcd:                 component.Secret{Name: "ca-etcd", Checksum: botanist.LoadCheckSum("ca-etcd")},
					CAFrontProxy:           component.Secret{Name: "ca-front-proxy", Checksum: botanist.LoadCheckSum("ca-front-proxy")},
					Etcd:                   component.Secret{Name: "etcd-client-tls", Checksum: botanist.LoadCheckSum("etcd-client-tls")},
					EtcdEncryptionConfig:   component.Secret{Name: "etcd-encryption-secret", Checksum: botanist.LoadCheckSum("etcd-encryption-secret")},
					KubeAggregator:         component.Secret{Name: "kube-aggregator", Checksum: botanist.LoadCheckSum("kube-aggregator")},
					KubeAPIServerToKubelet: component.Secret{Name: "kube-apiserver-kubelet", Checksum: botanist.LoadCheckSum("kube-apiserver-kubelet")},
					Server:                 component.Secret{Name: "kube-apiserver", Checksum: botanist.LoadCheckSum("kube-apiserver")},
					ServiceAccountKey:      component.Secret{Name: "service-account-key", Checksum: botanist.LoadCheckSum("service-account-key")},
					StaticToken:            component.Secret{Name: "static-token", Checksum: botanist.LoadCheckSum("static-token")},
				}
				mutateSecrets(&secrets)

				kubeAPIServer.EXPECT().GetValues().Return(values)
				kubeAPIServer.EXPECT().SetAutoscalingReplicas(gomock.Any())
				kubeAPIServer.EXPECT().SetSecrets(secrets)
				kubeAPIServer.EXPECT().SetSNIConfig(gomock.Any())
				kubeAPIServer.EXPECT().SetProbeToken(gomock.Any())
				kubeAPIServer.EXPECT().SetExternalHostname(gomock.Any())
				kubeAPIServer.EXPECT().SetServiceAccountConfig(gomock.Any())
				kubeAPIServer.EXPECT().Deploy(ctx)

				Expect(botanist.DeployKubeAPIServer(ctx)).To(Succeed())
			},

			Entry("reversed vpn disabled",
				kubeapiserver.Values{VPN: kubeapiserver.VPNConfig{ReversedVPNEnabled: false}},
				func(s *kubeapiserver.Secrets) {
					s.VPNSeed = &component.Secret{Name: "vpn-seed", Checksum: botanist.LoadCheckSum("vpn-seed")}
					s.VPNSeedTLSAuth = &component.Secret{Name: "vpn-seed-tlsauth", Checksum: botanist.LoadCheckSum("vpn-seed-tlsauth")}
				},
			),
			Entry("reversed vpn enabled",
				kubeapiserver.Values{VPN: kubeapiserver.VPNConfig{ReversedVPNEnabled: true}},
				func(s *kubeapiserver.Secrets) {
					s.HTTPProxy = &component.Secret{Name: "kube-apiserver-http-proxy", Checksum: botanist.LoadCheckSum("kube-apiserver-http-proxy")}
					s.VPNSeedServerTLSAuth = &component.Secret{Name: "vpn-seed-server-tlsauth", Checksum: botanist.LoadCheckSum("vpn-seed-server-tlsauth")}
				},
			),
			Entry("basic auth enabled",
				kubeapiserver.Values{BasicAuthenticationEnabled: true, VPN: kubeapiserver.VPNConfig{ReversedVPNEnabled: true}},
				func(s *kubeapiserver.Secrets) {
					s.BasicAuthentication = &component.Secret{Name: "kube-apiserver-basic-auth", Checksum: botanist.LoadCheckSum("kube-apiserver-basic-auth")}
					s.HTTPProxy = &component.Secret{Name: "kube-apiserver-http-proxy", Checksum: botanist.LoadCheckSum("kube-apiserver-http-proxy")}
					s.VPNSeedServerTLSAuth = &component.Secret{Name: "vpn-seed-server-tlsauth", Checksum: botanist.LoadCheckSum("vpn-seed-server-tlsauth")}
				},
			),
		)

		Describe("ExternalHostname", func() {
			It("should set the external hostname to the out-of-cluster address", func() {
				kubeAPIServer.EXPECT().GetValues()
				kubeAPIServer.EXPECT().SetAutoscalingReplicas(gomock.Any())
				kubeAPIServer.EXPECT().SetSecrets(gomock.Any())
				kubeAPIServer.EXPECT().SetSNIConfig(gomock.Any())
				kubeAPIServer.EXPECT().SetProbeToken(gomock.Any())
				kubeAPIServer.EXPECT().SetExternalHostname("api.foo.bar.com")
				kubeAPIServer.EXPECT().SetServiceAccountConfig(gomock.Any())
				kubeAPIServer.EXPECT().Deploy(ctx)

				Expect(botanist.DeployKubeAPIServer(ctx)).To(Succeed())
			})
		})

		Describe("ProbeToken", func() {
			It("should have the correct probe token", func() {
				kubeAPIServer.EXPECT().GetValues()
				kubeAPIServer.EXPECT().SetAutoscalingReplicas(gomock.Any())
				kubeAPIServer.EXPECT().SetSecrets(gomock.Any())
				kubeAPIServer.EXPECT().SetSNIConfig(gomock.Any())
				kubeAPIServer.EXPECT().SetProbeToken(healthCheckToken)
				kubeAPIServer.EXPECT().SetExternalHostname(gomock.Any())
				kubeAPIServer.EXPECT().SetServiceAccountConfig(gomock.Any())
				kubeAPIServer.EXPECT().Deploy(ctx)

				Expect(botanist.DeployKubeAPIServer(ctx)).To(Succeed())
			})
		})

		Describe("ServiceAccountConfig", func() {
			var (
				signingKey            = []byte("some-key")
				signingKeySecret      *corev1.Secret
				maxTokenExpiration    = metav1.Duration{Duration: time.Hour}
				extendTokenExpiration = false
			)

			BeforeEach(func() {
				signingKeySecret = &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-secret",
						Namespace: projectNamespace,
					},
					Data: map[string][]byte{"signing-key": signingKey},
				}
			})

			DescribeTable("should have the expected ServiceAccountConfig config",
				func(prepTest func(), expectedConfig kubeapiserver.ServiceAccountConfig, expectError bool, errMatcher gomegatypes.GomegaMatcher) {
					if prepTest != nil {
						prepTest()
					}

					kubeAPIServer.EXPECT().GetValues()
					kubeAPIServer.EXPECT().SetAutoscalingReplicas(gomock.Any())
					kubeAPIServer.EXPECT().SetSecrets(gomock.Any())
					kubeAPIServer.EXPECT().SetSNIConfig(gomock.Any())
					kubeAPIServer.EXPECT().SetProbeToken(gomock.Any())
					kubeAPIServer.EXPECT().SetExternalHostname(gomock.Any())
					if !expectError {
						kubeAPIServer.EXPECT().SetServiceAccountConfig(expectedConfig)
						kubeAPIServer.EXPECT().Deploy(ctx)
					}

					Expect(botanist.DeployKubeAPIServer(ctx)).To(errMatcher)
				},

				Entry("KubeAPIServerConfig is nil",
					nil,
					kubeapiserver.ServiceAccountConfig{Issuer: "https://api." + internalClusterDomain},
					false,
					Not(HaveOccurred()),
				),
				Entry("ServiceAccountConfig is nil",
					func() {
						botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{
							Spec: gardencorev1beta1.ShootSpec{
								Kubernetes: gardencorev1beta1.Kubernetes{
									KubeAPIServer: &gardencorev1beta1.KubeAPIServerConfig{},
								},
							},
						})
					},
					kubeapiserver.ServiceAccountConfig{Issuer: "https://api." + internalClusterDomain},
					false,
					Not(HaveOccurred()),
				),
				Entry("Issuer is not provided",
					func() {
						botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{
							Spec: gardencorev1beta1.ShootSpec{
								Kubernetes: gardencorev1beta1.Kubernetes{
									KubeAPIServer: &gardencorev1beta1.KubeAPIServerConfig{
										ServiceAccountConfig: &gardencorev1beta1.ServiceAccountConfig{
											ExtendTokenExpiration: &extendTokenExpiration,
											MaxTokenExpiration:    &maxTokenExpiration,
										},
									},
								},
							},
						})
					},
					kubeapiserver.ServiceAccountConfig{
						Issuer:                "https://api." + internalClusterDomain,
						ExtendTokenExpiration: &extendTokenExpiration,
						MaxTokenExpiration:    &maxTokenExpiration,
					},
					false,
					Not(HaveOccurred()),
				),
				Entry("Issuer is provided",
					func() {
						botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{
							Spec: gardencorev1beta1.ShootSpec{
								Kubernetes: gardencorev1beta1.Kubernetes{
									KubeAPIServer: &gardencorev1beta1.KubeAPIServerConfig{
										ServiceAccountConfig: &gardencorev1beta1.ServiceAccountConfig{
											Issuer: pointer.String("issuer"),
										},
									},
								},
							},
						})
					},
					kubeapiserver.ServiceAccountConfig{Issuer: "issuer"},
					false,
					Not(HaveOccurred()),
				),
				Entry("SigningKeySecret is nil",
					func() {
						botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{
							Spec: gardencorev1beta1.ShootSpec{
								Kubernetes: gardencorev1beta1.Kubernetes{
									KubeAPIServer: &gardencorev1beta1.KubeAPIServerConfig{
										ServiceAccountConfig: &gardencorev1beta1.ServiceAccountConfig{},
									},
								},
							},
						})
					},
					kubeapiserver.ServiceAccountConfig{Issuer: "https://api." + internalClusterDomain},
					false,
					Not(HaveOccurred()),
				),
				Entry("SigningKeySecret is provided but secret is missing",
					func() {
						botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{
							ObjectMeta: metav1.ObjectMeta{
								Namespace: projectNamespace,
							},
							Spec: gardencorev1beta1.ShootSpec{
								Kubernetes: gardencorev1beta1.Kubernetes{
									KubeAPIServer: &gardencorev1beta1.KubeAPIServerConfig{
										ServiceAccountConfig: &gardencorev1beta1.ServiceAccountConfig{
											SigningKeySecret: &corev1.LocalObjectReference{
												Name: signingKeySecret.Name,
											},
										},
									},
								},
							},
						})
					},
					nil,
					true,
					MatchError(ContainSubstring("not found")),
				),
				Entry("SigningKeySecret is provided but secret does not have correct data field",
					func() {
						signingKeySecret.Data = nil
						Expect(gc.Create(ctx, signingKeySecret)).To(Succeed())

						botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{
							ObjectMeta: metav1.ObjectMeta{
								Namespace: projectNamespace,
							},
							Spec: gardencorev1beta1.ShootSpec{
								Kubernetes: gardencorev1beta1.Kubernetes{
									KubeAPIServer: &gardencorev1beta1.KubeAPIServerConfig{
										ServiceAccountConfig: &gardencorev1beta1.ServiceAccountConfig{
											SigningKeySecret: &corev1.LocalObjectReference{
												Name: signingKeySecret.Name,
											},
										},
									},
								},
							},
						})
					},
					kubeapiserver.ServiceAccountConfig{Issuer: "https://api." + internalClusterDomain},
					true,
					MatchError(ContainSubstring("no signing key in secret")),
				),
				Entry("SigningKeySecret is provided and secret is compliant",
					func() {
						Expect(gc.Create(ctx, signingKeySecret)).To(Succeed())

						botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{
							ObjectMeta: metav1.ObjectMeta{
								Namespace: projectNamespace,
							},
							Spec: gardencorev1beta1.ShootSpec{
								Kubernetes: gardencorev1beta1.Kubernetes{
									KubeAPIServer: &gardencorev1beta1.KubeAPIServerConfig{
										ServiceAccountConfig: &gardencorev1beta1.ServiceAccountConfig{
											SigningKeySecret: &corev1.LocalObjectReference{
												Name: signingKeySecret.Name,
											},
										},
									},
								},
							},
						})
					},
					kubeapiserver.ServiceAccountConfig{
						Issuer:     "https://api." + internalClusterDomain,
						SigningKey: signingKey,
					},
					false,
					Not(HaveOccurred()),
				),
			)
		})

		Describe("SNIConfig", func() {
			DescribeTable("should have the expected SNI config",
				func(prepTest func(), featureGate *featuregate.Feature, value *bool, expectedConfig kubeapiserver.SNIConfig) {
					if prepTest != nil {
						prepTest()
					}

					if featureGate != nil && value != nil {
						defer test.WithFeatureGate(gardenletfeatures.FeatureGate, *featureGate, *value)()
					}

					kubeAPIServer.EXPECT().GetValues()
					kubeAPIServer.EXPECT().SetAutoscalingReplicas(gomock.Any())
					kubeAPIServer.EXPECT().SetSecrets(gomock.Any())
					kubeAPIServer.EXPECT().SetSNIConfig(expectedConfig)
					kubeAPIServer.EXPECT().SetProbeToken(gomock.Any())
					kubeAPIServer.EXPECT().SetExternalHostname(gomock.Any())
					kubeAPIServer.EXPECT().SetServiceAccountConfig(gomock.Any())
					kubeAPIServer.EXPECT().Deploy(ctx)

					Expect(botanist.DeployKubeAPIServer(ctx)).To(Succeed())
				},

				Entry("SNI disabled",
					nil,
					featureGatePtr(features.APIServerSNI), pointer.Bool(false),
					kubeapiserver.SNIConfig{
						PodMutatorEnabled: false,
					},
				),
				Entry("SNI enabled but no need for internal DNS",
					func() {
						botanist.Shoot.DisableDNS = true
					},
					featureGatePtr(features.APIServerSNI), pointer.Bool(true),
					kubeapiserver.SNIConfig{
						PodMutatorEnabled: false,
					},
				),
				Entry("SNI enabled but no need for external DNS",
					func() {
						botanist.Shoot.DisableDNS = false
						botanist.Garden.InternalDomain = &gardenpkg.Domain{}
						botanist.Shoot.GetInfo().Spec.DNS = nil
					},
					featureGatePtr(features.APIServerSNI), pointer.Bool(true),
					kubeapiserver.SNIConfig{
						PodMutatorEnabled: false,
					},
				),
				Entry("SNI and both DNS enabled",
					func() {
						botanist.Shoot.DisableDNS = false
						botanist.Garden.InternalDomain = &gardenpkg.Domain{}
						botanist.Shoot.ExternalDomain = &gardenpkg.Domain{}
						botanist.Shoot.ExternalClusterDomain = pointer.StringPtr("some-domain")
						botanist.Shoot.GetInfo().Spec.DNS = &gardencorev1beta1.DNS{
							Domain:    pointer.StringPtr("some-domain"),
							Providers: []gardencorev1beta1.DNSProvider{{}},
						}
					},
					featureGatePtr(features.APIServerSNI), pointer.Bool(true),
					kubeapiserver.SNIConfig{
						Enabled:           true,
						AdvertiseAddress:  apiServerClusterIP,
						PodMutatorEnabled: true,
						APIServerFQDN:     "api." + internalClusterDomain,
					},
				),
				Entry("SNI and both DNS enabled but pod injector disabled via annotation",
					func() {
						botanist.Shoot.DisableDNS = false
						botanist.Garden.InternalDomain = &gardenpkg.Domain{}
						botanist.Shoot.ExternalDomain = &gardenpkg.Domain{}
						botanist.Shoot.ExternalClusterDomain = pointer.StringPtr("some-domain")
						botanist.Shoot.GetInfo().Spec.DNS = &gardencorev1beta1.DNS{
							Domain:    pointer.StringPtr("some-domain"),
							Providers: []gardencorev1beta1.DNSProvider{{}},
						}
						botanist.Shoot.GetInfo().Annotations = map[string]string{"alpha.featuregates.shoot.gardener.cloud/apiserver-sni-pod-injector": "disable"}
					},
					featureGatePtr(features.APIServerSNI), pointer.Bool(true),
					kubeapiserver.SNIConfig{
						Enabled:           true,
						AdvertiseAddress:  apiServerClusterIP,
						PodMutatorEnabled: false,
					},
				),
			)
		})
	})

	Describe("#DeleteKubeAPIServer", func() {
		It("should properly invalidate the client and destroy the component", func() {
			clientMap := fakeclientmap.NewClientMap().AddClient(keys.ForShoot(botanist.Shoot.GetInfo()), k8sSeedClient)
			botanist.ClientMap = clientMap

			shootClient, err := botanist.ClientMap.GetClient(ctx, keys.ForShoot(botanist.Shoot.GetInfo()))
			Expect(err).NotTo(HaveOccurred())
			Expect(shootClient).To(Equal(k8sSeedClient))

			k8sShootClient := fake.NewClientSetBuilder().WithClient(c).Build()
			botanist.K8sShootClient = k8sShootClient

			kubeAPIServer.EXPECT().Destroy(ctx)

			Expect(botanist.DeleteKubeAPIServer(ctx)).To(Succeed())

			shootClient, err = clientMap.GetClient(ctx, keys.ForShoot(botanist.Shoot.GetInfo()))
			Expect(err).To(MatchError("clientSet for key \"/\" not found"))
			Expect(shootClient).To(BeNil())

			Expect(botanist.K8sShootClient).To(BeNil())
		})
	})

	Describe("#ScaleKubeAPIServerToOne", func() {
		deployment := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "kube-apiserver", Namespace: shootNamespace}}

		It("should scale the KAPI deployment", func() {
			Expect(c.Create(ctx, deployment)).To(Succeed())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(deployment), deployment)).To(Succeed())

			Expect(botanist.ScaleKubeAPIServerToOne(ctx)).To(Succeed())

			Expect(c.Get(ctx, client.ObjectKeyFromObject(deployment), deployment)).To(Succeed())
			Expect(deployment.Spec.Replicas).To(Equal(pointer.Int32(1)))
		})
	})
})

func featureGatePtr(f featuregate.Feature) *featuregate.Feature {
	return &f
}
