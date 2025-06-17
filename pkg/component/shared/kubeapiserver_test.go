// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shared_test

import (
	"context"
	"net"
	"time"

	"github.com/Masterminds/semver/v3"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	gomegatypes "github.com/onsi/gomega/types"
	"go.uber.org/mock/gomock"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	apiserverv1beta1 "k8s.io/apiserver/pkg/apis/apiserver/v1beta1"
	kubernetesscheme "k8s.io/client-go/kubernetes/scheme"
	admissionapiv1 "k8s.io/pod-security-admission/admission/api/v1"
	admissionapiv1alpha1 "k8s.io/pod-security-admission/admission/api/v1alpha1"
	admissionapiv1beta1 "k8s.io/pod-security-admission/admission/api/v1beta1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/fake"
	"github.com/gardener/gardener/pkg/component/apiserver"
	kubeapiserver "github.com/gardener/gardener/pkg/component/kubernetes/apiserver"
	mockkubeapiserver "github.com/gardener/gardener/pkg/component/kubernetes/apiserver/mock"
	. "github.com/gardener/gardener/pkg/component/shared"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	fakesecretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager/fake"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("KubeAPIServer", func() {
	var (
		ctx = context.TODO()

		runtimeClient        client.Client
		namespace            string
		apiServerConfig      *gardencorev1beta1.KubeAPIServerConfig
		serviceAccountConfig kubeapiserver.ServiceAccountConfig
	)

	BeforeEach(func() {
		namespace = "foo"
		apiServerConfig = nil

		runtimeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
	})

	Describe("#NewKubeAPIServer", func() {
		var (
			name                         string
			objectMeta                   metav1.ObjectMeta
			secret                       *corev1.Secret
			runtimeVersion               *semver.Version
			targetVersion                *semver.Version
			namePrefix                   string
			autoscalingConfig            kubeapiserver.AutoscalingConfig
			vpnConfig                    kubeapiserver.VPNConfig
			priorityClassName            string
			isWorkerless                 bool
			runsAsStaticPod              bool
			auditWebhookConfig           *apiserver.AuditWebhook
			authenticationWebhookConfig  *kubeapiserver.AuthenticationWebhook
			authorizationWebhookConfigs  []kubeapiserver.AuthorizationWebhook
			resourcesToStoreInETCDEvents []schema.GroupResource

			runtimeClientSet     kubernetes.Interface
			resourceConfigClient client.Client
			sm                   secretsmanager.Interface
		)

		BeforeEach(func() {
			name = "bar"
			objectMeta = metav1.ObjectMeta{Namespace: namespace, Name: name}
			runtimeVersion = semver.MustParse("1.31.0")
			targetVersion = semver.MustParse("1.31.0")
			namePrefix = ""
			autoscalingConfig = kubeapiserver.AutoscalingConfig{}
			vpnConfig = kubeapiserver.VPNConfig{}
			priorityClassName = "priority-class"
			isWorkerless = false
			runsAsStaticPod = false
			auditWebhookConfig = nil
			authenticationWebhookConfig = &kubeapiserver.AuthenticationWebhook{Version: ptr.To("authn-version")}
			authorizationWebhookConfigs = []kubeapiserver.AuthorizationWebhook{{Name: "custom", Kubeconfig: []byte("bar"), WebhookConfiguration: apiserverv1beta1.WebhookConfiguration{FailurePolicy: "Fail"}}}
			resourcesToStoreInETCDEvents = []schema.GroupResource{{Resource: "foo", Group: "bar"}}

			secret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "secret-1",
					Namespace: namespace,
				},
				Data: map[string][]byte{"kubeconfig": []byte("kubeconfig-data")},
			}

			runtimeClientSet = fake.NewClientSetBuilder().WithClient(runtimeClient).WithVersion(runtimeVersion.String()).Build()
			resourceConfigClient = fakeclient.NewClientBuilder().WithScheme(kubernetesscheme.Scheme).Build()
			sm = fakesecretsmanager.New(runtimeClient, namespace)
		})

		Describe("AnonymousAuthenticationEnabled", func() {
			It("should not set the field by default", func() {
				kubeAPIServer, err := NewKubeAPIServer(ctx, runtimeClientSet, resourceConfigClient, namespace, objectMeta, runtimeVersion, targetVersion, sm, namePrefix, apiServerConfig, autoscalingConfig, vpnConfig, priorityClassName, isWorkerless, runsAsStaticPod, auditWebhookConfig, authenticationWebhookConfig, authorizationWebhookConfigs, resourcesToStoreInETCDEvents)
				Expect(err).NotTo(HaveOccurred())
				Expect(kubeAPIServer.GetValues().AnonymousAuthenticationEnabled).To(BeNil())
			})

			It("should set the field to true if explicitly enabled", func() {
				apiServerConfig = &gardencorev1beta1.KubeAPIServerConfig{EnableAnonymousAuthentication: ptr.To(true)}

				kubeAPIServer, err := NewKubeAPIServer(ctx, runtimeClientSet, resourceConfigClient, namespace, objectMeta, runtimeVersion, targetVersion, sm, namePrefix, apiServerConfig, autoscalingConfig, vpnConfig, priorityClassName, isWorkerless, runsAsStaticPod, auditWebhookConfig, authenticationWebhookConfig, authorizationWebhookConfigs, resourcesToStoreInETCDEvents)
				Expect(err).NotTo(HaveOccurred())
				Expect(kubeAPIServer.GetValues().AnonymousAuthenticationEnabled).To(PointTo(BeTrue()))
			})
		})

		Describe("APIAudiences", func() {
			It("should set the field to 'kubernetes' and 'gardener' by default", func() {
				kubeAPIServer, err := NewKubeAPIServer(ctx, runtimeClientSet, resourceConfigClient, namespace, objectMeta, runtimeVersion, targetVersion, sm, namePrefix, apiServerConfig, autoscalingConfig, vpnConfig, priorityClassName, isWorkerless, runsAsStaticPod, auditWebhookConfig, authenticationWebhookConfig, authorizationWebhookConfigs, resourcesToStoreInETCDEvents)
				Expect(err).NotTo(HaveOccurred())
				Expect(kubeAPIServer.GetValues().APIAudiences).To(ConsistOf("kubernetes", "gardener"))
			})

			It("should set the field to the configured values", func() {
				apiAudiences := []string{"foo", "bar"}
				apiServerConfig = &gardencorev1beta1.KubeAPIServerConfig{APIAudiences: apiAudiences}

				kubeAPIServer, err := NewKubeAPIServer(ctx, runtimeClientSet, resourceConfigClient, namespace, objectMeta, runtimeVersion, targetVersion, sm, namePrefix, apiServerConfig, autoscalingConfig, vpnConfig, priorityClassName, isWorkerless, runsAsStaticPod, auditWebhookConfig, authenticationWebhookConfig, authorizationWebhookConfigs, resourcesToStoreInETCDEvents)
				Expect(err).NotTo(HaveOccurred())
				Expect(kubeAPIServer.GetValues().APIAudiences).To(Equal(append(apiAudiences, "gardener")))
			})

			It("should not add gardener audience if already present", func() {
				apiAudiences := []string{"foo", "bar", "gardener"}
				apiServerConfig = &gardencorev1beta1.KubeAPIServerConfig{APIAudiences: apiAudiences}

				kubeAPIServer, err := NewKubeAPIServer(ctx, runtimeClientSet, resourceConfigClient, namespace, objectMeta, runtimeVersion, targetVersion, sm, namePrefix, apiServerConfig, autoscalingConfig, vpnConfig, priorityClassName, isWorkerless, runsAsStaticPod, auditWebhookConfig, authenticationWebhookConfig, authorizationWebhookConfigs, resourcesToStoreInETCDEvents)
				Expect(err).NotTo(HaveOccurred())
				Expect(kubeAPIServer.GetValues().APIAudiences).To(Equal(apiAudiences))
			})
		})

		Describe("AdmissionPlugins", func() {
			BeforeEach(func() {
				Expect(resourceConfigClient.Create(ctx, secret)).To(Succeed())
				apiServerConfig = &gardencorev1beta1.KubeAPIServerConfig{AdmissionPlugins: []gardencorev1beta1.AdmissionPlugin{}}
			})

			DescribeTable("should have the expected admission plugins config",
				func(configuredPlugins []gardencorev1beta1.AdmissionPlugin, expectedPlugins []apiserver.AdmissionPluginConfig, isWorkerless bool) {
					apiServerConfig.AdmissionPlugins = configuredPlugins

					kubeAPIServer, err := NewKubeAPIServer(ctx, runtimeClientSet, resourceConfigClient, namespace, objectMeta, runtimeVersion, targetVersion, sm, namePrefix, apiServerConfig, autoscalingConfig, vpnConfig, priorityClassName, isWorkerless, runsAsStaticPod, auditWebhookConfig, authenticationWebhookConfig, authorizationWebhookConfigs, resourcesToStoreInETCDEvents)
					Expect(err).NotTo(HaveOccurred())
					Expect(kubeAPIServer.GetValues().EnabledAdmissionPlugins).To(Equal(expectedPlugins))
				},

				Entry("only default plugins",
					nil,
					[]apiserver.AdmissionPluginConfig{
						{AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{Name: "NodeRestriction"}},
					},
					false,
				),
				Entry("default plugins with overrides",
					[]gardencorev1beta1.AdmissionPlugin{
						{Name: "NodeRestriction", Config: &runtime.RawExtension{Raw: []byte("node-restriction-config")}, KubeconfigSecretName: ptr.To("secret-1")},
					},
					[]apiserver.AdmissionPluginConfig{
						{AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{Name: "NodeRestriction", Config: &runtime.RawExtension{Raw: []byte("node-restriction-config")}, KubeconfigSecretName: ptr.To("secret-1")}, Kubeconfig: []byte("kubeconfig-data")},
					},
					false,
				),
				Entry("default plugins with overrides and other plugins",
					[]gardencorev1beta1.AdmissionPlugin{
						{Name: "NodeRestriction", Config: &runtime.RawExtension{Raw: []byte("node-restriction-config")}},
						{Name: "Foo"},
						{Name: "Bar"},
						{Name: "Baz", Config: &runtime.RawExtension{Raw: []byte("baz-config")}},
					},
					[]apiserver.AdmissionPluginConfig{
						{AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{Name: "NodeRestriction", Config: &runtime.RawExtension{Raw: []byte("node-restriction-config")}}},
						{AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{Name: "Foo"}},
						{AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{Name: "Bar"}},
						{AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{Name: "Baz", Config: &runtime.RawExtension{Raw: []byte("baz-config")}}},
					},
					false,
				),
				Entry("default plugins with overrides and skipping configured plugins if disabled",
					[]gardencorev1beta1.AdmissionPlugin{
						{Name: "NodeRestriction", Config: &runtime.RawExtension{Raw: []byte("node-restriction-config")}},
						{Name: "Foo"},
						{Name: "Bar", Disabled: ptr.To(true)},
						{Name: "Baz", Config: &runtime.RawExtension{Raw: []byte("baz-config")}, Disabled: ptr.To(true)},
					},
					[]apiserver.AdmissionPluginConfig{
						{AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{Name: "NodeRestriction", Config: &runtime.RawExtension{Raw: []byte("node-restriction-config")}}},
						{AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{Name: "Foo"}},
					},
					false,
				),
				Entry("skipping default plugins if disabled",
					[]gardencorev1beta1.AdmissionPlugin{
						{Name: "NodeRestriction", Disabled: ptr.To(true)},
						{Name: "Foo"},
						{Name: "Bar"},
						{Name: "Baz", Config: &runtime.RawExtension{Raw: []byte("baz-config")}},
					},
					[]apiserver.AdmissionPluginConfig{
						{AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{Name: "Foo"}},
						{AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{Name: "Bar"}},
						{AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{Name: "Baz", Config: &runtime.RawExtension{Raw: []byte("baz-config")}}},
					},
					false,
				),
			)

			Context("should have the expected disabled admission plugins", func() {
				var expectedDisabledPlugins []gardencorev1beta1.AdmissionPlugin

				AfterEach(func() {
					kubeAPIServer, err := NewKubeAPIServer(ctx, runtimeClientSet, resourceConfigClient, namespace, objectMeta, runtimeVersion, targetVersion, sm, namePrefix, apiServerConfig, autoscalingConfig, vpnConfig, priorityClassName, isWorkerless, runsAsStaticPod, auditWebhookConfig, authenticationWebhookConfig, authorizationWebhookConfigs, resourcesToStoreInETCDEvents)
					Expect(err).NotTo(HaveOccurred())
					Expect(kubeAPIServer.GetValues().DisabledAdmissionPlugins).To(Equal(expectedDisabledPlugins))
				})

				It("should return the correct list of disabled admission plugins", func() {
					apiServerConfig.AdmissionPlugins = []gardencorev1beta1.AdmissionPlugin{
						{Name: "Priority"},
						{Name: "NamespaceLifecycle", Config: &runtime.RawExtension{Raw: []byte("namespace-lifecycle-config")}},
						{Name: "LimitRanger"},
						{Name: "ServiceAccount", Disabled: ptr.To(true)},
						{Name: "NodeRestriction"},
						{Name: "DefaultStorageClass"},
						{Name: "DefaultTolerationSeconds", Disabled: ptr.To(true)},
						{Name: "ResourceQuota"},
					}

					expectedDisabledPlugins = []gardencorev1beta1.AdmissionPlugin{
						{Name: "ServiceAccount", Disabled: ptr.To(true)},
						{Name: "DefaultTolerationSeconds", Disabled: ptr.To(true)},
					}
				})

				It("should return the correct list of disabled admission plugins", func() {
					apiServerConfig.AdmissionPlugins = []gardencorev1beta1.AdmissionPlugin{
						{Name: "Priority"},
						{Name: "NamespaceLifecycle", Config: &runtime.RawExtension{Raw: []byte("namespace-lifecycle-config")}, Disabled: ptr.To(true)},
						{Name: "LimitRanger"},
						{Name: "ServiceAccount"},
						{Name: "NodeRestriction"},
						{Name: "DefaultStorageClass", Disabled: ptr.To(true)},
						{Name: "DefaultTolerationSeconds"},
						{Name: "ResourceQuota"},
						{Name: "foo", Config: &runtime.RawExtension{Raw: []byte("foo-config")}, Disabled: ptr.To(true)},
					}

					expectedDisabledPlugins = []gardencorev1beta1.AdmissionPlugin{
						{Name: "NamespaceLifecycle", Config: &runtime.RawExtension{Raw: []byte("namespace-lifecycle-config")}, Disabled: ptr.To(true)},
						{Name: "DefaultStorageClass", Disabled: ptr.To(true)},
						{Name: "foo", Config: &runtime.RawExtension{Raw: []byte("foo-config")}, Disabled: ptr.To(true)},
					}
				})
			})

			Describe("PodSecurity Admission Plugin", func() {
				var (
					configData    *runtime.RawExtension
					err           error
					kubeAPIServer kubeapiserver.Interface

					runtimeScheme = runtime.NewScheme()
					codec         runtime.Codec
				)

				JustBeforeEach(func() {
					utilruntime.Must(admissionapiv1alpha1.AddToScheme(runtimeScheme))
					utilruntime.Must(admissionapiv1beta1.AddToScheme(runtimeScheme))
					utilruntime.Must(admissionapiv1.AddToScheme(runtimeScheme))

					var (
						ser = json.NewSerializerWithOptions(json.DefaultMetaFactory, runtimeScheme, runtimeScheme, json.SerializerOptions{
							Yaml:   true,
							Pretty: false,
							Strict: false,
						})
						versions = schema.GroupVersions([]schema.GroupVersion{
							admissionapiv1alpha1.SchemeGroupVersion,
							admissionapiv1beta1.SchemeGroupVersion,
							admissionapiv1.SchemeGroupVersion,
						})
					)

					codec = serializer.NewCodecFactory(runtimeScheme).CodecForVersions(ser, ser, versions, versions)

					configData = nil
					kubeAPIServer, err = NewKubeAPIServer(ctx, runtimeClientSet, resourceConfigClient, namespace, objectMeta, runtimeVersion, targetVersion, sm, namePrefix, apiServerConfig, autoscalingConfig, vpnConfig, priorityClassName, isWorkerless, runsAsStaticPod, auditWebhookConfig, authenticationWebhookConfig, authorizationWebhookConfigs, resourcesToStoreInETCDEvents)
				})

				Context("When the config is nil", func() {
					It("should do nothing", func() {
						Expect(err).NotTo(HaveOccurred())

						admissionPlugins := kubeAPIServer.GetValues().EnabledAdmissionPlugins
						for _, plugin := range admissionPlugins {
							if plugin.Name == "PodSecurity" {
								configData = plugin.Config
							}
						}

						Expect(configData).To(BeNil())
					})
				})

				Context("When the config is not nil", func() {
					var (
						err    error
						ok     bool
						config runtime.Object
					)

					JustBeforeEach(func() {
						admissionPlugins := kubeAPIServer.GetValues().EnabledAdmissionPlugins
						for _, plugin := range admissionPlugins {
							if plugin.Name == "PodSecurity" {
								configData = plugin.Config
							}
						}

						Expect(configData).NotTo(BeNil())

						config, err = runtime.Decode(codec, configData.Raw)
						Expect(err).NotTo(HaveOccurred())
					})

					Context("PodSecurity admission config is v1", func() {
						BeforeEach(func() {
							apiServerConfig.AdmissionPlugins = []gardencorev1beta1.AdmissionPlugin{
								{
									Name: "PodSecurity",
									Config: &runtime.RawExtension{Raw: []byte(`apiVersion: pod-security.admission.config.k8s.io/v1
kind: PodSecurityConfiguration
defaults:
  enforce: "privileged"
  enforce-version: "latest"
  audit-version: "latest"
  warn: "baseline"
  warn-version: "v1.31"
exemptions:
  usernames: ["admin"]
  runtimeClasses: ["random"]
  namespaces: ["random"]
`),
									},
								},
							}
						})

						It("should add kube-system to exempted namespaces and not touch other fields", func() {
							var admConfig *admissionapiv1.PodSecurityConfiguration

							admConfig, ok = config.(*admissionapiv1.PodSecurityConfiguration)
							Expect(ok).To(BeTrue())

							Expect(admConfig.Defaults).To(MatchFields(IgnoreExtras, Fields{
								"Enforce": Equal("privileged"),
								// This field is defaulted by kubernetes
								"Audit":       Equal("privileged"),
								"Warn":        Equal("baseline"),
								"WarnVersion": Equal("v1.31"),
							}))
							Expect(admConfig.Exemptions.Usernames).To(ContainElement("admin"))
							Expect(admConfig.Exemptions.Namespaces).To(ContainElements("kube-system", "random"))
						})
					})

					Context("PodSecurity admission config is v1beta1", func() {
						BeforeEach(func() {
							apiServerConfig.AdmissionPlugins = []gardencorev1beta1.AdmissionPlugin{
								{
									Name: "PodSecurity",
									Config: &runtime.RawExtension{Raw: []byte(`apiVersion: pod-security.admission.config.k8s.io/v1beta1
kind: PodSecurityConfiguration
defaults:
  enforce: "privileged"
  enforce-version: "latest"
  audit-version: "latest"
  warn: "baseline"
  warn-version: "v1.31"
exemptions:
  usernames: ["admin"]
  runtimeClasses: ["random"]
  namespaces: ["random"]
`),
									},
								},
							}
						})

						It("should add kube-system to exempted namespaces and not touch other fields", func() {
							var admConfig *admissionapiv1beta1.PodSecurityConfiguration

							admConfig, ok = config.(*admissionapiv1beta1.PodSecurityConfiguration)
							Expect(ok).To(BeTrue())

							Expect(admConfig.Defaults).To(MatchFields(IgnoreExtras, Fields{
								"Enforce": Equal("privileged"),
								// This field is defaulted by kubernetes
								"Audit":       Equal("privileged"),
								"Warn":        Equal("baseline"),
								"WarnVersion": Equal("v1.31"),
							}))
							Expect(admConfig.Exemptions.Usernames).To(ContainElement("admin"))
							Expect(admConfig.Exemptions.Namespaces).To(ContainElements("kube-system", "random"))
						})
					})

					Context("PodSecurity admission config is v1alpha1", func() {
						BeforeEach(func() {
							apiServerConfig.AdmissionPlugins = []gardencorev1beta1.AdmissionPlugin{
								{
									Name: "PodSecurity",
									Config: &runtime.RawExtension{Raw: []byte(`apiVersion: pod-security.admission.config.k8s.io/v1alpha1
kind: PodSecurityConfiguration
defaults:
  enforce: "privileged"
  enforce-version: "latest"
  audit-version: "latest"
  warn: "baseline"
  warn-version: "v1.31"
exemptions:
  usernames: ["admin"]
  runtimeClasses: ["random"]
  namespaces: ["random"]
`),
									},
								},
							}
						})

						It("should add kube-system to exempted namespaces and not touch other fields", func() {
							var admConfig *admissionapiv1alpha1.PodSecurityConfiguration

							admConfig, ok = config.(*admissionapiv1alpha1.PodSecurityConfiguration)
							Expect(ok).To(BeTrue())

							Expect(admConfig.Defaults).To(MatchFields(IgnoreExtras, Fields{
								"Enforce": Equal("privileged"),
								// This field is defaulted by kubernetes
								"Audit":       Equal("privileged"),
								"Warn":        Equal("baseline"),
								"WarnVersion": Equal("v1.31"),
							}))
							Expect(admConfig.Exemptions.Usernames).To(ContainElement("admin"))
							Expect(admConfig.Exemptions.Namespaces).To(ContainElements("kube-system", "random"))
						})
					})
				})

				Context("PodSecurity admission config is neither v1alpha1 nor v1beta1 nor v1", func() {
					BeforeEach(func() {
						apiServerConfig.AdmissionPlugins = []gardencorev1beta1.AdmissionPlugin{
							{
								Name: "PodSecurity",
								Config: &runtime.RawExtension{Raw: []byte(`apiVersion: pod-security.admission.config.k8s.io/foo
kind: PodSecurityConfiguration-bar
defaults:
  enforce: "privileged"
  enforce-version: "latest"
exemptions:
  usernames: ["admin"]
`),
								},
							},
						}
					})

					It("should throw an error", func() {
						Expect(kubeAPIServer).To(BeNil())

						Expect(err).To(BeNotRegisteredError())
					})
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

					kubeAPIServer, err := NewKubeAPIServer(ctx, runtimeClientSet, resourceConfigClient, namespace, objectMeta, runtimeVersion, targetVersion, sm, namePrefix, apiServerConfig, autoscalingConfig, vpnConfig, priorityClassName, isWorkerless, runsAsStaticPod, auditWebhookConfig, authenticationWebhookConfig, authorizationWebhookConfigs, resourcesToStoreInETCDEvents)
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
						apiServerConfig = &gardencorev1beta1.KubeAPIServerConfig{}
					},
					nil,
					Not(HaveOccurred()),
				),
				Entry("AuditPolicy is nil",
					func() {
						apiServerConfig = &gardencorev1beta1.KubeAPIServerConfig{
							AuditConfig: &gardencorev1beta1.AuditConfig{},
						}
					},
					nil,
					Not(HaveOccurred()),
				),
				Entry("ConfigMapRef is nil",
					func() {
						apiServerConfig = &gardencorev1beta1.KubeAPIServerConfig{
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
						apiServerConfig = &gardencorev1beta1.KubeAPIServerConfig{
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
						apiServerConfig = &gardencorev1beta1.KubeAPIServerConfig{
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
						Expect(resourceConfigClient.Create(ctx, auditPolicyConfigMap)).To(Succeed())

						apiServerConfig = &gardencorev1beta1.KubeAPIServerConfig{
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
						Expect(resourceConfigClient.Create(ctx, auditPolicyConfigMap)).To(Succeed())

						apiServerConfig = &gardencorev1beta1.KubeAPIServerConfig{
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
						Expect(resourceConfigClient.Create(ctx, auditPolicyConfigMap)).To(Succeed())

						apiServerConfig = &gardencorev1beta1.KubeAPIServerConfig{
							AuditConfig: &gardencorev1beta1.AuditConfig{
								AuditPolicy: &gardencorev1beta1.AuditPolicy{
									ConfigMapRef: &corev1.ObjectReference{
										Name: auditPolicyConfigMap.Name,
									},
								},
							},
						}
						auditWebhookConfig = &apiserver.AuditWebhook{Version: ptr.To("audit-version")}
					},
					&apiserver.AuditConfig{
						Policy:  &policy,
						Webhook: &apiserver.AuditWebhook{Version: ptr.To("audit-version")},
					},
					Not(HaveOccurred()),
				),
			)
		})

		Describe("AuthenticationConfiguration", func() {
			var (
				config                        = "some-config"
				authenticationConfigurationCm *corev1.ConfigMap
			)

			BeforeEach(func() {
				authenticationConfigurationCm = &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "auth-config",
						Namespace: objectMeta.Namespace,
					},
					Data: map[string]string{"config.yaml": config},
				}
			})

			DescribeTable("should have the expected authentication configuration",
				func(prepTest func(), expectedConfig *string, errMatcher gomegatypes.GomegaMatcher) {
					if prepTest != nil {
						prepTest()
					}

					kubeAPIServer, err := NewKubeAPIServer(ctx, runtimeClientSet, resourceConfigClient, namespace, objectMeta, runtimeVersion, targetVersion, sm, namePrefix, apiServerConfig, autoscalingConfig, vpnConfig, priorityClassName, isWorkerless, runsAsStaticPod, auditWebhookConfig, authenticationWebhookConfig, authorizationWebhookConfigs, resourcesToStoreInETCDEvents)
					Expect(err).To(errMatcher)
					if kubeAPIServer != nil {
						Expect(kubeAPIServer.GetValues().AuthenticationConfiguration).To(Equal(expectedConfig))
					}
				},

				Entry("KubeAPIServerConfig is nil",
					nil,
					nil,
					Not(HaveOccurred()),
				),
				Entry("StructuredAuthentication is nil",
					func() {
						apiServerConfig = &gardencorev1beta1.KubeAPIServerConfig{}
					},
					nil,
					Not(HaveOccurred()),
				),
				Entry("ConfigMapName is empty",
					func() {
						apiServerConfig = &gardencorev1beta1.KubeAPIServerConfig{
							StructuredAuthentication: &gardencorev1beta1.StructuredAuthentication{
								ConfigMapName: "",
							},
						}
					},
					nil,
					Not(HaveOccurred()),
				),
				Entry("ConfigMapName is provided but configmap is missing",
					func() {
						apiServerConfig = &gardencorev1beta1.KubeAPIServerConfig{
							StructuredAuthentication: &gardencorev1beta1.StructuredAuthentication{
								ConfigMapName: authenticationConfigurationCm.Name,
							},
						}
					},
					nil,
					MatchError(ContainSubstring("not found")),
				),
				Entry("ConfigMapName is provided but configmap is missing while shoot has a deletion timestamp",
					func() {
						objectMeta.DeletionTimestamp = &metav1.Time{}
						apiServerConfig = &gardencorev1beta1.KubeAPIServerConfig{
							StructuredAuthentication: &gardencorev1beta1.StructuredAuthentication{
								ConfigMapName: authenticationConfigurationCm.Name,
							},
						}
					},
					nil,
					Not(HaveOccurred()),
				),
				Entry("ConfigMapName is provided but configmap does not have correct data field",
					func() {
						authenticationConfigurationCm.Data = nil
						Expect(resourceConfigClient.Create(ctx, authenticationConfigurationCm)).To(Succeed())

						apiServerConfig = &gardencorev1beta1.KubeAPIServerConfig{
							StructuredAuthentication: &gardencorev1beta1.StructuredAuthentication{
								ConfigMapName: authenticationConfigurationCm.Name,
							},
						}
					},
					nil,
					MatchError(ContainSubstring("missing '.data[config.yaml]' in authentication configuration ConfigMap")),
				),
				Entry("ConfigMapName is provided and configmap is compliant",
					func() {
						Expect(resourceConfigClient.Create(ctx, authenticationConfigurationCm)).To(Succeed())

						apiServerConfig = &gardencorev1beta1.KubeAPIServerConfig{
							StructuredAuthentication: &gardencorev1beta1.StructuredAuthentication{
								ConfigMapName: authenticationConfigurationCm.Name,
							},
						}
					},
					ptr.To(config),
					Not(HaveOccurred()),
				),
			)
		})

		Describe("AuthorizationWebhooks", func() {
			var (
				config = `---
apiVersion: apiserver.config.k8s.io/v1beta1
kind: AuthorizationConfiguration
authorizers:
- type: Webhook
  name: webhook
  webhook:
    timeout: 3s
    subjectAccessReviewVersion: v1
    matchConditionSubjectAccessReviewVersion: v1
    failurePolicy: Deny
    matchConditions:
    - expression: request.resourceAttributes.namespace == 'kube-system'
`
				authorizationConfigurationCm *corev1.ConfigMap
			)

			BeforeEach(func() {
				authorizationConfigurationCm = &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "auth-config",
						Namespace: objectMeta.Namespace,
					},
					Data: map[string]string{"config.yaml": config},
				}
			})

			DescribeTable("should have the expected authorization configuration",
				func(prepTest func(), expectedWebhooks []kubeapiserver.AuthorizationWebhook, errMatcher gomegatypes.GomegaMatcher) {
					if prepTest != nil {
						prepTest()
					}

					kubeAPIServer, err := NewKubeAPIServer(ctx, runtimeClientSet, resourceConfigClient, namespace, objectMeta, runtimeVersion, targetVersion, sm, namePrefix, apiServerConfig, autoscalingConfig, vpnConfig, priorityClassName, isWorkerless, runsAsStaticPod, auditWebhookConfig, authenticationWebhookConfig, authorizationWebhookConfigs, resourcesToStoreInETCDEvents)
					Expect(err).To(errMatcher)
					if kubeAPIServer != nil {
						Expect(kubeAPIServer.GetValues().AuthorizationWebhooks).To(Equal(expectedWebhooks))
					}
				},

				Entry("KubeAPIServerConfig is nil",
					nil,
					[]kubeapiserver.AuthorizationWebhook{{
						Name:                 "custom",
						Kubeconfig:           []byte("bar"),
						WebhookConfiguration: apiserverv1beta1.WebhookConfiguration{FailurePolicy: "Fail"}},
					},
					Not(HaveOccurred()),
				),
				Entry("StructuredAuthorization is nil",
					func() {
						apiServerConfig = &gardencorev1beta1.KubeAPIServerConfig{}
					},
					[]kubeapiserver.AuthorizationWebhook{{
						Name:                 "custom",
						Kubeconfig:           []byte("bar"),
						WebhookConfiguration: apiserverv1beta1.WebhookConfiguration{FailurePolicy: "Fail"}},
					},
					Not(HaveOccurred()),
				),
				Entry("ConfigMapName is empty",
					func() {
						apiServerConfig = &gardencorev1beta1.KubeAPIServerConfig{
							StructuredAuthorization: &gardencorev1beta1.StructuredAuthorization{
								ConfigMapName: "",
							},
						}
					},
					[]kubeapiserver.AuthorizationWebhook{{
						Name:                 "custom",
						Kubeconfig:           []byte("bar"),
						WebhookConfiguration: apiserverv1beta1.WebhookConfiguration{FailurePolicy: "Fail"}},
					},
					Not(HaveOccurred()),
				),
				Entry("ConfigMapName is provided but ConfigMap is missing",
					func() {
						apiServerConfig = &gardencorev1beta1.KubeAPIServerConfig{
							StructuredAuthorization: &gardencorev1beta1.StructuredAuthorization{
								ConfigMapName: authorizationConfigurationCm.Name,
							},
						}
					},
					nil,
					MatchError(ContainSubstring("not found")),
				),
				Entry("ConfigMapName is provided but ConfigMap is missing while shoot has a deletion timestamp",
					func() {
						objectMeta.DeletionTimestamp = &metav1.Time{}
						apiServerConfig = &gardencorev1beta1.KubeAPIServerConfig{
							StructuredAuthorization: &gardencorev1beta1.StructuredAuthorization{
								ConfigMapName: authorizationConfigurationCm.Name,
							},
						}
					},
					[]kubeapiserver.AuthorizationWebhook{{
						Name:                 "custom",
						Kubeconfig:           []byte("bar"),
						WebhookConfiguration: apiserverv1beta1.WebhookConfiguration{FailurePolicy: "Fail"}},
					},
					Not(HaveOccurred()),
				),
				Entry("ConfigMapName is provided but ConfigMap does not have correct data field",
					func() {
						authorizationConfigurationCm.Data = nil
						Expect(resourceConfigClient.Create(ctx, authorizationConfigurationCm)).To(Succeed())

						apiServerConfig = &gardencorev1beta1.KubeAPIServerConfig{
							StructuredAuthorization: &gardencorev1beta1.StructuredAuthorization{
								ConfigMapName: authorizationConfigurationCm.Name,
							},
						}
					},
					nil,
					MatchError(ContainSubstring("missing '.data[config.yaml]' in authorization configuration ConfigMap")),
				),
				Entry("ConfigMapName is provided and ConfigMap is compliant but kubeconfig secret ref is missing",
					func() {
						Expect(resourceConfigClient.Create(ctx, authorizationConfigurationCm)).To(Succeed())

						apiServerConfig = &gardencorev1beta1.KubeAPIServerConfig{
							StructuredAuthorization: &gardencorev1beta1.StructuredAuthorization{
								ConfigMapName: authorizationConfigurationCm.Name,
							},
						}
					},
					nil,
					MatchError(ContainSubstring("missing kubeconfig secret reference for authorizer webhook")),
				),
				Entry("ConfigMapName is provided and ConfigMap is compliant but referenced kubeconfig secret is missing",
					func() {
						Expect(resourceConfigClient.Create(ctx, authorizationConfigurationCm)).To(Succeed())

						apiServerConfig = &gardencorev1beta1.KubeAPIServerConfig{
							StructuredAuthorization: &gardencorev1beta1.StructuredAuthorization{
								ConfigMapName: authorizationConfigurationCm.Name,
								Kubeconfigs:   []gardencorev1beta1.AuthorizerKubeconfigReference{{AuthorizerName: "webhook", SecretName: "authz-kubeconfig"}},
							},
						}
					},
					nil,
					MatchError(ContainSubstring("retrieving kubeconfig secret foo/authz-kubeconfig failed: secrets \"authz-kubeconfig\" not found")),
				),
				Entry("ConfigMapName is provided and ConfigMap is compliant and referenced kubeconfig secret is present",
					func() {
						Expect(resourceConfigClient.Create(ctx, authorizationConfigurationCm)).To(Succeed())
						Expect(resourceConfigClient.Create(ctx, &corev1.Secret{
							ObjectMeta: metav1.ObjectMeta{Name: "authz-kubeconfig", Namespace: authorizationConfigurationCm.Namespace},
							Data:       map[string][]byte{"kubeconfig": []byte("webhook-kubeconfig")},
						})).To(Succeed())

						apiServerConfig = &gardencorev1beta1.KubeAPIServerConfig{
							StructuredAuthorization: &gardencorev1beta1.StructuredAuthorization{
								ConfigMapName: authorizationConfigurationCm.Name,
								Kubeconfigs:   []gardencorev1beta1.AuthorizerKubeconfigReference{{AuthorizerName: "webhook", SecretName: "authz-kubeconfig"}},
							},
						}
					},
					[]kubeapiserver.AuthorizationWebhook{
						{
							Name:                 "custom",
							Kubeconfig:           []byte("bar"),
							WebhookConfiguration: apiserverv1beta1.WebhookConfiguration{FailurePolicy: "Fail"},
						},
						{
							Name:       "webhook",
							Kubeconfig: []byte("webhook-kubeconfig"),
							WebhookConfiguration: apiserverv1beta1.WebhookConfiguration{
								AuthorizedTTL:                            metav1.Duration{Duration: 5 * time.Minute},
								UnauthorizedTTL:                          metav1.Duration{Duration: 30 * time.Second},
								Timeout:                                  metav1.Duration{Duration: 3 * time.Second},
								SubjectAccessReviewVersion:               "v1",
								MatchConditionSubjectAccessReviewVersion: "v1",
								FailurePolicy:                            "Deny",
								MatchConditions:                          []apiserverv1beta1.WebhookMatchCondition{{Expression: `request.resourceAttributes.namespace == 'kube-system'`}},
							},
						},
					},
					Not(HaveOccurred()),
				),
			)
		})

		Describe("DefaultNotReadyTolerationSeconds and DefaultUnreachableTolerationSeconds", func() {
			It("should not set the fields", func() {
				kubeAPIServer, err := NewKubeAPIServer(ctx, runtimeClientSet, resourceConfigClient, namespace, objectMeta, runtimeVersion, targetVersion, sm, namePrefix, apiServerConfig, autoscalingConfig, vpnConfig, priorityClassName, isWorkerless, runsAsStaticPod, auditWebhookConfig, authenticationWebhookConfig, authorizationWebhookConfigs, resourcesToStoreInETCDEvents)
				Expect(err).NotTo(HaveOccurred())
				Expect(kubeAPIServer.GetValues().DefaultNotReadyTolerationSeconds).To(BeNil())
				Expect(kubeAPIServer.GetValues().DefaultUnreachableTolerationSeconds).To(BeNil())
			})

			It("should set the fields to the configured values", func() {
				apiServerConfig = &gardencorev1beta1.KubeAPIServerConfig{
					DefaultNotReadyTolerationSeconds:    ptr.To[int64](120),
					DefaultUnreachableTolerationSeconds: ptr.To[int64](130),
				}

				kubeAPIServer, err := NewKubeAPIServer(ctx, runtimeClientSet, resourceConfigClient, namespace, objectMeta, runtimeVersion, targetVersion, sm, namePrefix, apiServerConfig, autoscalingConfig, vpnConfig, priorityClassName, isWorkerless, runsAsStaticPod, auditWebhookConfig, authenticationWebhookConfig, authorizationWebhookConfigs, resourcesToStoreInETCDEvents)
				Expect(err).NotTo(HaveOccurred())
				Expect(kubeAPIServer.GetValues().DefaultNotReadyTolerationSeconds).To(PointTo(Equal(int64(120))))
				Expect(kubeAPIServer.GetValues().DefaultUnreachableTolerationSeconds).To(PointTo(Equal(int64(130))))
			})
		})

		Describe("EventTTL", func() {
			It("should not set the event ttl field", func() {
				kubeAPIServer, err := NewKubeAPIServer(ctx, runtimeClientSet, resourceConfigClient, namespace, objectMeta, runtimeVersion, targetVersion, sm, namePrefix, apiServerConfig, autoscalingConfig, vpnConfig, priorityClassName, isWorkerless, runsAsStaticPod, auditWebhookConfig, authenticationWebhookConfig, authorizationWebhookConfigs, resourcesToStoreInETCDEvents)
				Expect(err).NotTo(HaveOccurred())
				Expect(kubeAPIServer.GetValues().EventTTL).To(BeNil())
			})

			It("should set the field to the configured values", func() {
				eventTTL := &metav1.Duration{Duration: 2 * time.Hour}

				apiServerConfig = &gardencorev1beta1.KubeAPIServerConfig{
					EventTTL: eventTTL,
				}

				kubeAPIServer, err := NewKubeAPIServer(ctx, runtimeClientSet, resourceConfigClient, namespace, objectMeta, runtimeVersion, targetVersion, sm, namePrefix, apiServerConfig, autoscalingConfig, vpnConfig, priorityClassName, isWorkerless, runsAsStaticPod, auditWebhookConfig, authenticationWebhookConfig, authorizationWebhookConfigs, resourcesToStoreInETCDEvents)
				Expect(err).NotTo(HaveOccurred())
				Expect(kubeAPIServer.GetValues().EventTTL).To(Equal(eventTTL))
			})
		})

		Describe("FeatureGates", func() {
			It("should set the field to nil by default", func() {
				kubeAPIServer, err := NewKubeAPIServer(ctx, runtimeClientSet, resourceConfigClient, namespace, objectMeta, runtimeVersion, targetVersion, sm, namePrefix, apiServerConfig, autoscalingConfig, vpnConfig, priorityClassName, isWorkerless, runsAsStaticPod, auditWebhookConfig, authenticationWebhookConfig, authorizationWebhookConfigs, resourcesToStoreInETCDEvents)
				Expect(err).NotTo(HaveOccurred())
				Expect(kubeAPIServer.GetValues().FeatureGates).To(BeNil())
			})

			It("should set the field to the configured values", func() {
				featureGates := map[string]bool{"foo": true, "bar": false}

				apiServerConfig = &gardencorev1beta1.KubeAPIServerConfig{
					KubernetesConfig: gardencorev1beta1.KubernetesConfig{
						FeatureGates: featureGates,
					},
				}

				kubeAPIServer, err := NewKubeAPIServer(ctx, runtimeClientSet, resourceConfigClient, namespace, objectMeta, runtimeVersion, targetVersion, sm, namePrefix, apiServerConfig, autoscalingConfig, vpnConfig, priorityClassName, isWorkerless, runsAsStaticPod, auditWebhookConfig, authenticationWebhookConfig, authorizationWebhookConfigs, resourcesToStoreInETCDEvents)
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

					kubeAPIServer, err := NewKubeAPIServer(ctx, runtimeClientSet, resourceConfigClient, namespace, objectMeta, runtimeVersion, targetVersion, sm, namePrefix, apiServerConfig, autoscalingConfig, vpnConfig, priorityClassName, isWorkerless, runsAsStaticPod, auditWebhookConfig, authenticationWebhookConfig, authorizationWebhookConfigs, resourcesToStoreInETCDEvents)
					Expect(err).NotTo(HaveOccurred())
					Expect(kubeAPIServer.GetValues().OIDC).To(Equal(expectedConfig))
				},

				Entry("KubeAPIServerConfig is nil",
					nil,
					nil,
				),
				Entry("OIDCConfig is nil",
					func() {
						apiServerConfig = &gardencorev1beta1.KubeAPIServerConfig{}
					},
					nil,
				),
				Entry("OIDCConfig is not nil",
					func() {
						apiServerConfig = &gardencorev1beta1.KubeAPIServerConfig{OIDCConfig: &gardencorev1beta1.OIDCConfig{}}
					},
					&gardencorev1beta1.OIDCConfig{},
				),
			)
		})

		Describe("Requests", func() {
			It("should set the field to nil by default", func() {
				kubeAPIServer, err := NewKubeAPIServer(ctx, runtimeClientSet, resourceConfigClient, namespace, objectMeta, runtimeVersion, targetVersion, sm, namePrefix, apiServerConfig, autoscalingConfig, vpnConfig, priorityClassName, isWorkerless, runsAsStaticPod, auditWebhookConfig, authenticationWebhookConfig, authorizationWebhookConfigs, resourcesToStoreInETCDEvents)
				Expect(err).NotTo(HaveOccurred())
				Expect(kubeAPIServer.GetValues().Requests).To(BeNil())
			})

			It("should set the field to the configured values", func() {
				requests := &gardencorev1beta1.APIServerRequests{
					MaxMutatingInflight:    ptr.To[int32](1),
					MaxNonMutatingInflight: ptr.To[int32](2),
				}
				apiServerConfig = &gardencorev1beta1.KubeAPIServerConfig{Requests: requests}

				kubeAPIServer, err := NewKubeAPIServer(ctx, runtimeClientSet, resourceConfigClient, namespace, objectMeta, runtimeVersion, targetVersion, sm, namePrefix, apiServerConfig, autoscalingConfig, vpnConfig, priorityClassName, isWorkerless, runsAsStaticPod, auditWebhookConfig, authenticationWebhookConfig, authorizationWebhookConfigs, resourcesToStoreInETCDEvents)
				Expect(err).NotTo(HaveOccurred())
				Expect(kubeAPIServer.GetValues().Requests).To(Equal(requests))
			})
		})

		Describe("RuntimeConfig", func() {
			It("should set the field to nil by default", func() {
				kubeAPIServer, err := NewKubeAPIServer(ctx, runtimeClientSet, resourceConfigClient, namespace, objectMeta, runtimeVersion, targetVersion, sm, namePrefix, apiServerConfig, autoscalingConfig, vpnConfig, priorityClassName, isWorkerless, runsAsStaticPod, auditWebhookConfig, authenticationWebhookConfig, authorizationWebhookConfigs, resourcesToStoreInETCDEvents)
				Expect(err).NotTo(HaveOccurred())
				Expect(kubeAPIServer.GetValues().RuntimeConfig).To(BeNil())
			})

			It("should set the field to the configured values", func() {
				runtimeConfig := map[string]bool{"foo": true, "bar": false}
				apiServerConfig = &gardencorev1beta1.KubeAPIServerConfig{RuntimeConfig: runtimeConfig}

				kubeAPIServer, err := NewKubeAPIServer(ctx, runtimeClientSet, resourceConfigClient, namespace, objectMeta, runtimeVersion, targetVersion, sm, namePrefix, apiServerConfig, autoscalingConfig, vpnConfig, priorityClassName, isWorkerless, runsAsStaticPod, auditWebhookConfig, authenticationWebhookConfig, authorizationWebhookConfigs, resourcesToStoreInETCDEvents)
				Expect(err).NotTo(HaveOccurred())
				Expect(kubeAPIServer.GetValues().RuntimeConfig).To(Equal(runtimeConfig))
			})
		})

		Describe("VPNConfig", func() {
			It("should set the field to the configured values", func() {
				vpnConfig = kubeapiserver.VPNConfig{Enabled: true}

				kubeAPIServer, err := NewKubeAPIServer(ctx, runtimeClientSet, resourceConfigClient, namespace, objectMeta, runtimeVersion, targetVersion, sm, namePrefix, apiServerConfig, autoscalingConfig, vpnConfig, priorityClassName, isWorkerless, runsAsStaticPod, auditWebhookConfig, authenticationWebhookConfig, authorizationWebhookConfigs, resourcesToStoreInETCDEvents)
				Expect(err).NotTo(HaveOccurred())
				Expect(kubeAPIServer.GetValues().VPN).To(Equal(vpnConfig))
			})
		})

		Describe("WatchCacheSizes", func() {
			It("should set the field to nil by default", func() {
				kubeAPIServer, err := NewKubeAPIServer(ctx, runtimeClientSet, resourceConfigClient, namespace, objectMeta, runtimeVersion, targetVersion, sm, namePrefix, apiServerConfig, autoscalingConfig, vpnConfig, priorityClassName, isWorkerless, runsAsStaticPod, auditWebhookConfig, authenticationWebhookConfig, authorizationWebhookConfigs, resourcesToStoreInETCDEvents)
				Expect(err).NotTo(HaveOccurred())
				Expect(kubeAPIServer.GetValues().WatchCacheSizes).To(BeNil())
			})

			It("should set the field to the configured values", func() {
				watchCacheSizes := &gardencorev1beta1.WatchCacheSizes{
					Default:   ptr.To[int32](1),
					Resources: []gardencorev1beta1.ResourceWatchCacheSize{{Resource: "foo"}},
				}
				apiServerConfig = &gardencorev1beta1.KubeAPIServerConfig{WatchCacheSizes: watchCacheSizes}

				kubeAPIServer, err := NewKubeAPIServer(ctx, runtimeClientSet, resourceConfigClient, namespace, objectMeta, runtimeVersion, targetVersion, sm, namePrefix, apiServerConfig, autoscalingConfig, vpnConfig, priorityClassName, isWorkerless, runsAsStaticPod, auditWebhookConfig, authenticationWebhookConfig, authorizationWebhookConfigs, resourcesToStoreInETCDEvents)
				Expect(err).NotTo(HaveOccurred())
				Expect(kubeAPIServer.GetValues().WatchCacheSizes).To(Equal(watchCacheSizes))
			})
		})

		Describe("PriorityClassName", func() {
			It("should set the field properly", func() {
				kubeAPIServer, err := NewKubeAPIServer(ctx, runtimeClientSet, resourceConfigClient, namespace, objectMeta, runtimeVersion, targetVersion, sm, namePrefix, apiServerConfig, autoscalingConfig, vpnConfig, priorityClassName, isWorkerless, runsAsStaticPod, auditWebhookConfig, authenticationWebhookConfig, authorizationWebhookConfigs, resourcesToStoreInETCDEvents)
				Expect(err).NotTo(HaveOccurred())
				Expect(kubeAPIServer.GetValues().PriorityClassName).To(Equal(priorityClassName))
			})
		})

		Describe("IsWorkerless", func() {
			It("should set the field properly", func() {
				kubeAPIServer, err := NewKubeAPIServer(ctx, runtimeClientSet, resourceConfigClient, namespace, objectMeta, runtimeVersion, targetVersion, sm, namePrefix, apiServerConfig, autoscalingConfig, vpnConfig, priorityClassName, isWorkerless, runsAsStaticPod, auditWebhookConfig, authenticationWebhookConfig, authorizationWebhookConfigs, resourcesToStoreInETCDEvents)
				Expect(err).NotTo(HaveOccurred())
				Expect(kubeAPIServer.GetValues().IsWorkerless).To(Equal(isWorkerless))
			})
		})

		Describe("AuthenticationWebhook", func() {
			It("should set the field properly", func() {
				kubeAPIServer, err := NewKubeAPIServer(ctx, runtimeClientSet, resourceConfigClient, namespace, objectMeta, runtimeVersion, targetVersion, sm, namePrefix, apiServerConfig, autoscalingConfig, vpnConfig, priorityClassName, isWorkerless, runsAsStaticPod, auditWebhookConfig, authenticationWebhookConfig, authorizationWebhookConfigs, resourcesToStoreInETCDEvents)
				Expect(err).NotTo(HaveOccurred())
				Expect(kubeAPIServer.GetValues().AuthenticationWebhook).To(Equal(authenticationWebhookConfig))
			})
		})

		Describe("AuthorizationWebhooks", func() {
			It("should set the field properly", func() {
				kubeAPIServer, err := NewKubeAPIServer(ctx, runtimeClientSet, resourceConfigClient, namespace, objectMeta, runtimeVersion, targetVersion, sm, namePrefix, apiServerConfig, autoscalingConfig, vpnConfig, priorityClassName, isWorkerless, runsAsStaticPod, auditWebhookConfig, authenticationWebhookConfig, authorizationWebhookConfigs, resourcesToStoreInETCDEvents)
				Expect(err).NotTo(HaveOccurred())
				Expect(kubeAPIServer.GetValues().AuthorizationWebhooks).To(Equal(authorizationWebhookConfigs))
			})
		})

		Describe("ResourcesToStoreInETCDEvents", func() {
			It("should set the field properly", func() {
				kubeAPIServer, err := NewKubeAPIServer(ctx, runtimeClientSet, resourceConfigClient, namespace, objectMeta, runtimeVersion, targetVersion, sm, namePrefix, apiServerConfig, autoscalingConfig, vpnConfig, priorityClassName, isWorkerless, runsAsStaticPod, auditWebhookConfig, authenticationWebhookConfig, authorizationWebhookConfigs, resourcesToStoreInETCDEvents)
				Expect(err).NotTo(HaveOccurred())
				Expect(kubeAPIServer.GetValues().ResourcesToStoreInETCDEvents).To(Equal(resourcesToStoreInETCDEvents))
			})
		})
	})

	Describe("#DeployKubeAPIServer", func() {
		var (
			ctrl          *gomock.Controller
			kubeAPIServer *mockkubeapiserver.MockInterface

			serverCertificateConfig        kubeapiserver.ServerCertificateConfig
			sniConfig                      kubeapiserver.SNIConfig
			externalHostname               string
			nodeNetworkCIDRs               []net.IPNet
			podNetworkCIDRs                []net.IPNet
			serviceNetworkCIDRs            []net.IPNet
			etcdEncryptionKeyRotationPhase gardencorev1beta1.CredentialsRotationPhase
			wantScaleDown                  bool
		)

		BeforeEach(func() {
			ctrl = gomock.NewController(GinkgoT())
			DeferCleanup(func() { ctrl.Finish() })

			kubeAPIServer = mockkubeapiserver.NewMockInterface(ctrl)

			serverCertificateConfig = kubeapiserver.ServerCertificateConfig{ExtraDNSNames: []string{"foo"}}
			sniConfig = kubeapiserver.SNIConfig{Enabled: true}
			externalHostname = "external-hostname"
			etcdEncryptionKeyRotationPhase = ""
			nodeNetworkCIDRs = []net.IPNet{{IP: net.ParseIP("10.250.0.0"), Mask: net.CIDRMask(24, 32)}}
			serviceNetworkCIDRs = []net.IPNet{{IP: net.ParseIP("10.0.2.0"), Mask: net.CIDRMask(24, 32)}}
			podNetworkCIDRs = []net.IPNet{{IP: net.ParseIP("10.0.1.0"), Mask: net.CIDRMask(24, 32)}}
			wantScaleDown = false
		})

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

				kubeAPIServer.EXPECT().GetValues().Return(kubeapiserver.Values{
					Autoscaling: autoscalingConfig,
				},
				)
				kubeAPIServer.EXPECT().SetAutoscalingReplicas(gomock.Any())
				if expectedResources != nil {
					kubeAPIServer.EXPECT().SetAutoscalingAPIServerResources(*expectedResources)
				}
				kubeAPIServer.EXPECT().SetSNIConfig(gomock.Any())
				kubeAPIServer.EXPECT().SetETCDEncryptionConfig(gomock.Any())
				kubeAPIServer.EXPECT().SetExternalHostname(gomock.Any())
				kubeAPIServer.EXPECT().SetNodeNetworkCIDRs(gomock.Any())
				kubeAPIServer.EXPECT().SetServiceNetworkCIDRs(gomock.Any())
				kubeAPIServer.EXPECT().SetPodNetworkCIDRs(gomock.Any())
				kubeAPIServer.EXPECT().SetServerCertificateConfig(gomock.Any())
				kubeAPIServer.EXPECT().SetServiceAccountConfig(gomock.Any())
				kubeAPIServer.EXPECT().Deploy(ctx)

				Expect(DeployKubeAPIServer(ctx, runtimeClient, namespace, kubeAPIServer, serviceAccountConfig, serverCertificateConfig, sniConfig, externalHostname, nodeNetworkCIDRs, serviceNetworkCIDRs, podNetworkCIDRs, nil, nil, etcdEncryptionKeyRotationPhase, wantScaleDown)).To(Succeed())
			},

			Entry("nothing is set when deployment is not found",
				nil,
				kubeapiserver.AutoscalingConfig{},
				nil,
			),
			Entry("set the existing requirements when the deployment is found and scale-down is disabled",
				func() {
					Expect(runtimeClient.Create(ctx, &appsv1.Deployment{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "kube-apiserver",
							Namespace: namespace,
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
				kubeapiserver.AutoscalingConfig{ScaleDownDisabled: true},
				&apiServerResources,
			),
			Entry("set the existing requirements when the deployment is found and scale-down is enabled",
				func() {
					Expect(runtimeClient.Create(ctx, &appsv1.Deployment{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "kube-apiserver",
							Namespace: namespace,
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
				kubeapiserver.AutoscalingConfig{},
				&apiServerResources,
			),
		)

		DescribeTable("should correctly set the autoscaling replicas",
			func(prepTest func(), autoscalingConfig kubeapiserver.AutoscalingConfig, expectedReplicas int32) {
				if prepTest != nil {
					prepTest()
				}

				kubeAPIServer.EXPECT().GetValues().Return(kubeapiserver.Values{
					Autoscaling: autoscalingConfig,
				})
				kubeAPIServer.EXPECT().SetAutoscalingReplicas(&expectedReplicas)
				kubeAPIServer.EXPECT().SetSNIConfig(gomock.Any())
				kubeAPIServer.EXPECT().SetETCDEncryptionConfig(gomock.Any())
				kubeAPIServer.EXPECT().SetExternalHostname(gomock.Any())
				kubeAPIServer.EXPECT().SetNodeNetworkCIDRs(gomock.Any())
				kubeAPIServer.EXPECT().SetServiceNetworkCIDRs(gomock.Any())
				kubeAPIServer.EXPECT().SetPodNetworkCIDRs(gomock.Any())
				kubeAPIServer.EXPECT().SetServerCertificateConfig(gomock.Any())
				kubeAPIServer.EXPECT().SetServiceAccountConfig(gomock.Any())
				kubeAPIServer.EXPECT().Deploy(ctx)

				Expect(DeployKubeAPIServer(ctx, runtimeClient, namespace, kubeAPIServer, serviceAccountConfig, serverCertificateConfig, sniConfig, externalHostname, nodeNetworkCIDRs, serviceNetworkCIDRs, podNetworkCIDRs, nil, nil, etcdEncryptionKeyRotationPhase, wantScaleDown)).To(Succeed())
			},

			Entry("no change due to already set",
				nil,
				kubeapiserver.AutoscalingConfig{Replicas: ptr.To[int32](1)},
				int32(1),
			),
			Entry("use minReplicas because deployment does not exist",
				nil,
				kubeapiserver.AutoscalingConfig{MinReplicas: 2},
				int32(2),
			),
			Entry("use 0 because shoot is hibernated, even  if deployment does not exist",
				func() {
					wantScaleDown = true
				},
				kubeapiserver.AutoscalingConfig{MinReplicas: 2},
				int32(0),
			),
			Entry("use deployment replicas because they are greater than 0",
				func() {
					Expect(runtimeClient.Create(ctx, &appsv1.Deployment{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "kube-apiserver",
							Namespace: namespace,
						},
						Spec: appsv1.DeploymentSpec{
							Replicas: ptr.To[int32](3),
						},
					})).To(Succeed())
				},
				kubeapiserver.AutoscalingConfig{},
				int32(3),
			),
			Entry("use 0 because shoot is hibernated and deployment is already scaled down",
				func() {
					wantScaleDown = true
					Expect(runtimeClient.Create(ctx, &appsv1.Deployment{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "kube-apiserver",
							Namespace: namespace,
						},
						Spec: appsv1.DeploymentSpec{
							Replicas: ptr.To[int32](0),
						},
					})).To(Succeed())
				},
				kubeapiserver.AutoscalingConfig{},
				int32(0),
			),
		)

		DescribeTable("ETCD Encryption Key rotation",
			func(rotationPhase gardencorev1beta1.CredentialsRotationPhase, prepTest func(), expectedETCDEncryptionConfig apiserver.ETCDEncryptionConfig, finalizeTest func()) {
				if len(rotationPhase) > 0 {
					etcdEncryptionKeyRotationPhase = rotationPhase
				}

				if prepTest != nil {
					prepTest()
				}

				kubeAPIServer.EXPECT().GetValues()
				kubeAPIServer.EXPECT().SetAutoscalingReplicas(gomock.Any())
				kubeAPIServer.EXPECT().SetSNIConfig(gomock.Any())
				kubeAPIServer.EXPECT().SetETCDEncryptionConfig(expectedETCDEncryptionConfig)
				kubeAPIServer.EXPECT().SetExternalHostname(gomock.Any())
				kubeAPIServer.EXPECT().SetNodeNetworkCIDRs(gomock.Any())
				kubeAPIServer.EXPECT().SetServiceNetworkCIDRs(gomock.Any())
				kubeAPIServer.EXPECT().SetPodNetworkCIDRs(gomock.Any())
				kubeAPIServer.EXPECT().SetServerCertificateConfig(gomock.Any())
				kubeAPIServer.EXPECT().SetServiceAccountConfig(gomock.Any())
				kubeAPIServer.EXPECT().Deploy(ctx)

				Expect(DeployKubeAPIServer(ctx, runtimeClient, namespace, kubeAPIServer, serviceAccountConfig, serverCertificateConfig, sniConfig, externalHostname, nodeNetworkCIDRs, serviceNetworkCIDRs, podNetworkCIDRs, nil, nil, etcdEncryptionKeyRotationPhase, wantScaleDown)).To(Succeed())

				if finalizeTest != nil {
					finalizeTest()
				}
			},

			Entry("no rotation",
				gardencorev1beta1.CredentialsRotationPhase(""),
				nil,
				apiserver.ETCDEncryptionConfig{EncryptWithCurrentKey: true, ResourcesToEncrypt: []string{"secrets"}, EncryptedResources: []string{"secrets"}},
				nil,
			),
			Entry("preparing phase, new key already populated",
				gardencorev1beta1.RotationPreparing,
				func() {
					Expect(runtimeClient.Create(ctx, &appsv1.Deployment{
						TypeMeta: metav1.TypeMeta{APIVersion: "apps/v1", Kind: "Deployment"},
						ObjectMeta: metav1.ObjectMeta{
							Name:        "kube-apiserver",
							Namespace:   namespace,
							Annotations: map[string]string{"credentials.gardener.cloud/new-encryption-key-populated": "true"},
						},
					})).To(Succeed())
				},
				apiserver.ETCDEncryptionConfig{RotationPhase: gardencorev1beta1.RotationPreparing, EncryptWithCurrentKey: true, ResourcesToEncrypt: []string{"secrets"}, EncryptedResources: []string{"secrets"}},
				nil,
			),
			Entry("preparing phase, new key not yet populated",
				gardencorev1beta1.RotationPreparing,
				func() {
					Expect(runtimeClient.Create(ctx, &appsv1.Deployment{
						TypeMeta: metav1.TypeMeta{APIVersion: "apps/v1", Kind: "Deployment"},
						ObjectMeta: metav1.ObjectMeta{
							Name:      "kube-apiserver",
							Namespace: namespace,
						},
					})).To(Succeed())

					kubeAPIServer.EXPECT().Wait(ctx)

					kubeAPIServer.EXPECT().SetETCDEncryptionConfig(apiserver.ETCDEncryptionConfig{
						RotationPhase:         gardencorev1beta1.RotationPreparing,
						EncryptWithCurrentKey: true,
						ResourcesToEncrypt:    []string{"secrets"},
						EncryptedResources:    []string{"secrets"},
					})
					kubeAPIServer.EXPECT().Deploy(ctx)
				},
				apiserver.ETCDEncryptionConfig{RotationPhase: gardencorev1beta1.RotationPreparing, EncryptWithCurrentKey: false, ResourcesToEncrypt: []string{"secrets"}, EncryptedResources: []string{"secrets"}},
				func() {
					deployment := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "kube-apiserver", Namespace: namespace}}
					Expect(runtimeClient.Get(ctx, client.ObjectKeyFromObject(deployment), deployment)).To(Succeed())
					Expect(deployment.Annotations).To(HaveKeyWithValue("credentials.gardener.cloud/new-encryption-key-populated", "true"))
				},
			),
			Entry("prepared phase",
				gardencorev1beta1.RotationPrepared,
				nil,
				apiserver.ETCDEncryptionConfig{RotationPhase: gardencorev1beta1.RotationPrepared, EncryptWithCurrentKey: true, ResourcesToEncrypt: []string{"secrets"}, EncryptedResources: []string{"secrets"}},
				nil,
			),
			Entry("completing phase",
				gardencorev1beta1.RotationCompleting,
				func() {
					Expect(runtimeClient.Create(ctx, &appsv1.Deployment{
						TypeMeta: metav1.TypeMeta{APIVersion: "apps/v1", Kind: "Deployment"},
						ObjectMeta: metav1.ObjectMeta{
							Name:        "kube-apiserver",
							Namespace:   namespace,
							Annotations: map[string]string{"credentials.gardener.cloud/new-encryption-key-populated": "true"},
						},
					})).To(Succeed())
				},
				apiserver.ETCDEncryptionConfig{RotationPhase: gardencorev1beta1.RotationCompleting, EncryptWithCurrentKey: true, ResourcesToEncrypt: []string{"secrets"}, EncryptedResources: []string{"secrets"}},
				func() {
					deployment := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "kube-apiserver", Namespace: namespace}}
					Expect(runtimeClient.Get(ctx, client.ObjectKeyFromObject(deployment), deployment)).To(Succeed())
					Expect(deployment.Annotations).NotTo(HaveKey("credentials.gardener.cloud/new-encryption-key-populated"))
				},
			),
			Entry("completed phase",
				gardencorev1beta1.RotationCompleted,
				nil,
				apiserver.ETCDEncryptionConfig{RotationPhase: gardencorev1beta1.RotationCompleted, EncryptWithCurrentKey: true, ResourcesToEncrypt: []string{"secrets"}, EncryptedResources: []string{"secrets"}},
				nil,
			),
		)

		Describe("ETCDEncryptionConfig", func() {
			It("It should deploy KubeAPIServer with the default ETCDEncryptionConfig when resources are nil", func() {
				expectedETCDEncryptionConfig := apiserver.ETCDEncryptionConfig{
					EncryptWithCurrentKey: true,
					ResourcesToEncrypt: []string{
						"secrets",
					},
					EncryptedResources: []string{
						"secrets",
					},
				}

				kubeAPIServer.EXPECT().GetValues()
				kubeAPIServer.EXPECT().SetAutoscalingReplicas(gomock.Any())
				kubeAPIServer.EXPECT().SetSNIConfig(gomock.Any())
				kubeAPIServer.EXPECT().SetETCDEncryptionConfig(expectedETCDEncryptionConfig)
				kubeAPIServer.EXPECT().SetExternalHostname(gomock.Any())
				kubeAPIServer.EXPECT().SetNodeNetworkCIDRs(gomock.Any())
				kubeAPIServer.EXPECT().SetServiceNetworkCIDRs(gomock.Any())
				kubeAPIServer.EXPECT().SetPodNetworkCIDRs(gomock.Any())
				kubeAPIServer.EXPECT().SetServerCertificateConfig(gomock.Any())
				kubeAPIServer.EXPECT().SetServiceAccountConfig(gomock.Any())
				kubeAPIServer.EXPECT().Deploy(ctx)

				Expect(DeployKubeAPIServer(ctx, runtimeClient, namespace, kubeAPIServer, serviceAccountConfig, serverCertificateConfig, sniConfig, externalHostname, nodeNetworkCIDRs, serviceNetworkCIDRs, podNetworkCIDRs, nil, nil, etcdEncryptionKeyRotationPhase, wantScaleDown)).To(Succeed())
			})

			It("It should deploy KubeAPIServer with the default resources appended to the passed resources", func() {
				expectedETCDEncryptionConfig := apiserver.ETCDEncryptionConfig{
					EncryptWithCurrentKey: true,
					ResourcesToEncrypt: []string{
						"configmaps",
						"customresource.fancyoperator.io",
						"secrets",
					},
					EncryptedResources: []string{
						"deployments.apps",
						"secrets",
					},
				}

				kubeAPIServer.EXPECT().GetValues()
				kubeAPIServer.EXPECT().SetAutoscalingReplicas(gomock.Any())
				kubeAPIServer.EXPECT().SetSNIConfig(gomock.Any())
				kubeAPIServer.EXPECT().SetETCDEncryptionConfig(expectedETCDEncryptionConfig)
				kubeAPIServer.EXPECT().SetExternalHostname(gomock.Any())
				kubeAPIServer.EXPECT().SetNodeNetworkCIDRs(gomock.Any())
				kubeAPIServer.EXPECT().SetServiceNetworkCIDRs(gomock.Any())
				kubeAPIServer.EXPECT().SetPodNetworkCIDRs(gomock.Any())
				kubeAPIServer.EXPECT().SetServerCertificateConfig(gomock.Any())
				kubeAPIServer.EXPECT().SetServiceAccountConfig(gomock.Any())
				kubeAPIServer.EXPECT().Deploy(ctx)

				resourcesToEncrypt := []string{
					"configmaps",
					"customresource.fancyoperator.io",
				}

				encryptedResources := []string{
					"deployments.apps",
				}

				Expect(DeployKubeAPIServer(ctx, runtimeClient, namespace, kubeAPIServer, serviceAccountConfig, serverCertificateConfig, sniConfig, externalHostname, nodeNetworkCIDRs, serviceNetworkCIDRs, podNetworkCIDRs, resourcesToEncrypt, encryptedResources, etcdEncryptionKeyRotationPhase, wantScaleDown)).To(Succeed())
			})
		})

		Describe("External{Hostname,Server}", func() {
			It("should set the external {hostname,server} to the provided addresses", func() {
				kubeAPIServer.EXPECT().GetValues()
				kubeAPIServer.EXPECT().SetAutoscalingReplicas(gomock.Any())
				kubeAPIServer.EXPECT().SetSNIConfig(gomock.Any())
				kubeAPIServer.EXPECT().SetETCDEncryptionConfig(gomock.Any())
				kubeAPIServer.EXPECT().SetExternalHostname(externalHostname)
				kubeAPIServer.EXPECT().SetNodeNetworkCIDRs(gomock.Any())
				kubeAPIServer.EXPECT().SetServiceNetworkCIDRs(gomock.Any())
				kubeAPIServer.EXPECT().SetPodNetworkCIDRs(gomock.Any())
				kubeAPIServer.EXPECT().SetServerCertificateConfig(gomock.Any())
				kubeAPIServer.EXPECT().SetServiceAccountConfig(gomock.Any())
				kubeAPIServer.EXPECT().Deploy(ctx)

				Expect(DeployKubeAPIServer(ctx, runtimeClient, namespace, kubeAPIServer, serviceAccountConfig, serverCertificateConfig, sniConfig, externalHostname, nodeNetworkCIDRs, serviceNetworkCIDRs, podNetworkCIDRs, nil, nil, etcdEncryptionKeyRotationPhase, wantScaleDown)).To(Succeed())
			})
		})

		Describe("ServerCertificateConfig", func() {
			It("should set the field to the provided config", func() {
				kubeAPIServer.EXPECT().GetValues()
				kubeAPIServer.EXPECT().SetAutoscalingReplicas(gomock.Any())
				kubeAPIServer.EXPECT().SetSNIConfig(gomock.Any())
				kubeAPIServer.EXPECT().SetETCDEncryptionConfig(gomock.Any())
				kubeAPIServer.EXPECT().SetExternalHostname(gomock.Any())
				kubeAPIServer.EXPECT().SetNodeNetworkCIDRs(gomock.Any())
				kubeAPIServer.EXPECT().SetServiceNetworkCIDRs(gomock.Any())
				kubeAPIServer.EXPECT().SetPodNetworkCIDRs(gomock.Any())
				kubeAPIServer.EXPECT().SetServerCertificateConfig(serverCertificateConfig)
				kubeAPIServer.EXPECT().SetServiceAccountConfig(gomock.Any())
				kubeAPIServer.EXPECT().Deploy(ctx)

				Expect(DeployKubeAPIServer(ctx, runtimeClient, namespace, kubeAPIServer, serviceAccountConfig, serverCertificateConfig, sniConfig, externalHostname, nodeNetworkCIDRs, serviceNetworkCIDRs, podNetworkCIDRs, nil, nil, etcdEncryptionKeyRotationPhase, wantScaleDown)).To(Succeed())
			})
		})

		Describe("ServiceAccountConfig", func() {
			It("should set the field to the provided config", func() {
				kubeAPIServer.EXPECT().GetValues()
				kubeAPIServer.EXPECT().SetAutoscalingReplicas(gomock.Any())
				kubeAPIServer.EXPECT().SetSNIConfig(gomock.Any())
				kubeAPIServer.EXPECT().SetETCDEncryptionConfig(gomock.Any())
				kubeAPIServer.EXPECT().SetExternalHostname(gomock.Any())
				kubeAPIServer.EXPECT().SetNodeNetworkCIDRs(gomock.Any())
				kubeAPIServer.EXPECT().SetServiceNetworkCIDRs(gomock.Any())
				kubeAPIServer.EXPECT().SetPodNetworkCIDRs(gomock.Any())
				kubeAPIServer.EXPECT().SetServerCertificateConfig(gomock.Any())
				kubeAPIServer.EXPECT().SetServiceAccountConfig(serviceAccountConfig)
				kubeAPIServer.EXPECT().Deploy(ctx)

				Expect(DeployKubeAPIServer(ctx, runtimeClient, namespace, kubeAPIServer, serviceAccountConfig, serverCertificateConfig, sniConfig, externalHostname, nodeNetworkCIDRs, serviceNetworkCIDRs, podNetworkCIDRs, nil, nil, etcdEncryptionKeyRotationPhase, wantScaleDown)).To(Succeed())
			})
		})

		Describe("SNIConfig", func() {
			It("should set the field to the provided config", func() {
				kubeAPIServer.EXPECT().GetValues()
				kubeAPIServer.EXPECT().SetAutoscalingReplicas(gomock.Any())
				kubeAPIServer.EXPECT().SetSNIConfig(sniConfig)
				kubeAPIServer.EXPECT().SetETCDEncryptionConfig(gomock.Any())
				kubeAPIServer.EXPECT().SetExternalHostname(gomock.Any())
				kubeAPIServer.EXPECT().SetNodeNetworkCIDRs(gomock.Any())
				kubeAPIServer.EXPECT().SetServiceNetworkCIDRs(gomock.Any())
				kubeAPIServer.EXPECT().SetPodNetworkCIDRs(gomock.Any())
				kubeAPIServer.EXPECT().SetServerCertificateConfig(gomock.Any())
				kubeAPIServer.EXPECT().SetServiceAccountConfig(gomock.Any())
				kubeAPIServer.EXPECT().Deploy(ctx)

				Expect(DeployKubeAPIServer(ctx, runtimeClient, namespace, kubeAPIServer, serviceAccountConfig, serverCertificateConfig, sniConfig, externalHostname, nodeNetworkCIDRs, serviceNetworkCIDRs, podNetworkCIDRs, nil, nil, etcdEncryptionKeyRotationPhase, wantScaleDown)).To(Succeed())
			})
		})

		Describe("NodeNetworkCIDR", func() {
			It("should set the field to the provided config", func() {
				kubeAPIServer.EXPECT().GetValues()
				kubeAPIServer.EXPECT().SetAutoscalingReplicas(gomock.Any())
				kubeAPIServer.EXPECT().SetSNIConfig(sniConfig)
				kubeAPIServer.EXPECT().SetETCDEncryptionConfig(gomock.Any())
				kubeAPIServer.EXPECT().SetExternalHostname(gomock.Any())
				kubeAPIServer.EXPECT().SetNodeNetworkCIDRs(gomock.Any())
				kubeAPIServer.EXPECT().SetServiceNetworkCIDRs(gomock.Any())
				kubeAPIServer.EXPECT().SetPodNetworkCIDRs(gomock.Any())
				kubeAPIServer.EXPECT().SetServerCertificateConfig(gomock.Any())
				kubeAPIServer.EXPECT().SetServiceAccountConfig(gomock.Any())
				kubeAPIServer.EXPECT().Deploy(ctx)

				Expect(DeployKubeAPIServer(ctx, runtimeClient, namespace, kubeAPIServer, serviceAccountConfig, serverCertificateConfig, sniConfig, externalHostname, nodeNetworkCIDRs, serviceNetworkCIDRs, podNetworkCIDRs, nil, nil, etcdEncryptionKeyRotationPhase, wantScaleDown)).To(Succeed())
			})
		})
	})
})
