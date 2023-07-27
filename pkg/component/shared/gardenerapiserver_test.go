// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package shared

import (
	"context"

	"github.com/Masterminds/semver"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component/apiserver"
	mockgardenerapiserver "github.com/gardener/gardener/pkg/component/gardenerapiserver/mock"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	fakesecretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager/fake"
)

var _ = Describe("GardenerAPIServer", func() {
	var (
		ctx = context.TODO()

		runtimeClient               client.Client
		namespace                   = "foo"
		clusterIdentity             = "cluster-id"
		topologyAwareRoutingEnabled = false
		apiServerConfig             *operatorv1alpha1.GardenerAPIServerConfig
	)

	BeforeEach(func() {
		runtimeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		apiServerConfig = nil
	})

	Describe("#NewGardenerAPIServer", func() {
		var (
			name               string
			objectMeta         metav1.ObjectMeta
			secret             *corev1.Secret
			runtimeVersion     *semver.Version
			autoscalingConfig  apiserver.AutoscalingConfig
			auditWebhookConfig *apiserver.AuditWebhook
			sm                 secretsmanager.Interface
		)

		BeforeEach(func() {
			name = "bar"
			objectMeta = metav1.ObjectMeta{Namespace: namespace, Name: name}
			runtimeVersion = semver.MustParse("1.27.0")
			autoscalingConfig = apiserver.AutoscalingConfig{}
			auditWebhookConfig = nil

			secret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "secret-1",
					Namespace: namespace,
				},
				Data: map[string][]byte{"kubeconfig": []byte("kubeconfig-data")},
			}

			sm = fakesecretsmanager.New(runtimeClient, namespace)
		})

		Describe("AdmissionPlugins", func() {
			BeforeEach(func() {
				Expect(runtimeClient.Create(ctx, secret)).To(BeNil())
				apiServerConfig = &operatorv1alpha1.GardenerAPIServerConfig{AdmissionPlugins: []gardencorev1beta1.AdmissionPlugin{}}
			})

			DescribeTable("should have the expected admission plugins config",
				func(configuredPlugins []gardencorev1beta1.AdmissionPlugin, expectedPlugins []apiserver.AdmissionPluginConfig) {
					apiServerConfig.AdmissionPlugins = configuredPlugins

					gardenerAPIServer, err := NewGardenerAPIServer(ctx, runtimeClient, namespace, objectMeta, runtimeVersion, sm, apiServerConfig, autoscalingConfig, auditWebhookConfig, topologyAwareRoutingEnabled, clusterIdentity)
					Expect(err).NotTo(HaveOccurred())
					Expect(gardenerAPIServer.GetValues().EnabledAdmissionPlugins).To(Equal(expectedPlugins))
				},

				Entry("only default plugins",
					nil,
					nil,
				),
				Entry("default plugins and other plugins",
					[]gardencorev1beta1.AdmissionPlugin{
						{Name: "Foo"},
						{Name: "Bar"},
						{Name: "Baz", Config: &runtime.RawExtension{Raw: []byte("baz-config")}},
					},
					[]apiserver.AdmissionPluginConfig{
						{AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{Name: "Foo"}},
						{AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{Name: "Bar"}},
						{AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{Name: "Baz", Config: &runtime.RawExtension{Raw: []byte("baz-config")}}},
					},
				),
				Entry("default plugins and skipping configured plugins if disabled",
					[]gardencorev1beta1.AdmissionPlugin{
						{Name: "Foo"},
						{Name: "Bar", Disabled: pointer.Bool(true)},
						{Name: "Baz", Config: &runtime.RawExtension{Raw: []byte("baz-config")}, Disabled: pointer.Bool(true)},
					},
					[]apiserver.AdmissionPluginConfig{
						{AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{Name: "Foo"}},
					},
				),
			)

			Context("should have the expected disabled admission plugins", func() {
				var expectedDisabledPlugins []gardencorev1beta1.AdmissionPlugin

				AfterEach(func() {
					gardenerAPIServer, err := NewGardenerAPIServer(ctx, runtimeClient, namespace, objectMeta, runtimeVersion, sm, apiServerConfig, autoscalingConfig, auditWebhookConfig, topologyAwareRoutingEnabled, clusterIdentity)
					Expect(err).NotTo(HaveOccurred())
					Expect(gardenerAPIServer.GetValues().DisabledAdmissionPlugins).To(Equal(expectedDisabledPlugins))
				})

				It("should return the correct list of disabled admission plugins", func() {
					apiServerConfig.AdmissionPlugins = []gardencorev1beta1.AdmissionPlugin{
						{Name: "Priority"},
						{Name: "NamespaceLifecycle", Config: &runtime.RawExtension{Raw: []byte("namespace-lifecycle-config")}},
						{Name: "LimitRanger"},
						{Name: "PodSecurityPolicy", Disabled: pointer.Bool(true)},
						{Name: "ServiceAccount"},
						{Name: "NodeRestriction"},
						{Name: "DefaultStorageClass"},
						{Name: "DefaultTolerationSeconds", Disabled: pointer.Bool(true)},
						{Name: "ResourceQuota"},
					}

					expectedDisabledPlugins = []gardencorev1beta1.AdmissionPlugin{
						{Name: "PodSecurityPolicy", Disabled: pointer.Bool(true)},
						{Name: "DefaultTolerationSeconds", Disabled: pointer.Bool(true)},
					}
				})

				It("should return the correct list of disabled admission plugins", func() {
					apiServerConfig.AdmissionPlugins = []gardencorev1beta1.AdmissionPlugin{
						{Name: "Priority"},
						{Name: "NamespaceLifecycle", Config: &runtime.RawExtension{Raw: []byte("namespace-lifecycle-config")}, Disabled: pointer.Bool(true)},
						{Name: "LimitRanger"},
						{Name: "PodSecurityPolicy"},
						{Name: "ServiceAccount"},
						{Name: "NodeRestriction"},
						{Name: "DefaultStorageClass", Disabled: pointer.Bool(true)},
						{Name: "DefaultTolerationSeconds"},
						{Name: "ResourceQuota"},
						{Name: "foo", Config: &runtime.RawExtension{Raw: []byte("foo-config")}, Disabled: pointer.Bool(true)},
					}

					expectedDisabledPlugins = []gardencorev1beta1.AdmissionPlugin{
						{Name: "NamespaceLifecycle", Config: &runtime.RawExtension{Raw: []byte("namespace-lifecycle-config")}, Disabled: pointer.Bool(true)},
						{Name: "DefaultStorageClass", Disabled: pointer.Bool(true)},
						{Name: "foo", Config: &runtime.RawExtension{Raw: []byte("foo-config")}, Disabled: pointer.Bool(true)},
					}
				})
			})
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
						Namespace: objectMeta.Namespace,
					},
					Data: map[string]string{"policy": policy},
				}
			})

			DescribeTable("should have the expected audit config",
				func(prepTest func(), expectedConfig *apiserver.AuditConfig, errMatcher gomegatypes.GomegaMatcher) {
					if prepTest != nil {
						prepTest()
					}

					gardenerAPIServer, err := NewGardenerAPIServer(ctx, runtimeClient, namespace, objectMeta, runtimeVersion, sm, apiServerConfig, autoscalingConfig, auditWebhookConfig, topologyAwareRoutingEnabled, clusterIdentity)
					Expect(err).To(errMatcher)
					if gardenerAPIServer != nil {
						Expect(gardenerAPIServer.GetValues().Audit).To(Equal(expectedConfig))
					}
				},

				Entry("GardenerAPIServerConfig is nil",
					nil,
					nil,
					Not(HaveOccurred()),
				),
				Entry("AuditConfig is nil",
					func() {
						apiServerConfig = &operatorv1alpha1.GardenerAPIServerConfig{}
					},
					nil,
					Not(HaveOccurred()),
				),
				Entry("AuditPolicy is nil",
					func() {
						apiServerConfig = &operatorv1alpha1.GardenerAPIServerConfig{
							AuditConfig: &gardencorev1beta1.AuditConfig{},
						}
					},
					nil,
					Not(HaveOccurred()),
				),
				Entry("ConfigMapRef is nil",
					func() {
						apiServerConfig = &operatorv1alpha1.GardenerAPIServerConfig{
							AuditConfig: &gardencorev1beta1.AuditConfig{
								AuditPolicy: &gardencorev1beta1.AuditPolicy{},
							},
						}
					},
					nil,
					Not(HaveOccurred()),
				),
				Entry("ConfigMapRef is provided but configmap is missing",
					func() {
						apiServerConfig = &operatorv1alpha1.GardenerAPIServerConfig{
							AuditConfig: &gardencorev1beta1.AuditConfig{
								AuditPolicy: &gardencorev1beta1.AuditPolicy{
									ConfigMapRef: &corev1.ObjectReference{
										Name: auditPolicyConfigMap.Name,
									},
								},
							},
						}
					},
					nil,
					MatchError(ContainSubstring("not found")),
				),
				Entry("ConfigMapRef is provided but configmap is missing while shoot has a deletion timestamp",
					func() {
						objectMeta.DeletionTimestamp = &metav1.Time{}
						apiServerConfig = &operatorv1alpha1.GardenerAPIServerConfig{
							AuditConfig: &gardencorev1beta1.AuditConfig{
								AuditPolicy: &gardencorev1beta1.AuditPolicy{
									ConfigMapRef: &corev1.ObjectReference{
										Name: auditPolicyConfigMap.Name,
									},
								},
							},
						}
					},
					&apiserver.AuditConfig{},
					Not(HaveOccurred()),
				),
				Entry("ConfigMapRef is provided but configmap does not have correct data field",
					func() {
						auditPolicyConfigMap.Data = nil
						Expect(runtimeClient.Create(ctx, auditPolicyConfigMap)).To(Succeed())

						apiServerConfig = &operatorv1alpha1.GardenerAPIServerConfig{
							AuditConfig: &gardencorev1beta1.AuditConfig{
								AuditPolicy: &gardencorev1beta1.AuditPolicy{
									ConfigMapRef: &corev1.ObjectReference{
										Name: auditPolicyConfigMap.Name,
									},
								},
							},
						}
					},
					nil,
					MatchError(ContainSubstring("missing '.data.policy' in audit policy ConfigMap")),
				),
				Entry("ConfigMapRef is provided and configmap is compliant",
					func() {
						Expect(runtimeClient.Create(ctx, auditPolicyConfigMap)).To(Succeed())

						apiServerConfig = &operatorv1alpha1.GardenerAPIServerConfig{
							AuditConfig: &gardencorev1beta1.AuditConfig{
								AuditPolicy: &gardencorev1beta1.AuditPolicy{
									ConfigMapRef: &corev1.ObjectReference{
										Name: auditPolicyConfigMap.Name,
									},
								},
							},
						}
					},
					&apiserver.AuditConfig{
						Policy: &policy,
					},
					Not(HaveOccurred()),
				),
				Entry("webhook config is provided",
					func() {
						Expect(runtimeClient.Create(ctx, auditPolicyConfigMap)).To(Succeed())

						apiServerConfig = &operatorv1alpha1.GardenerAPIServerConfig{
							AuditConfig: &gardencorev1beta1.AuditConfig{
								AuditPolicy: &gardencorev1beta1.AuditPolicy{
									ConfigMapRef: &corev1.ObjectReference{
										Name: auditPolicyConfigMap.Name,
									},
								},
							},
						}
						auditWebhookConfig = &apiserver.AuditWebhook{Version: pointer.String("audit-version")}
					},
					&apiserver.AuditConfig{
						Policy:  &policy,
						Webhook: &apiserver.AuditWebhook{Version: pointer.String("audit-version")},
					},
					Not(HaveOccurred()),
				),
			)
		})

		Describe("FeatureGates", func() {
			It("should set the field to nil by default", func() {
				gardenerAPIServer, err := NewGardenerAPIServer(ctx, runtimeClient, namespace, objectMeta, runtimeVersion, sm, apiServerConfig, autoscalingConfig, auditWebhookConfig, topologyAwareRoutingEnabled, clusterIdentity)
				Expect(err).NotTo(HaveOccurred())
				Expect(gardenerAPIServer.GetValues().FeatureGates).To(BeNil())
			})

			It("should set the field to the configured values", func() {
				featureGates := map[string]bool{"foo": true, "bar": false}

				apiServerConfig = &operatorv1alpha1.GardenerAPIServerConfig{
					KubernetesConfig: gardencorev1beta1.KubernetesConfig{
						FeatureGates: featureGates,
					},
				}

				gardenerAPIServer, err := NewGardenerAPIServer(ctx, runtimeClient, namespace, objectMeta, runtimeVersion, sm, apiServerConfig, autoscalingConfig, auditWebhookConfig, topologyAwareRoutingEnabled, clusterIdentity)
				Expect(err).NotTo(HaveOccurred())
				Expect(gardenerAPIServer.GetValues().FeatureGates).To(Equal(featureGates))
			})
		})

		Describe("Requests", func() {
			It("should set the field to nil by default", func() {
				gardenerAPIServer, err := NewGardenerAPIServer(ctx, runtimeClient, namespace, objectMeta, runtimeVersion, sm, apiServerConfig, autoscalingConfig, auditWebhookConfig, topologyAwareRoutingEnabled, clusterIdentity)
				Expect(err).NotTo(HaveOccurred())
				Expect(gardenerAPIServer.GetValues().Requests).To(BeNil())
			})

			It("should set the field to the configured values", func() {
				requests := &gardencorev1beta1.APIServerRequests{
					MaxMutatingInflight:    pointer.Int32(1),
					MaxNonMutatingInflight: pointer.Int32(2),
				}
				apiServerConfig = &operatorv1alpha1.GardenerAPIServerConfig{Requests: requests}

				gardenerAPIServer, err := NewGardenerAPIServer(ctx, runtimeClient, namespace, objectMeta, runtimeVersion, sm, apiServerConfig, autoscalingConfig, auditWebhookConfig, topologyAwareRoutingEnabled, clusterIdentity)
				Expect(err).NotTo(HaveOccurred())
				Expect(gardenerAPIServer.GetValues().Requests).To(Equal(requests))
			})
		})

		Describe("WatchCacheSizes", func() {
			It("should set the field to nil by default", func() {
				gardenerAPIServer, err := NewGardenerAPIServer(ctx, runtimeClient, namespace, objectMeta, runtimeVersion, sm, apiServerConfig, autoscalingConfig, auditWebhookConfig, topologyAwareRoutingEnabled, clusterIdentity)
				Expect(err).NotTo(HaveOccurred())
				Expect(gardenerAPIServer.GetValues().WatchCacheSizes).To(BeNil())
			})

			It("should set the field to the configured values", func() {
				watchCacheSizes := &gardencorev1beta1.WatchCacheSizes{
					Default:   pointer.Int32(1),
					Resources: []gardencorev1beta1.ResourceWatchCacheSize{{Resource: "foo"}},
				}
				apiServerConfig = &operatorv1alpha1.GardenerAPIServerConfig{WatchCacheSizes: watchCacheSizes}

				gardenerAPIServer, err := NewGardenerAPIServer(ctx, runtimeClient, namespace, objectMeta, runtimeVersion, sm, apiServerConfig, autoscalingConfig, auditWebhookConfig, topologyAwareRoutingEnabled, clusterIdentity)
				Expect(err).NotTo(HaveOccurred())
				Expect(gardenerAPIServer.GetValues().WatchCacheSizes).To(Equal(watchCacheSizes))
			})
		})
	})

	Describe("#DeployGardenerAPIServer", func() {
		var (
			ctrl                           *gomock.Controller
			gardenerAPIServer              *mockgardenerapiserver.MockInterface
			etcdEncryptionKeyRotationPhase gardencorev1beta1.CredentialsRotationPhase
		)

		BeforeEach(func() {
			ctrl = gomock.NewController(GinkgoT())
			DeferCleanup(func() { ctrl.Finish() })

			gardenerAPIServer = mockgardenerapiserver.NewMockInterface(ctrl)
			etcdEncryptionKeyRotationPhase = ""
		})

		DescribeTable("ETCDEncryptionConfig",
			func(rotationPhase gardencorev1beta1.CredentialsRotationPhase, prepTest func(), expectedETCDEncryptionConfig apiserver.ETCDEncryptionConfig, finalizeTest func()) {
				if len(rotationPhase) > 0 {
					etcdEncryptionKeyRotationPhase = rotationPhase
				}

				if prepTest != nil {
					prepTest()
				}

				gardenerAPIServer.EXPECT().SetETCDEncryptionConfig(expectedETCDEncryptionConfig)
				gardenerAPIServer.EXPECT().Deploy(ctx)

				Expect(DeployGardenerAPIServer(ctx, runtimeClient, namespace, gardenerAPIServer, etcdEncryptionKeyRotationPhase)).To(Succeed())

				if finalizeTest != nil {
					finalizeTest()
				}
			},

			Entry("no rotation",
				gardencorev1beta1.CredentialsRotationPhase(""),
				nil,
				apiserver.ETCDEncryptionConfig{EncryptWithCurrentKey: true, Resources: []string{
					"controllerdeployments.core.gardener.cloud",
					"controllerregistrations.core.gardener.cloud",
					"internalsecrets.core.gardener.cloud",
					"shootstates.core.gardener.cloud",
				}},
				nil,
			),
			Entry("preparing phase, new key already populated",
				gardencorev1beta1.RotationPreparing,
				func() {
					Expect(runtimeClient.Create(ctx, &appsv1.Deployment{
						ObjectMeta: metav1.ObjectMeta{
							Name:        "gardener-apiserver",
							Namespace:   namespace,
							Annotations: map[string]string{"credentials.gardener.cloud/new-encryption-key-populated": "true"},
						},
					})).To(Succeed())
				},
				apiserver.ETCDEncryptionConfig{RotationPhase: gardencorev1beta1.RotationPreparing, EncryptWithCurrentKey: true, Resources: []string{
					"controllerdeployments.core.gardener.cloud",
					"controllerregistrations.core.gardener.cloud",
					"internalsecrets.core.gardener.cloud",
					"shootstates.core.gardener.cloud",
				}},
				nil,
			),
			Entry("preparing phase, new key not yet populated",
				gardencorev1beta1.RotationPreparing,
				func() {
					Expect(runtimeClient.Create(ctx, &appsv1.Deployment{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "gardener-apiserver",
							Namespace: namespace,
						},
					})).To(Succeed())

					gardenerAPIServer.EXPECT().Wait(ctx)

					gardenerAPIServer.EXPECT().SetETCDEncryptionConfig(apiserver.ETCDEncryptionConfig{
						RotationPhase:         gardencorev1beta1.RotationPreparing,
						EncryptWithCurrentKey: true,
						Resources: []string{
							"controllerdeployments.core.gardener.cloud",
							"controllerregistrations.core.gardener.cloud",
							"internalsecrets.core.gardener.cloud",
							"shootstates.core.gardener.cloud",
						},
					})
					gardenerAPIServer.EXPECT().Deploy(ctx)
				},
				apiserver.ETCDEncryptionConfig{RotationPhase: gardencorev1beta1.RotationPreparing, EncryptWithCurrentKey: false, Resources: []string{
					"controllerdeployments.core.gardener.cloud",
					"controllerregistrations.core.gardener.cloud",
					"internalsecrets.core.gardener.cloud",
					"shootstates.core.gardener.cloud",
				}},
				func() {
					deployment := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "gardener-apiserver", Namespace: namespace}}
					Expect(runtimeClient.Get(ctx, client.ObjectKeyFromObject(deployment), deployment)).To(Succeed())
					Expect(deployment.Annotations).To(HaveKeyWithValue("credentials.gardener.cloud/new-encryption-key-populated", "true"))
				},
			),
			Entry("prepared phase",
				gardencorev1beta1.RotationPrepared,
				nil,
				apiserver.ETCDEncryptionConfig{RotationPhase: gardencorev1beta1.RotationPrepared, EncryptWithCurrentKey: true, Resources: []string{
					"controllerdeployments.core.gardener.cloud",
					"controllerregistrations.core.gardener.cloud",
					"internalsecrets.core.gardener.cloud",
					"shootstates.core.gardener.cloud",
				}},
				nil,
			),
			Entry("completing phase",
				gardencorev1beta1.RotationCompleting,
				func() {
					Expect(runtimeClient.Create(ctx, &appsv1.Deployment{
						ObjectMeta: metav1.ObjectMeta{
							Name:        "gardener-apiserver",
							Namespace:   namespace,
							Annotations: map[string]string{"credentials.gardener.cloud/new-encryption-key-populated": "true"},
						},
					})).To(Succeed())
				},
				apiserver.ETCDEncryptionConfig{RotationPhase: gardencorev1beta1.RotationCompleting, EncryptWithCurrentKey: true, Resources: []string{
					"controllerdeployments.core.gardener.cloud",
					"controllerregistrations.core.gardener.cloud",
					"internalsecrets.core.gardener.cloud",
					"shootstates.core.gardener.cloud",
				}},
				func() {
					deployment := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "gardener-apiserver", Namespace: namespace}}
					Expect(runtimeClient.Get(ctx, client.ObjectKeyFromObject(deployment), deployment)).To(Succeed())
					Expect(deployment.Annotations).NotTo(HaveKey("credentials.gardener.cloud/new-encryption-key-populated"))
				},
			),
			Entry("completed phase",
				gardencorev1beta1.RotationCompleted,
				nil,
				apiserver.ETCDEncryptionConfig{RotationPhase: gardencorev1beta1.RotationCompleted, EncryptWithCurrentKey: true, Resources: []string{
					"controllerdeployments.core.gardener.cloud",
					"controllerregistrations.core.gardener.cloud",
					"internalsecrets.core.gardener.cloud",
					"shootstates.core.gardener.cloud",
				}},
				nil,
			),
		)
	})
})
