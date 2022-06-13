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

	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	fakeclientmap "github.com/gardener/gardener/pkg/client/kubernetes/clientmap/fake"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	"github.com/gardener/gardener/pkg/client/kubernetes/fake"
	"github.com/gardener/gardener/pkg/features"
	gardenletfeatures "github.com/gardener/gardener/pkg/gardenlet/features"
	"github.com/gardener/gardener/pkg/operation"
	"github.com/gardener/gardener/pkg/operation/botanist/component/kubeapiserver"
	mockkubeapiserver "github.com/gardener/gardener/pkg/operation/botanist/component/kubeapiserver/mock"
	gardenpkg "github.com/gardener/gardener/pkg/operation/garden"
	seedpkg "github.com/gardener/gardener/pkg/operation/seed"
	shootpkg "github.com/gardener/gardener/pkg/operation/shoot"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	fakesecretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager/fake"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
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
		sm              secretsmanager.Interface
		botanist        *Botanist
		kubeAPIServer   *mockkubeapiserver.MockInterface

		ctx                   = context.TODO()
		projectNamespace      = "garden-foo"
		seedNamespace         = "shoot--foo--bar"
		shootName             = "bar"
		internalClusterDomain = "internal.foo.bar.com"
		externalClusterDomain = "external.foo.bar.com"
		podNetwork            *net.IPNet
		serviceNetwork        *net.IPNet
		apiServerNetwork      = net.ParseIP("10.0.4.1")
		podNetworkCIDR        = "10.0.1.0/24"
		serviceNetworkCIDR    = "10.0.2.0/24"
		nodeNetworkCIDR       = "10.0.3.0/24"
		apiServerClusterIP    = "1.2.3.4"
		apiServerAddress      = "5.6.7.8"
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

		sm = fakesecretsmanager.New(c, seedNamespace)

		By("creating secrets managed outside of this function for whose secretsmanager.Get() will be called")
		Expect(c.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "ca", Namespace: seedNamespace}})).To(Succeed())
		Expect(c.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "user-kubeconfig", Namespace: seedNamespace}})).To(Succeed())

		kubeAPIServer = mockkubeapiserver.NewMockInterface(ctrl)
		botanist = &Botanist{
			Operation: &operation.Operation{
				K8sGardenClient: k8sGardenClient,
				K8sSeedClient:   k8sSeedClient,
				SecretsManager:  sm,
				Garden:          &gardenpkg.Garden{},
				Seed:            &seedpkg.Seed{},
				Shoot: &shootpkg.Shoot{
					SeedNamespace: seedNamespace,
					Components: &shootpkg.Components{
						ControlPlane: &shootpkg.ControlPlane{
							KubeAPIServer: kubeAPIServer,
						},
					},
					InternalClusterDomain: internalClusterDomain,
					ExternalClusterDomain: &externalClusterDomain,
					Networks: &shootpkg.Networks{
						APIServer: apiServerNetwork,
						Pods:      podNetwork,
						Services:  serviceNetwork,
					},
				},
				ImageVector: imagevector.ImageVector{
					{Name: "alpine-iptables"},
					{Name: "apiserver-proxy-pod-webhook"},
					{Name: "kube-apiserver"},
					{Name: "vpn-seed"},
				},
				APIServerAddress:   apiServerAddress,
				APIServerClusterIP: apiServerClusterIP,
			},
		}

		botanist.Seed.SetInfo(&gardencorev1beta1.Seed{
			Spec: gardencorev1beta1.SeedSpec{
				Settings: &gardencorev1beta1.SeedSettings{
					ShootDNS: &gardencorev1beta1.SeedSettingShootDNS{
						Enabled: true,
					},
				},
			},
		})
		botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{
			ObjectMeta: metav1.ObjectMeta{
				Name:      shootName,
				Namespace: projectNamespace,
			},
			Spec: gardencorev1beta1.ShootSpec{
				DNS: &gardencorev1beta1.DNS{
					Domain: &externalClusterDomain,
				},
				Networking: gardencorev1beta1.Networking{
					Nodes: &nodeNetworkCIDR,
				},
			},
			Status: gardencorev1beta1.ShootStatus{
				TechnicalID: seedNamespace,
			},
		})
		botanist.SetShootState(&gardencorev1alpha1.ShootState{})
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
				shootCopy := botanist.Shoot.GetInfo().DeepCopy()
				shootCopy.Spec.Kubernetes = gardencorev1beta1.Kubernetes{
					KubeAPIServer: &gardencorev1beta1.KubeAPIServerConfig{
						EnableAnonymousAuthentication: pointer.Bool(true),
					},
				}
				botanist.Shoot.SetInfo(shootCopy)

				kubeAPIServer, err := botanist.DefaultKubeAPIServer(ctx)
				Expect(err).NotTo(HaveOccurred())
				Expect(kubeAPIServer.GetValues().AnonymousAuthenticationEnabled).To(BeTrue())
			})
		})

		Describe("APIAudiences", func() {
			It("should set the field to 'kubernetes' and 'gardener' by default", func() {
				kubeAPIServer, err := botanist.DefaultKubeAPIServer(ctx)
				Expect(err).NotTo(HaveOccurred())
				Expect(kubeAPIServer.GetValues().APIAudiences).To(ConsistOf("kubernetes", "gardener"))
			})

			It("should set the field to the configured values", func() {
				apiAudiences := []string{"foo", "bar"}

				shootCopy := botanist.Shoot.GetInfo().DeepCopy()
				shootCopy.Spec.Kubernetes = gardencorev1beta1.Kubernetes{
					KubeAPIServer: &gardencorev1beta1.KubeAPIServerConfig{
						APIAudiences: apiAudiences,
					},
				}
				botanist.Shoot.SetInfo(shootCopy)

				kubeAPIServer, err := botanist.DefaultKubeAPIServer(ctx)
				Expect(err).NotTo(HaveOccurred())
				Expect(kubeAPIServer.GetValues().APIAudiences).To(Equal(append(apiAudiences, "gardener")))
			})

			It("should not add gardener audience if already present", func() {
				apiAudiences := []string{"foo", "bar", "gardener"}

				shootCopy := botanist.Shoot.GetInfo().DeepCopy()
				shootCopy.Spec.Kubernetes = gardencorev1beta1.Kubernetes{
					KubeAPIServer: &gardencorev1beta1.KubeAPIServerConfig{
						APIAudiences: apiAudiences,
					},
				}
				botanist.Shoot.SetInfo(shootCopy)

				kubeAPIServer, err := botanist.DefaultKubeAPIServer(ctx)
				Expect(err).NotTo(HaveOccurred())
				Expect(kubeAPIServer.GetValues().APIAudiences).To(Equal(apiAudiences))
			})
		})

		Describe("AdmissionPlugins", func() {
			DescribeTable("should have the expected admission plugins config",
				func(configuredPlugins, expectedPlugins []gardencorev1beta1.AdmissionPlugin) {
					shootCopy := botanist.Shoot.GetInfo().DeepCopy()
					shootCopy.Spec.Kubernetes = gardencorev1beta1.Kubernetes{
						KubeAPIServer: &gardencorev1beta1.KubeAPIServerConfig{
							AdmissionPlugins: configuredPlugins,
						},
						Version: "1.22.1",
					}
					botanist.Shoot.SetInfo(shootCopy)

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
						shootCopy := botanist.Shoot.GetInfo().DeepCopy()
						shootCopy.Spec.Kubernetes = gardencorev1beta1.Kubernetes{
							KubeAPIServer: &gardencorev1beta1.KubeAPIServerConfig{},
						}
						botanist.Shoot.SetInfo(shootCopy)
					},
					nil,
					Not(HaveOccurred()),
				),
				Entry("AuditPolicy is nil",
					func() {
						shootCopy := botanist.Shoot.GetInfo().DeepCopy()
						shootCopy.Spec.Kubernetes = gardencorev1beta1.Kubernetes{
							KubeAPIServer: &gardencorev1beta1.KubeAPIServerConfig{
								AuditConfig: &gardencorev1beta1.AuditConfig{},
							},
						}
						botanist.Shoot.SetInfo(shootCopy)
					},
					nil,
					Not(HaveOccurred()),
				),
				Entry("ConfigMapRef is nil",
					func() {
						shootCopy := botanist.Shoot.GetInfo().DeepCopy()
						shootCopy.Spec.Kubernetes = gardencorev1beta1.Kubernetes{
							KubeAPIServer: &gardencorev1beta1.KubeAPIServerConfig{
								AuditConfig: &gardencorev1beta1.AuditConfig{
									AuditPolicy: &gardencorev1beta1.AuditPolicy{},
								},
							},
						}
						botanist.Shoot.SetInfo(shootCopy)
					},
					nil,
					Not(HaveOccurred()),
				),
				Entry("ConfigMapRef is provided but configmap is missing",
					func() {
						shootCopy := botanist.Shoot.GetInfo().DeepCopy()
						shootCopy.Spec.Kubernetes = gardencorev1beta1.Kubernetes{
							KubeAPIServer: &gardencorev1beta1.KubeAPIServerConfig{
								AuditConfig: &gardencorev1beta1.AuditConfig{
									AuditPolicy: &gardencorev1beta1.AuditPolicy{
										ConfigMapRef: &corev1.ObjectReference{
											Name: auditPolicyConfigMap.Name,
										},
									},
								},
							},
						}
						botanist.Shoot.SetInfo(shootCopy)
					},
					nil,
					MatchError(ContainSubstring("not found")),
				),
				Entry("ConfigMapRef is provided but configmap is missing while shoot has a deletion timestamp",
					func() {
						shootCopy := botanist.Shoot.GetInfo().DeepCopy()
						shootCopy.DeletionTimestamp = &metav1.Time{}
						shootCopy.Spec.Kubernetes = gardencorev1beta1.Kubernetes{
							KubeAPIServer: &gardencorev1beta1.KubeAPIServerConfig{
								AuditConfig: &gardencorev1beta1.AuditConfig{
									AuditPolicy: &gardencorev1beta1.AuditPolicy{
										ConfigMapRef: &corev1.ObjectReference{
											Name: auditPolicyConfigMap.Name,
										},
									},
								},
							},
						}
						botanist.Shoot.SetInfo(shootCopy)
					},
					&kubeapiserver.AuditConfig{},
					Not(HaveOccurred()),
				),
				Entry("ConfigMapRef is provided but configmap does not have correct data field",
					func() {
						auditPolicyConfigMap.Data = nil
						Expect(gc.Create(ctx, auditPolicyConfigMap)).To(Succeed())

						shootCopy := botanist.Shoot.GetInfo().DeepCopy()
						shootCopy.Spec.Kubernetes = gardencorev1beta1.Kubernetes{
							KubeAPIServer: &gardencorev1beta1.KubeAPIServerConfig{
								AuditConfig: &gardencorev1beta1.AuditConfig{
									AuditPolicy: &gardencorev1beta1.AuditPolicy{
										ConfigMapRef: &corev1.ObjectReference{
											Name: auditPolicyConfigMap.Name,
										},
									},
								},
							},
						}
						botanist.Shoot.SetInfo(shootCopy)
					},
					nil,
					MatchError(ContainSubstring("missing '.data.policy' in audit policy configmap")),
				),
				Entry("ConfigMapRef is provided and configmap is compliant",
					func() {
						Expect(gc.Create(ctx, auditPolicyConfigMap)).To(Succeed())

						shootCopy := botanist.Shoot.GetInfo().DeepCopy()
						shootCopy.Spec.Kubernetes = gardencorev1beta1.Kubernetes{
							KubeAPIServer: &gardencorev1beta1.KubeAPIServerConfig{
								AuditConfig: &gardencorev1beta1.AuditConfig{
									AuditPolicy: &gardencorev1beta1.AuditPolicy{
										ConfigMapRef: &corev1.ObjectReference{
											Name: auditPolicyConfigMap.Name,
										},
									},
								},
							},
						}
						botanist.Shoot.SetInfo(shootCopy)
					},
					&kubeapiserver.AuditConfig{
						Policy: &policy,
					},
					Not(HaveOccurred()),
				),
			)
		})

		Describe("ZoneSpreadConfig", func() {
			DescribeTable("should have the expected zoneSpread config",
				func(prepTest func(), featureGate *featuregate.Feature, value *bool, enabled bool) {
					if prepTest != nil {
						prepTest()
					}

					if featureGate != nil && value != nil {
						defer test.WithFeatureGate(gardenletfeatures.FeatureGate, *featureGate, *value)()
					}

					kubeAPIServer, err := botanist.DefaultKubeAPIServer(ctx)
					Expect(err).NotTo(HaveOccurred())
					Expect(kubeAPIServer.GetValues().ZoneSpread).To(Equal(enabled))
				},
				Entry("when HAControlPlanes feature gate is disabled and annotation is not set",
					func() {
						botanist.Shoot.GetInfo().Annotations = nil
					},
					featureGatePtr(features.HAControlPlanes),
					pointer.Bool(false),
					false,
				),
				Entry("when HAControlPlanes feature gate is disabled and annotation is set",
					func() {
						botanist.Shoot.GetInfo().Annotations = map[string]string{
							v1beta1constants.ShootAlphaControlPlaneHighAvailability: v1beta1constants.ShootAlphaControlPlaneHighAvailabilityMultiZone,
						}
					},
					featureGatePtr(features.HAControlPlanes),
					pointer.Bool(false),
					false,
				),
				Entry("when HAControlPlanes feature gate is enabled and annotation is not set",
					func() {
						botanist.Shoot.GetInfo().Annotations = nil
					},
					featureGatePtr(features.HAControlPlanes),
					pointer.Bool(true),
					false,
				),
				Entry("when HAControlPlanes feature gate is enabled and annotation is set",
					func() {
						botanist.Shoot.GetInfo().Annotations = map[string]string{
							v1beta1constants.ShootAlphaControlPlaneHighAvailability: v1beta1constants.ShootAlphaControlPlaneHighAvailabilityMultiZone,
						}
					},
					featureGatePtr(features.HAControlPlanes),
					pointer.Bool(true),
					true,
				),
				Entry("when HAControlPlanes feature gate is enabled and annotation is set to any value but multi-zone",
					func() {
						botanist.Shoot.GetInfo().Annotations = map[string]string{
							v1beta1constants.ShootAlphaControlPlaneHighAvailability: "foo",
						}
					},
					featureGatePtr(features.HAControlPlanes),
					pointer.Bool(true),
					false,
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
						},
						HVPAEnabled:               false,
						MinReplicas:               16,
						MaxReplicas:               32,
						Replicas:                  pointer.Int32(24),
						UseMemoryMetricForHvpaHPA: true,
						ScaleDownDisabledForHvpa:  false,
					},
				),
				Entry("shoot enables HA control planes",
					func() {
						botanist.Shoot.GetInfo().Annotations = map[string]string{
							v1beta1constants.ShootAlphaControlPlaneHighAvailability: "foo",
						}
					},
					featureGatePtr(features.HAControlPlanes), pointer.Bool(true),
					kubeapiserver.AutoscalingConfig{
						APIServerResources:        resourcesRequirementsForKubeAPIServer(0, ""),
						HVPAEnabled:               false,
						MinReplicas:               3,
						MaxReplicas:               4,
						UseMemoryMetricForHvpaHPA: false,
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

				shootCopy := botanist.Shoot.GetInfo().DeepCopy()
				shootCopy.Spec.Kubernetes = gardencorev1beta1.Kubernetes{
					KubeAPIServer: &gardencorev1beta1.KubeAPIServerConfig{
						EventTTL: eventTTL,
					},
				}
				botanist.Shoot.SetInfo(shootCopy)

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

				shootCopy := botanist.Shoot.GetInfo().DeepCopy()
				shootCopy.Spec.Kubernetes = gardencorev1beta1.Kubernetes{
					KubeAPIServer: &gardencorev1beta1.KubeAPIServerConfig{
						KubernetesConfig: gardencorev1beta1.KubernetesConfig{
							FeatureGates: featureGates,
						},
					},
				}
				botanist.Shoot.SetInfo(shootCopy)

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
						shootCopy := botanist.Shoot.GetInfo().DeepCopy()
						shootCopy.Spec.Kubernetes = gardencorev1beta1.Kubernetes{
							KubeAPIServer: &gardencorev1beta1.KubeAPIServerConfig{},
						}
						botanist.Shoot.SetInfo(shootCopy)
					},
					nil,
				),
				Entry("OIDCConfig is not nil",
					func() {
						shootCopy := botanist.Shoot.GetInfo().DeepCopy()
						shootCopy.Spec.Kubernetes = gardencorev1beta1.Kubernetes{
							KubeAPIServer: &gardencorev1beta1.KubeAPIServerConfig{
								OIDCConfig: &gardencorev1beta1.OIDCConfig{},
							},
						}
						botanist.Shoot.SetInfo(shootCopy)
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

				shootCopy := botanist.Shoot.GetInfo().DeepCopy()
				shootCopy.Spec.Kubernetes = gardencorev1beta1.Kubernetes{
					KubeAPIServer: &gardencorev1beta1.KubeAPIServerConfig{
						Requests: requests,
					},
				}
				botanist.Shoot.SetInfo(shootCopy)

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

				shootCopy := botanist.Shoot.GetInfo().DeepCopy()
				shootCopy.Spec.Kubernetes = gardencorev1beta1.Kubernetes{
					KubeAPIServer: &gardencorev1beta1.KubeAPIServerConfig{
						RuntimeConfig: runtimeConfig,
					},
				}
				botanist.Shoot.SetInfo(shootCopy)

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
						shootCopy := botanist.Shoot.GetInfo().DeepCopy()
						shootCopy.Spec.Networking.Nodes = nil
						botanist.Shoot.SetInfo(shootCopy)
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

				shootCopy := botanist.Shoot.GetInfo().DeepCopy()
				shootCopy.Spec.Kubernetes = gardencorev1beta1.Kubernetes{
					KubeAPIServer: &gardencorev1beta1.KubeAPIServerConfig{
						WatchCacheSizes: watchCacheSizes,
					},
				}
				botanist.Shoot.SetInfo(shootCopy)

				kubeAPIServer, err := botanist.DefaultKubeAPIServer(ctx)
				Expect(err).NotTo(HaveOccurred())
				Expect(kubeAPIServer.GetValues().WatchCacheSizes).To(Equal(watchCacheSizes))
			})
		})

	})

	DescribeTable("#resourcesRequirementsForKubeAPIServer",
		func(nodes int, storageClass, expectedCPURequest, expectedMemoryRequest string) {
			Expect(resourcesRequirementsForKubeAPIServer(int32(nodes), storageClass)).To(Equal(
				corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse(expectedCPURequest),
						corev1.ResourceMemory: resource.MustParse(expectedMemoryRequest),
					},
				}))
		},

		// nodes tests
		Entry("nodes <= 2", 2, "", "800m", "800Mi"),
		Entry("nodes <= 10", 10, "", "1000m", "1100Mi"),
		Entry("nodes <= 50", 50, "", "1200m", "1600Mi"),
		Entry("nodes <= 100", 100, "", "2500m", "5200Mi"),
		Entry("nodes > 100", 1000, "", "3000m", "5200Mi"),

		// scaling class tests
		Entry("scaling class small", -1, "small", "800m", "800Mi"),
		Entry("scaling class medium", -1, "medium", "1000m", "1100Mi"),
		Entry("scaling class large", -1, "large", "1200m", "1600Mi"),
		Entry("scaling class xlarge", -1, "xlarge", "2500m", "5200Mi"),
		Entry("scaling class 2xlarge", -1, "2xlarge", "3000m", "5200Mi"),

		// scaling class always decides if provided
		Entry("nodes > 100, scaling class small", 100, "small", "800m", "800Mi"),
		Entry("nodes <= 100, scaling class medium", 100, "medium", "1000m", "1100Mi"),
		Entry("nodes <= 50, scaling class large", 50, "large", "1200m", "1600Mi"),
		Entry("nodes <= 10, scaling class xlarge", 10, "xlarge", "2500m", "5200Mi"),
		Entry("nodes <= 2, scaling class 2xlarge", 2, "2xlarge", "3000m", "5200Mi"),
	)

	Describe("#DeployKubeAPIServer", func() {
		DescribeTable("should correctly set the autoscaling replicas",
			func(prepTest func(), autoscalingConfig kubeapiserver.AutoscalingConfig, expectedReplicas int32) {
				if prepTest != nil {
					prepTest()
				}

				kubeAPIServer.EXPECT().GetValues().Return(kubeapiserver.Values{Autoscaling: autoscalingConfig})
				kubeAPIServer.EXPECT().SetAutoscalingReplicas(&expectedReplicas)
				kubeAPIServer.EXPECT().SetSNIConfig(gomock.Any())
				kubeAPIServer.EXPECT().SetETCDEncryptionConfig(gomock.Any())
				kubeAPIServer.EXPECT().SetExternalHostname(gomock.Any())
				kubeAPIServer.EXPECT().SetExternalServer(gomock.Any())
				kubeAPIServer.EXPECT().SetServerCertificateConfig(gomock.Any())
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
							Namespace: seedNamespace,
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
							Namespace: seedNamespace,
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
				kubeAPIServer.EXPECT().SetSNIConfig(gomock.Any())
				kubeAPIServer.EXPECT().SetETCDEncryptionConfig(gomock.Any())
				kubeAPIServer.EXPECT().SetExternalHostname(gomock.Any())
				kubeAPIServer.EXPECT().SetExternalServer(gomock.Any())
				kubeAPIServer.EXPECT().SetServerCertificateConfig(gomock.Any())
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
							Namespace: seedNamespace,
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
							Namespace: seedNamespace,
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

		DescribeTable("ETCDEncryptionConfig",
			func(rotationPhase gardencorev1beta1.ShootCredentialsRotationPhase, prepTest func(), expectedETCDEncryptionConfig kubeapiserver.ETCDEncryptionConfig, finalizeTest func()) {
				if len(rotationPhase) > 0 {
					shootCopy := botanist.Shoot.GetInfo().DeepCopy()
					shootCopy.Status.Credentials = &gardencorev1beta1.ShootCredentials{
						Rotation: &gardencorev1beta1.ShootCredentialsRotation{
							ETCDEncryptionKey: &gardencorev1beta1.ShootETCDEncryptionKeyRotation{
								Phase: rotationPhase,
							},
						},
					}
					botanist.Shoot.SetInfo(shootCopy)
				}

				if prepTest != nil {
					prepTest()
				}

				kubeAPIServer.EXPECT().GetValues()
				kubeAPIServer.EXPECT().SetAutoscalingReplicas(gomock.Any())
				kubeAPIServer.EXPECT().SetSNIConfig(gomock.Any())
				kubeAPIServer.EXPECT().SetETCDEncryptionConfig(expectedETCDEncryptionConfig)
				kubeAPIServer.EXPECT().SetExternalHostname(gomock.Any())
				kubeAPIServer.EXPECT().SetExternalServer(gomock.Any())
				kubeAPIServer.EXPECT().SetServerCertificateConfig(gomock.Any())
				kubeAPIServer.EXPECT().SetServiceAccountConfig(gomock.Any())
				kubeAPIServer.EXPECT().Deploy(ctx)

				Expect(botanist.DeployKubeAPIServer(ctx)).To(Succeed())

				if finalizeTest != nil {
					finalizeTest()
				}
			},

			Entry("no rotation",
				gardencorev1beta1.ShootCredentialsRotationPhase(""),
				nil,
				kubeapiserver.ETCDEncryptionConfig{EncryptWithCurrentKey: true},
				nil,
			),
			Entry("preparing phase, new key already populated",
				gardencorev1beta1.RotationPreparing,
				func() {
					Expect(c.Create(ctx, &appsv1.Deployment{
						ObjectMeta: metav1.ObjectMeta{
							Name:        "kube-apiserver",
							Namespace:   seedNamespace,
							Annotations: map[string]string{"credentials.gardener.cloud/new-encryption-key-populated": "true"},
						},
					})).To(Succeed())
				},
				kubeapiserver.ETCDEncryptionConfig{RotationPhase: gardencorev1beta1.RotationPreparing, EncryptWithCurrentKey: true},
				nil,
			),
			Entry("preparing phase, new key not yet populated",
				gardencorev1beta1.RotationPreparing,
				func() {
					Expect(c.Create(ctx, &appsv1.Deployment{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "kube-apiserver",
							Namespace: seedNamespace,
						},
					})).To(Succeed())

					kubeAPIServer.EXPECT().Wait(ctx)

					kubeAPIServer.EXPECT().SetETCDEncryptionConfig(kubeapiserver.ETCDEncryptionConfig{
						RotationPhase:         gardencorev1beta1.RotationPreparing,
						EncryptWithCurrentKey: true,
					})
					kubeAPIServer.EXPECT().Deploy(ctx)
				},
				kubeapiserver.ETCDEncryptionConfig{RotationPhase: gardencorev1beta1.RotationPreparing, EncryptWithCurrentKey: false},
				func() {
					deployment := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "kube-apiserver", Namespace: seedNamespace}}
					Expect(c.Get(ctx, client.ObjectKeyFromObject(deployment), deployment)).To(Succeed())
					Expect(deployment.Annotations).To(HaveKeyWithValue("credentials.gardener.cloud/new-encryption-key-populated", "true"))
				},
			),
			Entry("prepared phase",
				gardencorev1beta1.RotationPrepared,
				nil,
				kubeapiserver.ETCDEncryptionConfig{RotationPhase: gardencorev1beta1.RotationPrepared, EncryptWithCurrentKey: true},
				nil,
			),
			Entry("completing phase",
				gardencorev1beta1.RotationCompleting,
				func() {
					Expect(c.Create(ctx, &appsv1.Deployment{
						ObjectMeta: metav1.ObjectMeta{
							Name:        "kube-apiserver",
							Namespace:   seedNamespace,
							Annotations: map[string]string{"credentials.gardener.cloud/new-encryption-key-populated": "true"},
						},
					})).To(Succeed())
				},
				kubeapiserver.ETCDEncryptionConfig{RotationPhase: gardencorev1beta1.RotationCompleting, EncryptWithCurrentKey: true},
				func() {
					deployment := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "kube-apiserver", Namespace: seedNamespace}}
					Expect(c.Get(ctx, client.ObjectKeyFromObject(deployment), deployment)).To(Succeed())
					Expect(deployment.Annotations).NotTo(HaveKey("credentials.gardener.cloud/new-encryption-key-populated"))
				},
			),
			Entry("completed phase",
				gardencorev1beta1.RotationCompleted,
				nil,
				kubeapiserver.ETCDEncryptionConfig{RotationPhase: gardencorev1beta1.RotationCompleted, EncryptWithCurrentKey: true},
				nil,
			),
		)

		Describe("ExternalHostname", func() {
			It("should set the external hostname to the out-of-cluster address (internal domain)", func() {
				kubeAPIServer.EXPECT().GetValues()
				kubeAPIServer.EXPECT().SetAutoscalingReplicas(gomock.Any())
				kubeAPIServer.EXPECT().SetSNIConfig(gomock.Any())
				kubeAPIServer.EXPECT().SetETCDEncryptionConfig(gomock.Any())
				kubeAPIServer.EXPECT().SetExternalHostname("api." + internalClusterDomain)
				kubeAPIServer.EXPECT().SetExternalServer(gomock.Any())
				kubeAPIServer.EXPECT().SetServerCertificateConfig(gomock.Any())
				kubeAPIServer.EXPECT().SetServiceAccountConfig(gomock.Any())
				kubeAPIServer.EXPECT().Deploy(ctx)

				Expect(botanist.DeployKubeAPIServer(ctx)).To(Succeed())
			})
		})

		Describe("ServerCertificateConfig", func() {
			DescribeTable("should have the expected ServerCertificateConfig config",
				func(prepTest func(), expectedConfig kubeapiserver.ServerCertificateConfig) {
					if prepTest != nil {
						prepTest()
					}

					kubeAPIServer.EXPECT().GetValues()
					kubeAPIServer.EXPECT().SetAutoscalingReplicas(gomock.Any())
					kubeAPIServer.EXPECT().SetSNIConfig(gomock.Any())
					kubeAPIServer.EXPECT().SetETCDEncryptionConfig(gomock.Any())
					kubeAPIServer.EXPECT().SetExternalHostname(gomock.Any())
					kubeAPIServer.EXPECT().SetExternalServer(gomock.Any())
					kubeAPIServer.EXPECT().SetServerCertificateConfig(expectedConfig)
					kubeAPIServer.EXPECT().SetServiceAccountConfig(gomock.Any())
					kubeAPIServer.EXPECT().Deploy(ctx)

					Expect(botanist.DeployKubeAPIServer(ctx)).To(Succeed())
				},

				Entry("seed enables DNS, shoot has external domain",
					nil,
					kubeapiserver.ServerCertificateConfig{
						ExtraIPAddresses: []net.IP{apiServerNetwork},
						ExtraDNSNames: []string{
							"api." + internalClusterDomain,
							seedNamespace,
							externalClusterDomain,
							"api." + externalClusterDomain,
						},
					},
				),
				Entry("seed enables DNS, shoot has no external domain",
					func() {
						botanist.Shoot.DisableDNS = true
						botanist.Shoot.ExternalClusterDomain = nil
					},
					kubeapiserver.ServerCertificateConfig{
						ExtraIPAddresses: []net.IP{apiServerNetwork},
						ExtraDNSNames: []string{
							"api." + internalClusterDomain,
							seedNamespace,
						},
					},
				),
				Entry("seed disables DNS, api server address is IP",
					func() {
						seedCopy := botanist.Seed.GetInfo().DeepCopy()
						seedCopy.Spec.Settings.ShootDNS.Enabled = false
						botanist.Seed.SetInfo(seedCopy)
					},
					kubeapiserver.ServerCertificateConfig{
						ExtraIPAddresses: []net.IP{apiServerNetwork, net.ParseIP(apiServerAddress)},
						ExtraDNSNames: []string{
							"api." + internalClusterDomain,
							seedNamespace,
							externalClusterDomain,
							"api." + externalClusterDomain,
						},
					},
				),
				Entry("seed disables DNS, api server address is hostname",
					func() {
						seedCopy := botanist.Seed.GetInfo().DeepCopy()
						seedCopy.Spec.Settings.ShootDNS.Enabled = false
						botanist.Seed.SetInfo(seedCopy)
						botanist.APIServerAddress = "some-hostname.com"
					},
					kubeapiserver.ServerCertificateConfig{
						ExtraIPAddresses: []net.IP{apiServerNetwork},
						ExtraDNSNames: []string{
							"api." + internalClusterDomain,
							seedNamespace,
							"some-hostname.com",
							externalClusterDomain,
							"api." + externalClusterDomain,
						},
					},
				),
			)
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
					kubeAPIServer.EXPECT().SetSNIConfig(gomock.Any())
					kubeAPIServer.EXPECT().SetExternalHostname(gomock.Any())
					kubeAPIServer.EXPECT().SetExternalServer(gomock.Any())
					kubeAPIServer.EXPECT().SetServerCertificateConfig(gomock.Any())
					if !expectError {
						kubeAPIServer.EXPECT().SetServiceAccountConfig(expectedConfig)
						kubeAPIServer.EXPECT().SetETCDEncryptionConfig(gomock.Any())
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
						shootCopy := botanist.Shoot.GetInfo().DeepCopy()
						shootCopy.Spec.Kubernetes = gardencorev1beta1.Kubernetes{
							KubeAPIServer: &gardencorev1beta1.KubeAPIServerConfig{},
						}
						botanist.Shoot.SetInfo(shootCopy)
					},
					kubeapiserver.ServiceAccountConfig{Issuer: "https://api." + internalClusterDomain},
					false,
					Not(HaveOccurred()),
				),
				Entry("service account key rotation phase is set",
					func() {
						shootCopy := botanist.Shoot.GetInfo().DeepCopy()
						shootCopy.Spec.Kubernetes = gardencorev1beta1.Kubernetes{
							KubeAPIServer: &gardencorev1beta1.KubeAPIServerConfig{},
						}
						shootCopy.Status.Credentials = &gardencorev1beta1.ShootCredentials{
							Rotation: &gardencorev1beta1.ShootCredentialsRotation{
								ServiceAccountKey: &gardencorev1beta1.ShootServiceAccountKeyRotation{
									Phase: gardencorev1beta1.RotationCompleting,
								},
							},
						}
						botanist.Shoot.SetInfo(shootCopy)
					},
					kubeapiserver.ServiceAccountConfig{
						Issuer:        "https://api." + internalClusterDomain,
						RotationPhase: gardencorev1beta1.RotationCompleting,
					},
					false,
					Not(HaveOccurred()),
				),
				Entry("Issuer is not provided",
					func() {
						shootCopy := botanist.Shoot.GetInfo().DeepCopy()
						shootCopy.Spec.Kubernetes = gardencorev1beta1.Kubernetes{
							KubeAPIServer: &gardencorev1beta1.KubeAPIServerConfig{
								ServiceAccountConfig: &gardencorev1beta1.ServiceAccountConfig{
									ExtendTokenExpiration: &extendTokenExpiration,
									MaxTokenExpiration:    &maxTokenExpiration,
								},
							},
						}
						botanist.Shoot.SetInfo(shootCopy)
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
						shootCopy := botanist.Shoot.GetInfo().DeepCopy()
						shootCopy.Spec.Kubernetes = gardencorev1beta1.Kubernetes{
							KubeAPIServer: &gardencorev1beta1.KubeAPIServerConfig{
								ServiceAccountConfig: &gardencorev1beta1.ServiceAccountConfig{
									Issuer: pointer.String("issuer"),
								},
							},
						}
						botanist.Shoot.SetInfo(shootCopy)
					},
					kubeapiserver.ServiceAccountConfig{
						Issuer:          "issuer",
						AcceptedIssuers: []string{"https://api." + internalClusterDomain},
					},
					false,
					Not(HaveOccurred()),
				),
				Entry("AcceptedIssuers is provided and Issuer is not",
					func() {
						shootCopy := botanist.Shoot.GetInfo().DeepCopy()
						shootCopy.Spec.Kubernetes = gardencorev1beta1.Kubernetes{
							KubeAPIServer: &gardencorev1beta1.KubeAPIServerConfig{
								ServiceAccountConfig: &gardencorev1beta1.ServiceAccountConfig{
									AcceptedIssuers: []string{"issuer1", "issuer2"},
								},
							},
						}
						botanist.Shoot.SetInfo(shootCopy)
					},
					kubeapiserver.ServiceAccountConfig{
						Issuer:          "https://api." + internalClusterDomain,
						AcceptedIssuers: []string{"issuer1", "issuer2"},
					},
					false,
					Not(HaveOccurred()),
				),
				Entry("AcceptedIssuers and Issuer are provided",
					func() {
						shootCopy := botanist.Shoot.GetInfo().DeepCopy()
						shootCopy.Spec.Kubernetes = gardencorev1beta1.Kubernetes{
							KubeAPIServer: &gardencorev1beta1.KubeAPIServerConfig{
								ServiceAccountConfig: &gardencorev1beta1.ServiceAccountConfig{
									Issuer:          pointer.String("issuer"),
									AcceptedIssuers: []string{"issuer1", "issuer2"},
								},
							},
						}
						botanist.Shoot.SetInfo(shootCopy)
					},
					kubeapiserver.ServiceAccountConfig{
						Issuer:          "issuer",
						AcceptedIssuers: []string{"issuer1", "issuer2", "https://api." + internalClusterDomain},
					},
					false,
					Not(HaveOccurred()),
				),
				Entry("Default Issuer is already part of AcceptedIssuers",
					func() {
						shootCopy := botanist.Shoot.GetInfo().DeepCopy()
						shootCopy.Spec.Kubernetes = gardencorev1beta1.Kubernetes{
							KubeAPIServer: &gardencorev1beta1.KubeAPIServerConfig{
								ServiceAccountConfig: &gardencorev1beta1.ServiceAccountConfig{
									Issuer:          pointer.String("issuer"),
									AcceptedIssuers: []string{"https://api." + internalClusterDomain},
								},
							},
						}
						botanist.Shoot.SetInfo(shootCopy)
					},
					kubeapiserver.ServiceAccountConfig{
						Issuer:          "issuer",
						AcceptedIssuers: []string{"https://api." + internalClusterDomain},
					},
					false,
					Not(HaveOccurred()),
				),
				Entry("AcceptedIssuers is not provided",
					func() {
						shootCopy := botanist.Shoot.GetInfo().DeepCopy()
						shootCopy.Spec.Kubernetes = gardencorev1beta1.Kubernetes{
							KubeAPIServer: &gardencorev1beta1.KubeAPIServerConfig{
								ServiceAccountConfig: &gardencorev1beta1.ServiceAccountConfig{},
							},
						}
						botanist.Shoot.SetInfo(shootCopy)
					},
					kubeapiserver.ServiceAccountConfig{Issuer: "https://api." + internalClusterDomain},
					false,
					Not(HaveOccurred()),
				),
				Entry("SigningKeySecret is nil",
					func() {
						shootCopy := botanist.Shoot.GetInfo().DeepCopy()
						shootCopy.Spec.Kubernetes = gardencorev1beta1.Kubernetes{
							KubeAPIServer: &gardencorev1beta1.KubeAPIServerConfig{
								ServiceAccountConfig: &gardencorev1beta1.ServiceAccountConfig{},
							},
						}
						botanist.Shoot.SetInfo(shootCopy)
					},
					kubeapiserver.ServiceAccountConfig{Issuer: "https://api." + internalClusterDomain},
					false,
					Not(HaveOccurred()),
				),
				Entry("SigningKeySecret is provided but secret is missing",
					func() {
						shootCopy := botanist.Shoot.GetInfo().DeepCopy()
						shootCopy.Spec.Kubernetes = gardencorev1beta1.Kubernetes{
							KubeAPIServer: &gardencorev1beta1.KubeAPIServerConfig{
								ServiceAccountConfig: &gardencorev1beta1.ServiceAccountConfig{
									SigningKeySecret: &corev1.LocalObjectReference{
										Name: signingKeySecret.Name,
									},
								},
							},
						}
						botanist.Shoot.SetInfo(shootCopy)
					},
					nil,
					true,
					MatchError(ContainSubstring("not found")),
				),
				Entry("SigningKeySecret is provided but secret does not have correct data field",
					func() {
						signingKeySecret.Data = nil
						Expect(gc.Create(ctx, signingKeySecret)).To(Succeed())

						shootCopy := botanist.Shoot.GetInfo().DeepCopy()
						shootCopy.Spec.Kubernetes = gardencorev1beta1.Kubernetes{
							KubeAPIServer: &gardencorev1beta1.KubeAPIServerConfig{
								ServiceAccountConfig: &gardencorev1beta1.ServiceAccountConfig{
									SigningKeySecret: &corev1.LocalObjectReference{
										Name: signingKeySecret.Name,
									},
								},
							},
						}
						botanist.Shoot.SetInfo(shootCopy)
					},
					kubeapiserver.ServiceAccountConfig{Issuer: "https://api." + internalClusterDomain},
					true,
					MatchError(ContainSubstring("no signing key in secret")),
				),
				Entry("SigningKeySecret is provided and secret is compliant",
					func() {
						Expect(gc.Create(ctx, signingKeySecret)).To(Succeed())

						shootCopy := botanist.Shoot.GetInfo().DeepCopy()
						shootCopy.Spec.Kubernetes = gardencorev1beta1.Kubernetes{
							KubeAPIServer: &gardencorev1beta1.KubeAPIServerConfig{
								ServiceAccountConfig: &gardencorev1beta1.ServiceAccountConfig{
									SigningKeySecret: &corev1.LocalObjectReference{
										Name: signingKeySecret.Name,
									},
								},
							},
						}
						botanist.Shoot.SetInfo(shootCopy)
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
					kubeAPIServer.EXPECT().SetSNIConfig(expectedConfig)
					kubeAPIServer.EXPECT().SetETCDEncryptionConfig(gomock.Any())
					kubeAPIServer.EXPECT().SetExternalHostname(gomock.Any())
					kubeAPIServer.EXPECT().SetExternalServer(gomock.Any())
					kubeAPIServer.EXPECT().SetServerCertificateConfig(gomock.Any())
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
						botanist.Shoot.DisableDNS = true
						botanist.Shoot.ExternalClusterDomain = nil
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

		Describe("ExternalServer", func() {
			It("should set the external server to the out-of-cluster address (no internal domain)", func() {
				kubeAPIServer.EXPECT().GetValues()
				kubeAPIServer.EXPECT().SetAutoscalingReplicas(gomock.Any())
				kubeAPIServer.EXPECT().SetSNIConfig(gomock.Any())
				kubeAPIServer.EXPECT().SetETCDEncryptionConfig(gomock.Any())
				kubeAPIServer.EXPECT().SetExternalHostname(gomock.Any())
				kubeAPIServer.EXPECT().SetExternalServer("api." + externalClusterDomain)
				kubeAPIServer.EXPECT().SetServerCertificateConfig(gomock.Any())
				kubeAPIServer.EXPECT().SetServiceAccountConfig(gomock.Any())
				kubeAPIServer.EXPECT().Deploy(ctx)

				Expect(botanist.DeployKubeAPIServer(ctx)).To(Succeed())
			})
		})

		It("should sync the kubeconfig to the garden project namespace when enableStaticTokenKubeconfig is set to true", func() {
			kubeAPIServer.EXPECT().GetValues()
			kubeAPIServer.EXPECT().SetAutoscalingReplicas(gomock.Any())
			kubeAPIServer.EXPECT().SetSNIConfig(gomock.Any())
			kubeAPIServer.EXPECT().SetETCDEncryptionConfig(gomock.Any())
			kubeAPIServer.EXPECT().SetExternalHostname(gomock.Any())
			kubeAPIServer.EXPECT().SetExternalServer(gomock.Any())
			kubeAPIServer.EXPECT().SetServerCertificateConfig(gomock.Any())
			kubeAPIServer.EXPECT().SetServiceAccountConfig(gomock.Any())
			kubeAPIServer.EXPECT().Deploy(ctx)

			Expect(gc.Get(ctx, kutil.Key(projectNamespace, shootName+".kubeconfig"), &corev1.Secret{})).To(BeNotFoundError())

			Expect(botanist.DeployKubeAPIServer(ctx)).To(Succeed())

			kubeconfigSecret := &corev1.Secret{}
			Expect(gc.Get(ctx, kutil.Key(projectNamespace, shootName+".kubeconfig"), kubeconfigSecret)).To(Succeed())
			Expect(kubeconfigSecret.Annotations).To(HaveKeyWithValue("url", "https://api."+externalClusterDomain))
			Expect(kubeconfigSecret.Labels).To(HaveKeyWithValue("gardener.cloud/role", "kubeconfig"))
			Expect(kubeconfigSecret.Data).To(And(
				HaveKey("ca.crt"),
				HaveKeyWithValue("data-for", []byte("user-kubeconfig")),
			))
		})

		It("should delete the old etcd encryption config secret", func() {
			kubeAPIServer.EXPECT().GetValues()
			kubeAPIServer.EXPECT().SetAutoscalingReplicas(gomock.Any())
			kubeAPIServer.EXPECT().SetSNIConfig(gomock.Any())
			kubeAPIServer.EXPECT().SetETCDEncryptionConfig(gomock.Any())
			kubeAPIServer.EXPECT().SetExternalHostname(gomock.Any())
			kubeAPIServer.EXPECT().SetExternalServer(gomock.Any())
			kubeAPIServer.EXPECT().SetServerCertificateConfig(gomock.Any())
			kubeAPIServer.EXPECT().SetServiceAccountConfig(gomock.Any())
			kubeAPIServer.EXPECT().Deploy(ctx)

			secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: seedNamespace, Name: "etcd-encryption-secret"}}
			Expect(c.Create(ctx, secret)).To(Succeed())

			Expect(botanist.DeployKubeAPIServer(ctx)).To(Succeed())

			Expect(c.Get(ctx, client.ObjectKeyFromObject(secret), &corev1.Secret{})).To(BeNotFoundError())
		})

		It("should not sync the kubeconfig to garden project namespace when enableStaticTokenKubeconfig is set to false", func() {
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      shootName + ".kubeconfig",
					Namespace: projectNamespace,
				},
			}
			Expect(gc.Create(ctx, secret)).To(Succeed())

			Expect(gc.Get(ctx, kutil.Key(projectNamespace, shootName+".kubeconfig"), &corev1.Secret{})).To(Succeed())

			kubeAPIServer.EXPECT().GetValues()
			kubeAPIServer.EXPECT().SetAutoscalingReplicas(gomock.Any())
			kubeAPIServer.EXPECT().SetSNIConfig(gomock.Any())
			kubeAPIServer.EXPECT().SetETCDEncryptionConfig(gomock.Any())
			kubeAPIServer.EXPECT().SetExternalHostname(gomock.Any())
			kubeAPIServer.EXPECT().SetExternalServer(gomock.Any())
			kubeAPIServer.EXPECT().SetServerCertificateConfig(gomock.Any())
			kubeAPIServer.EXPECT().SetServiceAccountConfig(gomock.Any())
			kubeAPIServer.EXPECT().Deploy(ctx)

			shootCopy := botanist.Shoot.GetInfo().DeepCopy()
			shootCopy.Spec.Kubernetes = gardencorev1beta1.Kubernetes{
				KubeAPIServer: &gardencorev1beta1.KubeAPIServerConfig{
					ServiceAccountConfig: &gardencorev1beta1.ServiceAccountConfig{
						Issuer:          pointer.String("issuer"),
						AcceptedIssuers: []string{"issuer1", "issuer2"},
					},
				},
				EnableStaticTokenKubeconfig: pointer.Bool(false),
			}
			botanist.Shoot.SetInfo(shootCopy)

			Expect(botanist.DeployKubeAPIServer(ctx)).To(Succeed())

			Expect(gc.Get(ctx, kutil.Key(projectNamespace, shootName+".kubeconfig"), &corev1.Secret{})).To(BeNotFoundError())
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
			Expect(err).To(MatchError(`clientSet for key "` + botanist.Shoot.GetInfo().Namespace + `/` + botanist.Shoot.GetInfo().Name + `" not found`))
			Expect(shootClient).To(BeNil())

			Expect(botanist.K8sShootClient).To(BeNil())
		})
	})

	Describe("#ScaleKubeAPIServerToOne", func() {
		deployment := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "kube-apiserver", Namespace: seedNamespace}}

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
