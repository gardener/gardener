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
	"time"

	"github.com/Masterminds/semver"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	gomegatypes "github.com/onsi/gomega/types"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kubernetesscheme "k8s.io/client-go/kubernetes/scheme"
	admissionapiv1 "k8s.io/pod-security-admission/admission/api/v1"
	admissionapiv1alpha1 "k8s.io/pod-security-admission/admission/api/v1alpha1"
	admissionapiv1beta1 "k8s.io/pod-security-admission/admission/api/v1beta1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/fake"
	"github.com/gardener/gardener/pkg/component/apiserver"
	"github.com/gardener/gardener/pkg/component/kubeapiserver"
	mockkubeapiserver "github.com/gardener/gardener/pkg/component/kubeapiserver/mock"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	fakesecretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager/fake"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("KubeAPIServer", func() {
	var (
		ctx = context.TODO()

		runtimeClient   client.Client
		namespace       string
		apiServerConfig *gardencorev1beta1.KubeAPIServerConfig
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
			imageVector                  imagevector.ImageVector
			namePrefix                   string
			serviceNetworkCIDR           string
			autoscalingConfig            apiserver.AutoscalingConfig
			vpnConfig                    kubeapiserver.VPNConfig
			priorityClassName            string
			isWorkerless                 bool
			staticTokenKubeconfigEnabled *bool
			auditWebhookConfig           *apiserver.AuditWebhook
			authenticationWebhookConfig  *kubeapiserver.AuthenticationWebhook
			authorizationWebhookConfig   *kubeapiserver.AuthorizationWebhook
			resourcesToStoreInETCDEvents []schema.GroupResource

			runtimeClientSet     kubernetes.Interface
			resourceConfigClient client.Client
			sm                   secretsmanager.Interface
		)

		BeforeEach(func() {
			name = "bar"
			objectMeta = metav1.ObjectMeta{Namespace: namespace, Name: name}
			runtimeVersion = semver.MustParse("1.22.0")
			targetVersion = semver.MustParse("1.22.1")
			imageVector = imagevector.ImageVector{{Name: "kube-apiserver"}}
			namePrefix = ""
			serviceNetworkCIDR = "10.0.2.0/24"
			autoscalingConfig = apiserver.AutoscalingConfig{}
			vpnConfig = kubeapiserver.VPNConfig{}
			priorityClassName = "priority-class"
			isWorkerless = false
			staticTokenKubeconfigEnabled = nil
			auditWebhookConfig = nil
			authenticationWebhookConfig = &kubeapiserver.AuthenticationWebhook{Version: pointer.String("authn-version")}
			authorizationWebhookConfig = &kubeapiserver.AuthorizationWebhook{Version: pointer.String("authnz-version")}
			resourcesToStoreInETCDEvents = []schema.GroupResource{{Resource: "foo", Group: "bar"}}

			secret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "secret-1",
					Namespace: namespace,
				},
				Data: map[string][]byte{"kubeconfig": []byte("kubeconfig-data")},
			}

			runtimeClientSet = fake.NewClientSetBuilder().WithClient(runtimeClient).WithVersion("1.22.0").Build()
			resourceConfigClient = fakeclient.NewClientBuilder().WithScheme(kubernetesscheme.Scheme).Build()
			sm = fakesecretsmanager.New(runtimeClient, namespace)
		})

		Describe("Images", func() {
			It("should return an error because the kube-apiserver cannot be found", func() {
				imageVector = imagevector.ImageVector{}

				kubeAPIServer, err := NewKubeAPIServer(ctx, runtimeClientSet, resourceConfigClient, namespace, objectMeta, runtimeVersion, targetVersion, imageVector, sm, namePrefix, apiServerConfig, autoscalingConfig, serviceNetworkCIDR, vpnConfig, priorityClassName, isWorkerless, staticTokenKubeconfigEnabled, auditWebhookConfig, authenticationWebhookConfig, authorizationWebhookConfig, resourcesToStoreInETCDEvents)
				Expect(kubeAPIServer).To(BeNil())
				Expect(err).To(MatchError(ContainSubstring("could not find image \"kube-apiserver\"")))
			})

			It("should return an error because the alpine cannot be found", func() {
				targetVersion = semver.MustParse("1.24.4")

				kubeAPIServer, err := NewKubeAPIServer(ctx, runtimeClientSet, resourceConfigClient, namespace, objectMeta, runtimeVersion, targetVersion, imageVector, sm, namePrefix, apiServerConfig, autoscalingConfig, serviceNetworkCIDR, vpnConfig, priorityClassName, isWorkerless, staticTokenKubeconfigEnabled, auditWebhookConfig, authenticationWebhookConfig, authorizationWebhookConfig, resourcesToStoreInETCDEvents)
				Expect(kubeAPIServer).To(BeNil())
				Expect(err).To(MatchError(ContainSubstring("could not find image \"alpine\"")))
			})

			It("should return an error because the alpine cannot be found", func() {
				vpnConfig.HighAvailabilityEnabled = true

				kubeAPIServer, err := NewKubeAPIServer(ctx, runtimeClientSet, resourceConfigClient, namespace, objectMeta, runtimeVersion, targetVersion, imageVector, sm, namePrefix, apiServerConfig, autoscalingConfig, serviceNetworkCIDR, vpnConfig, priorityClassName, isWorkerless, staticTokenKubeconfigEnabled, auditWebhookConfig, authenticationWebhookConfig, authorizationWebhookConfig, resourcesToStoreInETCDEvents)
				Expect(kubeAPIServer).To(BeNil())
				Expect(err).To(MatchError(ContainSubstring("could not find image \"vpn-shoot-client\"")))
			})

			It("should succeed because all images can be found", func() {
				targetVersion = semver.MustParse("1.24.4")
				vpnConfig.HighAvailabilityEnabled = true
				imageVector = append(imageVector, &imagevector.ImageSource{Name: "alpine"}, &imagevector.ImageSource{Name: "vpn-shoot-client"})

				kubeAPIServer, err := NewKubeAPIServer(ctx, runtimeClientSet, resourceConfigClient, namespace, objectMeta, runtimeVersion, targetVersion, imageVector, sm, namePrefix, apiServerConfig, autoscalingConfig, serviceNetworkCIDR, vpnConfig, priorityClassName, isWorkerless, staticTokenKubeconfigEnabled, auditWebhookConfig, authenticationWebhookConfig, authorizationWebhookConfig, resourcesToStoreInETCDEvents)
				Expect(kubeAPIServer).NotTo(BeNil())
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Describe("AnonymousAuthenticationEnabled", func() {
			It("should set the field to false by default", func() {
				kubeAPIServer, err := NewKubeAPIServer(ctx, runtimeClientSet, resourceConfigClient, namespace, objectMeta, runtimeVersion, targetVersion, imageVector, sm, namePrefix, apiServerConfig, autoscalingConfig, serviceNetworkCIDR, vpnConfig, priorityClassName, isWorkerless, staticTokenKubeconfigEnabled, auditWebhookConfig, authenticationWebhookConfig, authorizationWebhookConfig, resourcesToStoreInETCDEvents)
				Expect(err).NotTo(HaveOccurred())
				Expect(kubeAPIServer.GetValues().AnonymousAuthenticationEnabled).To(BeFalse())
			})

			It("should set the field to true if explicitly enabled", func() {
				apiServerConfig = &gardencorev1beta1.KubeAPIServerConfig{EnableAnonymousAuthentication: pointer.Bool(true)}

				kubeAPIServer, err := NewKubeAPIServer(ctx, runtimeClientSet, resourceConfigClient, namespace, objectMeta, runtimeVersion, targetVersion, imageVector, sm, namePrefix, apiServerConfig, autoscalingConfig, serviceNetworkCIDR, vpnConfig, priorityClassName, isWorkerless, staticTokenKubeconfigEnabled, auditWebhookConfig, authenticationWebhookConfig, authorizationWebhookConfig, resourcesToStoreInETCDEvents)
				Expect(err).NotTo(HaveOccurred())
				Expect(kubeAPIServer.GetValues().AnonymousAuthenticationEnabled).To(BeTrue())
			})
		})

		Describe("APIAudiences", func() {
			It("should set the field to 'kubernetes' and 'gardener' by default", func() {
				kubeAPIServer, err := NewKubeAPIServer(ctx, runtimeClientSet, resourceConfigClient, namespace, objectMeta, runtimeVersion, targetVersion, imageVector, sm, namePrefix, apiServerConfig, autoscalingConfig, serviceNetworkCIDR, vpnConfig, priorityClassName, isWorkerless, staticTokenKubeconfigEnabled, auditWebhookConfig, authenticationWebhookConfig, authorizationWebhookConfig, resourcesToStoreInETCDEvents)
				Expect(err).NotTo(HaveOccurred())
				Expect(kubeAPIServer.GetValues().APIAudiences).To(ConsistOf("kubernetes", "gardener"))
			})

			It("should set the field to the configured values", func() {
				apiAudiences := []string{"foo", "bar"}
				apiServerConfig = &gardencorev1beta1.KubeAPIServerConfig{APIAudiences: apiAudiences}

				kubeAPIServer, err := NewKubeAPIServer(ctx, runtimeClientSet, resourceConfigClient, namespace, objectMeta, runtimeVersion, targetVersion, imageVector, sm, namePrefix, apiServerConfig, autoscalingConfig, serviceNetworkCIDR, vpnConfig, priorityClassName, isWorkerless, staticTokenKubeconfigEnabled, auditWebhookConfig, authenticationWebhookConfig, authorizationWebhookConfig, resourcesToStoreInETCDEvents)
				Expect(err).NotTo(HaveOccurred())
				Expect(kubeAPIServer.GetValues().APIAudiences).To(Equal(append(apiAudiences, "gardener")))
			})

			It("should not add gardener audience if already present", func() {
				apiAudiences := []string{"foo", "bar", "gardener"}
				apiServerConfig = &gardencorev1beta1.KubeAPIServerConfig{APIAudiences: apiAudiences}

				kubeAPIServer, err := NewKubeAPIServer(ctx, runtimeClientSet, resourceConfigClient, namespace, objectMeta, runtimeVersion, targetVersion, imageVector, sm, namePrefix, apiServerConfig, autoscalingConfig, serviceNetworkCIDR, vpnConfig, priorityClassName, isWorkerless, staticTokenKubeconfigEnabled, auditWebhookConfig, authenticationWebhookConfig, authorizationWebhookConfig, resourcesToStoreInETCDEvents)
				Expect(err).NotTo(HaveOccurred())
				Expect(kubeAPIServer.GetValues().APIAudiences).To(Equal(apiAudiences))
			})
		})

		Describe("AdmissionPlugins", func() {
			BeforeEach(func() {
				Expect(resourceConfigClient.Create(ctx, secret)).To(BeNil())
				apiServerConfig = &gardencorev1beta1.KubeAPIServerConfig{AdmissionPlugins: []gardencorev1beta1.AdmissionPlugin{}}
			})

			DescribeTable("should have the expected admission plugins config",
				func(configuredPlugins []gardencorev1beta1.AdmissionPlugin, expectedPlugins []apiserver.AdmissionPluginConfig, isWorkerless bool) {
					apiServerConfig.AdmissionPlugins = configuredPlugins

					kubeAPIServer, err := NewKubeAPIServer(ctx, runtimeClientSet, resourceConfigClient, namespace, objectMeta, runtimeVersion, targetVersion, imageVector, sm, namePrefix, apiServerConfig, autoscalingConfig, serviceNetworkCIDR, vpnConfig, priorityClassName, isWorkerless, staticTokenKubeconfigEnabled, auditWebhookConfig, authenticationWebhookConfig, authorizationWebhookConfig, resourcesToStoreInETCDEvents)
					Expect(err).NotTo(HaveOccurred())
					Expect(kubeAPIServer.GetValues().EnabledAdmissionPlugins).To(Equal(expectedPlugins))
				},

				Entry("only default plugins",
					nil,
					[]apiserver.AdmissionPluginConfig{
						{AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{Name: "Priority"}},
						{AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{Name: "NamespaceLifecycle"}},
						{AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{Name: "LimitRanger"}},
						{AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{Name: "PodSecurityPolicy"}},
						{AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{Name: "ServiceAccount"}},
						{AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{Name: "NodeRestriction"}},
						{AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{Name: "DefaultStorageClass"}},
						{AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{Name: "DefaultTolerationSeconds"}},
						{AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{Name: "ResourceQuota"}},
						{AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{Name: "StorageObjectInUseProtection"}},
						{AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{Name: "MutatingAdmissionWebhook"}},
						{AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{Name: "ValidatingAdmissionWebhook"}},
					},
					false,
				),
				Entry("exclude PodSecurityPolicy from default plugins (workerless)",
					nil,
					[]apiserver.AdmissionPluginConfig{
						{AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{Name: "Priority"}},
						{AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{Name: "NamespaceLifecycle"}},
						{AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{Name: "LimitRanger"}},
						{AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{Name: "ServiceAccount"}},
						{AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{Name: "NodeRestriction"}},
						{AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{Name: "DefaultStorageClass"}},
						{AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{Name: "DefaultTolerationSeconds"}},
						{AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{Name: "ResourceQuota"}},
						{AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{Name: "StorageObjectInUseProtection"}},
						{AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{Name: "MutatingAdmissionWebhook"}},
						{AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{Name: "ValidatingAdmissionWebhook"}},
					},
					true,
				),
				Entry("default plugins with overrides",
					[]gardencorev1beta1.AdmissionPlugin{
						{Name: "NamespaceLifecycle", Config: &runtime.RawExtension{Raw: []byte("namespace-lifecycle-config")}, KubeconfigSecretName: pointer.String("secret-1")},
					},
					[]apiserver.AdmissionPluginConfig{
						{AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{Name: "Priority"}},
						{AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{Name: "NamespaceLifecycle", Config: &runtime.RawExtension{Raw: []byte("namespace-lifecycle-config")}, KubeconfigSecretName: pointer.String("secret-1")}, Kubeconfig: []byte("kubeconfig-data")},
						{AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{Name: "LimitRanger"}},
						{AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{Name: "PodSecurityPolicy"}},
						{AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{Name: "ServiceAccount"}},
						{AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{Name: "NodeRestriction"}},
						{AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{Name: "DefaultStorageClass"}},
						{AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{Name: "DefaultTolerationSeconds"}},
						{AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{Name: "ResourceQuota"}},
						{AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{Name: "StorageObjectInUseProtection"}},
						{AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{Name: "MutatingAdmissionWebhook"}},
						{AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{Name: "ValidatingAdmissionWebhook"}},
					},
					false,
				),
				Entry("default plugins with overrides and other plugins",
					[]gardencorev1beta1.AdmissionPlugin{
						{Name: "NamespaceLifecycle", Config: &runtime.RawExtension{Raw: []byte("namespace-lifecycle-config")}},
						{Name: "Foo"},
						{Name: "Bar"},
						{Name: "Baz", Config: &runtime.RawExtension{Raw: []byte("baz-config")}},
					},
					[]apiserver.AdmissionPluginConfig{
						{AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{Name: "Priority"}},
						{AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{Name: "NamespaceLifecycle", Config: &runtime.RawExtension{Raw: []byte("namespace-lifecycle-config")}}},
						{AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{Name: "LimitRanger"}},
						{AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{Name: "PodSecurityPolicy"}},
						{AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{Name: "ServiceAccount"}},
						{AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{Name: "NodeRestriction"}},
						{AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{Name: "DefaultStorageClass"}},
						{AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{Name: "DefaultTolerationSeconds"}},
						{AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{Name: "ResourceQuota"}},
						{AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{Name: "StorageObjectInUseProtection"}},
						{AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{Name: "MutatingAdmissionWebhook"}},
						{AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{Name: "ValidatingAdmissionWebhook"}},
						{AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{Name: "Foo"}},
						{AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{Name: "Bar"}},
						{AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{Name: "Baz", Config: &runtime.RawExtension{Raw: []byte("baz-config")}}},
					},
					false,
				),
				Entry("default plugins with overrides and skipping configured plugins if disabled",
					[]gardencorev1beta1.AdmissionPlugin{
						{Name: "NamespaceLifecycle", Config: &runtime.RawExtension{Raw: []byte("namespace-lifecycle-config")}},
						{Name: "Foo"},
						{Name: "Bar", Disabled: pointer.Bool(true)},
						{Name: "Baz", Config: &runtime.RawExtension{Raw: []byte("baz-config")}, Disabled: pointer.Bool(true)},
					},
					[]apiserver.AdmissionPluginConfig{
						{AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{Name: "Priority"}},
						{AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{Name: "NamespaceLifecycle", Config: &runtime.RawExtension{Raw: []byte("namespace-lifecycle-config")}}},
						{AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{Name: "LimitRanger"}},
						{AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{Name: "PodSecurityPolicy"}},
						{AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{Name: "ServiceAccount"}},
						{AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{Name: "NodeRestriction"}},
						{AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{Name: "DefaultStorageClass"}},
						{AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{Name: "DefaultTolerationSeconds"}},
						{AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{Name: "ResourceQuota"}},
						{AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{Name: "StorageObjectInUseProtection"}},
						{AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{Name: "MutatingAdmissionWebhook"}},
						{AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{Name: "ValidatingAdmissionWebhook"}},
						{AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{Name: "Foo"}},
					},
					false,
				),
				Entry("default plugins with overrides and skipping default plugins if disabled",
					[]gardencorev1beta1.AdmissionPlugin{
						{Name: "NamespaceLifecycle", Config: &runtime.RawExtension{Raw: []byte("namespace-lifecycle-config")}},
						{Name: "PodSecurityPolicy", Disabled: pointer.Bool(true)},
						{Name: "ResourceQuota", Disabled: pointer.Bool(true)},
						{Name: "Foo"},
						{Name: "Bar"},
						{Name: "Baz", Config: &runtime.RawExtension{Raw: []byte("baz-config")}},
						{Name: "ServiceAccount", Disabled: pointer.Bool(false)},
					},
					[]apiserver.AdmissionPluginConfig{
						{AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{Name: "Priority"}},
						{AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{Name: "NamespaceLifecycle", Config: &runtime.RawExtension{Raw: []byte("namespace-lifecycle-config")}}},
						{AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{Name: "LimitRanger"}},
						{AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{Name: "ServiceAccount", Disabled: pointer.Bool(false)}},
						{AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{Name: "NodeRestriction"}},
						{AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{Name: "DefaultStorageClass"}},
						{AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{Name: "DefaultTolerationSeconds"}},
						{AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{Name: "StorageObjectInUseProtection"}},
						{AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{Name: "MutatingAdmissionWebhook"}},
						{AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{Name: "ValidatingAdmissionWebhook"}},
						{AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{Name: "Foo"}},
						{AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{Name: "Bar"}},
						{AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{Name: "Baz", Config: &runtime.RawExtension{Raw: []byte("baz-config")}}},
					},
					false,
				),
				Entry("default plugins with overrides and skipping disabled plugins for Workerless even if enabled in the config",
					[]gardencorev1beta1.AdmissionPlugin{
						{Name: "NamespaceLifecycle", Config: &runtime.RawExtension{Raw: []byte("namespace-lifecycle-config")}},
						{Name: "PodSecurityPolicy", Disabled: pointer.Bool(true)},
						{Name: "ResourceQuota", Disabled: pointer.Bool(true)},
						{Name: "Foo"},
						{Name: "Bar"},
						{Name: "Baz", Config: &runtime.RawExtension{Raw: []byte("baz-config")}},
						{Name: "ServiceAccount", Disabled: pointer.Bool(false)},
					},
					[]apiserver.AdmissionPluginConfig{
						{AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{Name: "Priority"}},
						{AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{Name: "NamespaceLifecycle", Config: &runtime.RawExtension{Raw: []byte("namespace-lifecycle-config")}}},
						{AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{Name: "LimitRanger"}},
						{AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{Name: "ServiceAccount", Disabled: pointer.Bool(false)}},
						{AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{Name: "NodeRestriction"}},
						{AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{Name: "DefaultStorageClass"}},
						{AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{Name: "DefaultTolerationSeconds"}},
						{AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{Name: "StorageObjectInUseProtection"}},
						{AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{Name: "MutatingAdmissionWebhook"}},
						{AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{Name: "ValidatingAdmissionWebhook"}},
						{AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{Name: "Foo"}},
						{AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{Name: "Bar"}},
						{AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{Name: "Baz", Config: &runtime.RawExtension{Raw: []byte("baz-config")}}},
					},
					true,
				),
			)

			Context("should have the expected disabled admission plugins", func() {
				var expectedDisabledPlugins []gardencorev1beta1.AdmissionPlugin

				AfterEach(func() {
					kubeAPIServer, err := NewKubeAPIServer(ctx, runtimeClientSet, resourceConfigClient, namespace, objectMeta, runtimeVersion, targetVersion, imageVector, sm, namePrefix, apiServerConfig, autoscalingConfig, serviceNetworkCIDR, vpnConfig, priorityClassName, isWorkerless, staticTokenKubeconfigEnabled, auditWebhookConfig, authenticationWebhookConfig, authorizationWebhookConfig, resourcesToStoreInETCDEvents)
					Expect(err).NotTo(HaveOccurred())
					Expect(kubeAPIServer.GetValues().DisabledAdmissionPlugins).To(Equal(expectedDisabledPlugins))
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

			Describe("PodSecurity Admission Plugin", func() {
				var (
					configData    *runtime.RawExtension
					err           error
					kubeAPIServer kubeapiserver.Interface
				)

				JustBeforeEach(func() {
					configData = nil
					kubeAPIServer, err = NewKubeAPIServer(ctx, runtimeClientSet, resourceConfigClient, namespace, objectMeta, runtimeVersion, targetVersion, imageVector, sm, namePrefix, apiServerConfig, autoscalingConfig, serviceNetworkCIDR, vpnConfig, priorityClassName, isWorkerless, staticTokenKubeconfigEnabled, auditWebhookConfig, authenticationWebhookConfig, authorizationWebhookConfig, resourcesToStoreInETCDEvents)
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
  warn-version: "v1.23"
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
								"WarnVersion": Equal("v1.23"),
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
  warn-version: "v1.23"
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
								"WarnVersion": Equal("v1.23"),
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
  warn-version: "v1.22"
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
								"WarnVersion": Equal("v1.22"),
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

					kubeAPIServer, err := NewKubeAPIServer(ctx, runtimeClientSet, resourceConfigClient, namespace, objectMeta, runtimeVersion, targetVersion, imageVector, sm, namePrefix, apiServerConfig, autoscalingConfig, serviceNetworkCIDR, vpnConfig, priorityClassName, isWorkerless, staticTokenKubeconfigEnabled, auditWebhookConfig, authenticationWebhookConfig, authorizationWebhookConfig, resourcesToStoreInETCDEvents)
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

		Describe("DefaultNotReadyTolerationSeconds and DefaultUnreachableTolerationSeconds", func() {
			It("should not set the fields", func() {
				kubeAPIServer, err := NewKubeAPIServer(ctx, runtimeClientSet, resourceConfigClient, namespace, objectMeta, runtimeVersion, targetVersion, imageVector, sm, namePrefix, apiServerConfig, autoscalingConfig, serviceNetworkCIDR, vpnConfig, priorityClassName, isWorkerless, staticTokenKubeconfigEnabled, auditWebhookConfig, authenticationWebhookConfig, authorizationWebhookConfig, resourcesToStoreInETCDEvents)
				Expect(err).NotTo(HaveOccurred())
				Expect(kubeAPIServer.GetValues().DefaultNotReadyTolerationSeconds).To(BeNil())
				Expect(kubeAPIServer.GetValues().DefaultUnreachableTolerationSeconds).To(BeNil())
			})

			It("should set the fields to the configured values", func() {
				apiServerConfig = &gardencorev1beta1.KubeAPIServerConfig{
					DefaultNotReadyTolerationSeconds:    pointer.Int64(120),
					DefaultUnreachableTolerationSeconds: pointer.Int64(130),
				}

				kubeAPIServer, err := NewKubeAPIServer(ctx, runtimeClientSet, resourceConfigClient, namespace, objectMeta, runtimeVersion, targetVersion, imageVector, sm, namePrefix, apiServerConfig, autoscalingConfig, serviceNetworkCIDR, vpnConfig, priorityClassName, isWorkerless, staticTokenKubeconfigEnabled, auditWebhookConfig, authenticationWebhookConfig, authorizationWebhookConfig, resourcesToStoreInETCDEvents)
				Expect(err).NotTo(HaveOccurred())
				Expect(kubeAPIServer.GetValues().DefaultNotReadyTolerationSeconds).To(PointTo(Equal(int64(120))))
				Expect(kubeAPIServer.GetValues().DefaultUnreachableTolerationSeconds).To(PointTo(Equal(int64(130))))
			})
		})

		Describe("EventTTL", func() {
			It("should not set the event ttl field", func() {
				kubeAPIServer, err := NewKubeAPIServer(ctx, runtimeClientSet, resourceConfigClient, namespace, objectMeta, runtimeVersion, targetVersion, imageVector, sm, namePrefix, apiServerConfig, autoscalingConfig, serviceNetworkCIDR, vpnConfig, priorityClassName, isWorkerless, staticTokenKubeconfigEnabled, auditWebhookConfig, authenticationWebhookConfig, authorizationWebhookConfig, resourcesToStoreInETCDEvents)
				Expect(err).NotTo(HaveOccurred())
				Expect(kubeAPIServer.GetValues().EventTTL).To(BeNil())
			})

			It("should set the field to the configured values", func() {
				eventTTL := &metav1.Duration{Duration: 2 * time.Hour}

				apiServerConfig = &gardencorev1beta1.KubeAPIServerConfig{
					EventTTL: eventTTL,
				}

				kubeAPIServer, err := NewKubeAPIServer(ctx, runtimeClientSet, resourceConfigClient, namespace, objectMeta, runtimeVersion, targetVersion, imageVector, sm, namePrefix, apiServerConfig, autoscalingConfig, serviceNetworkCIDR, vpnConfig, priorityClassName, isWorkerless, staticTokenKubeconfigEnabled, auditWebhookConfig, authenticationWebhookConfig, authorizationWebhookConfig, resourcesToStoreInETCDEvents)
				Expect(err).NotTo(HaveOccurred())
				Expect(kubeAPIServer.GetValues().EventTTL).To(Equal(eventTTL))
			})
		})

		Describe("FeatureGates", func() {
			It("should set the field to nil by default", func() {
				kubeAPIServer, err := NewKubeAPIServer(ctx, runtimeClientSet, resourceConfigClient, namespace, objectMeta, runtimeVersion, targetVersion, imageVector, sm, namePrefix, apiServerConfig, autoscalingConfig, serviceNetworkCIDR, vpnConfig, priorityClassName, isWorkerless, staticTokenKubeconfigEnabled, auditWebhookConfig, authenticationWebhookConfig, authorizationWebhookConfig, resourcesToStoreInETCDEvents)
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

				kubeAPIServer, err := NewKubeAPIServer(ctx, runtimeClientSet, resourceConfigClient, namespace, objectMeta, runtimeVersion, targetVersion, imageVector, sm, namePrefix, apiServerConfig, autoscalingConfig, serviceNetworkCIDR, vpnConfig, priorityClassName, isWorkerless, staticTokenKubeconfigEnabled, auditWebhookConfig, authenticationWebhookConfig, authorizationWebhookConfig, resourcesToStoreInETCDEvents)
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

					kubeAPIServer, err := NewKubeAPIServer(ctx, runtimeClientSet, resourceConfigClient, namespace, objectMeta, runtimeVersion, targetVersion, imageVector, sm, namePrefix, apiServerConfig, autoscalingConfig, serviceNetworkCIDR, vpnConfig, priorityClassName, isWorkerless, staticTokenKubeconfigEnabled, auditWebhookConfig, authenticationWebhookConfig, authorizationWebhookConfig, resourcesToStoreInETCDEvents)
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
				kubeAPIServer, err := NewKubeAPIServer(ctx, runtimeClientSet, resourceConfigClient, namespace, objectMeta, runtimeVersion, targetVersion, imageVector, sm, namePrefix, apiServerConfig, autoscalingConfig, serviceNetworkCIDR, vpnConfig, priorityClassName, isWorkerless, staticTokenKubeconfigEnabled, auditWebhookConfig, authenticationWebhookConfig, authorizationWebhookConfig, resourcesToStoreInETCDEvents)
				Expect(err).NotTo(HaveOccurred())
				Expect(kubeAPIServer.GetValues().Requests).To(BeNil())
			})

			It("should set the field to the configured values", func() {
				requests := &gardencorev1beta1.KubeAPIServerRequests{
					MaxMutatingInflight:    pointer.Int32(1),
					MaxNonMutatingInflight: pointer.Int32(2),
				}
				apiServerConfig = &gardencorev1beta1.KubeAPIServerConfig{Requests: requests}

				kubeAPIServer, err := NewKubeAPIServer(ctx, runtimeClientSet, resourceConfigClient, namespace, objectMeta, runtimeVersion, targetVersion, imageVector, sm, namePrefix, apiServerConfig, autoscalingConfig, serviceNetworkCIDR, vpnConfig, priorityClassName, isWorkerless, staticTokenKubeconfigEnabled, auditWebhookConfig, authenticationWebhookConfig, authorizationWebhookConfig, resourcesToStoreInETCDEvents)
				Expect(err).NotTo(HaveOccurred())
				Expect(kubeAPIServer.GetValues().Requests).To(Equal(requests))
			})
		})

		Describe("RuntimeConfig", func() {
			It("should set the field to nil by default", func() {
				kubeAPIServer, err := NewKubeAPIServer(ctx, runtimeClientSet, resourceConfigClient, namespace, objectMeta, runtimeVersion, targetVersion, imageVector, sm, namePrefix, apiServerConfig, autoscalingConfig, serviceNetworkCIDR, vpnConfig, priorityClassName, isWorkerless, staticTokenKubeconfigEnabled, auditWebhookConfig, authenticationWebhookConfig, authorizationWebhookConfig, resourcesToStoreInETCDEvents)
				Expect(err).NotTo(HaveOccurred())
				Expect(kubeAPIServer.GetValues().RuntimeConfig).To(BeNil())
			})

			It("should set the field to the configured values", func() {
				runtimeConfig := map[string]bool{"foo": true, "bar": false}
				apiServerConfig = &gardencorev1beta1.KubeAPIServerConfig{RuntimeConfig: runtimeConfig}

				kubeAPIServer, err := NewKubeAPIServer(ctx, runtimeClientSet, resourceConfigClient, namespace, objectMeta, runtimeVersion, targetVersion, imageVector, sm, namePrefix, apiServerConfig, autoscalingConfig, serviceNetworkCIDR, vpnConfig, priorityClassName, isWorkerless, staticTokenKubeconfigEnabled, auditWebhookConfig, authenticationWebhookConfig, authorizationWebhookConfig, resourcesToStoreInETCDEvents)
				Expect(err).NotTo(HaveOccurred())
				Expect(kubeAPIServer.GetValues().RuntimeConfig).To(Equal(runtimeConfig))
			})
		})

		Describe("VPNConfig", func() {
			It("should set the field to the configured values", func() {
				vpnConfig = kubeapiserver.VPNConfig{Enabled: true}

				kubeAPIServer, err := NewKubeAPIServer(ctx, runtimeClientSet, resourceConfigClient, namespace, objectMeta, runtimeVersion, targetVersion, imageVector, sm, namePrefix, apiServerConfig, autoscalingConfig, serviceNetworkCIDR, vpnConfig, priorityClassName, isWorkerless, staticTokenKubeconfigEnabled, auditWebhookConfig, authenticationWebhookConfig, authorizationWebhookConfig, resourcesToStoreInETCDEvents)
				Expect(err).NotTo(HaveOccurred())
				Expect(kubeAPIServer.GetValues().VPN).To(Equal(vpnConfig))
			})
		})

		Describe("WatchCacheSizes", func() {
			It("should set the field to nil by default", func() {
				kubeAPIServer, err := NewKubeAPIServer(ctx, runtimeClientSet, resourceConfigClient, namespace, objectMeta, runtimeVersion, targetVersion, imageVector, sm, namePrefix, apiServerConfig, autoscalingConfig, serviceNetworkCIDR, vpnConfig, priorityClassName, isWorkerless, staticTokenKubeconfigEnabled, auditWebhookConfig, authenticationWebhookConfig, authorizationWebhookConfig, resourcesToStoreInETCDEvents)
				Expect(err).NotTo(HaveOccurred())
				Expect(kubeAPIServer.GetValues().WatchCacheSizes).To(BeNil())
			})

			It("should set the field to the configured values", func() {
				watchCacheSizes := &gardencorev1beta1.WatchCacheSizes{
					Default:   pointer.Int32(1),
					Resources: []gardencorev1beta1.ResourceWatchCacheSize{{Resource: "foo"}},
				}
				apiServerConfig = &gardencorev1beta1.KubeAPIServerConfig{WatchCacheSizes: watchCacheSizes}

				kubeAPIServer, err := NewKubeAPIServer(ctx, runtimeClientSet, resourceConfigClient, namespace, objectMeta, runtimeVersion, targetVersion, imageVector, sm, namePrefix, apiServerConfig, autoscalingConfig, serviceNetworkCIDR, vpnConfig, priorityClassName, isWorkerless, staticTokenKubeconfigEnabled, auditWebhookConfig, authenticationWebhookConfig, authorizationWebhookConfig, resourcesToStoreInETCDEvents)
				Expect(err).NotTo(HaveOccurred())
				Expect(kubeAPIServer.GetValues().WatchCacheSizes).To(Equal(watchCacheSizes))
			})
		})

		Describe("PriorityClassName", func() {
			It("should set the field properly", func() {
				kubeAPIServer, err := NewKubeAPIServer(ctx, runtimeClientSet, resourceConfigClient, namespace, objectMeta, runtimeVersion, targetVersion, imageVector, sm, namePrefix, apiServerConfig, autoscalingConfig, serviceNetworkCIDR, vpnConfig, priorityClassName, isWorkerless, staticTokenKubeconfigEnabled, auditWebhookConfig, authenticationWebhookConfig, authorizationWebhookConfig, resourcesToStoreInETCDEvents)
				Expect(err).NotTo(HaveOccurred())
				Expect(kubeAPIServer.GetValues().PriorityClassName).To(Equal(priorityClassName))
			})
		})

		Describe("IsWorkerless", func() {
			It("should set the field properly", func() {
				kubeAPIServer, err := NewKubeAPIServer(ctx, runtimeClientSet, resourceConfigClient, namespace, objectMeta, runtimeVersion, targetVersion, imageVector, sm, namePrefix, apiServerConfig, autoscalingConfig, serviceNetworkCIDR, vpnConfig, priorityClassName, isWorkerless, staticTokenKubeconfigEnabled, auditWebhookConfig, authenticationWebhookConfig, authorizationWebhookConfig, resourcesToStoreInETCDEvents)
				Expect(err).NotTo(HaveOccurred())
				Expect(kubeAPIServer.GetValues().IsWorkerless).To(Equal(isWorkerless))
			})
		})

		Describe("Authentication", func() {
			It("should set the field properly", func() {
				kubeAPIServer, err := NewKubeAPIServer(ctx, runtimeClientSet, resourceConfigClient, namespace, objectMeta, runtimeVersion, targetVersion, imageVector, sm, namePrefix, apiServerConfig, autoscalingConfig, serviceNetworkCIDR, vpnConfig, priorityClassName, isWorkerless, staticTokenKubeconfigEnabled, auditWebhookConfig, authenticationWebhookConfig, authorizationWebhookConfig, resourcesToStoreInETCDEvents)
				Expect(err).NotTo(HaveOccurred())
				Expect(kubeAPIServer.GetValues().AuthenticationWebhook).To(Equal(authenticationWebhookConfig))
			})
		})

		Describe("Authorization", func() {
			It("should set the field properly", func() {
				kubeAPIServer, err := NewKubeAPIServer(ctx, runtimeClientSet, resourceConfigClient, namespace, objectMeta, runtimeVersion, targetVersion, imageVector, sm, namePrefix, apiServerConfig, autoscalingConfig, serviceNetworkCIDR, vpnConfig, priorityClassName, isWorkerless, staticTokenKubeconfigEnabled, auditWebhookConfig, authenticationWebhookConfig, authorizationWebhookConfig, resourcesToStoreInETCDEvents)
				Expect(err).NotTo(HaveOccurred())
				Expect(kubeAPIServer.GetValues().AuthorizationWebhook).To(Equal(authorizationWebhookConfig))
			})
		})

		Describe("ResourcesToStoreInETCDEvents", func() {
			It("should set the field properly", func() {
				kubeAPIServer, err := NewKubeAPIServer(ctx, runtimeClientSet, resourceConfigClient, namespace, objectMeta, runtimeVersion, targetVersion, imageVector, sm, namePrefix, apiServerConfig, autoscalingConfig, serviceNetworkCIDR, vpnConfig, priorityClassName, isWorkerless, staticTokenKubeconfigEnabled, auditWebhookConfig, authenticationWebhookConfig, authorizationWebhookConfig, resourcesToStoreInETCDEvents)
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
			externalServer                 string
			etcdEncryptionKeyRotationPhase gardencorev1beta1.CredentialsRotationPhase
			serviceAccountKeyRotationPhase gardencorev1beta1.CredentialsRotationPhase
			wantScaleDown                  bool
		)

		BeforeEach(func() {
			ctrl = gomock.NewController(GinkgoT())
			DeferCleanup(func() { ctrl.Finish() })

			kubeAPIServer = mockkubeapiserver.NewMockInterface(ctrl)

			serverCertificateConfig = kubeapiserver.ServerCertificateConfig{ExtraDNSNames: []string{"foo"}}
			sniConfig = kubeapiserver.SNIConfig{Enabled: true}
			externalHostname = "external-hostname"
			externalServer = "external-server"
			etcdEncryptionKeyRotationPhase = ""
			serviceAccountKeyRotationPhase = ""
			wantScaleDown = false
		})

		var apiServerResources = corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("1"),
				corev1.ResourceMemory: resource.MustParse("2"),
			},
		}

		DescribeTable("should correctly set the autoscaling apiserver resources",
			func(prepTest func(), autoscalingConfig apiserver.AutoscalingConfig, expectedResources *corev1.ResourceRequirements) {
				if prepTest != nil {
					prepTest()
				}

				kubeAPIServer.EXPECT().GetValues().Return(kubeapiserver.Values{
					Values: apiserver.Values{
						Autoscaling: autoscalingConfig,
					}},
				)
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

				Expect(DeployKubeAPIServer(ctx, runtimeClient, namespace, kubeAPIServer, apiServerConfig, serverCertificateConfig, sniConfig, externalHostname, externalServer, etcdEncryptionKeyRotationPhase, serviceAccountKeyRotationPhase, wantScaleDown)).To(Succeed())
			},

			Entry("nothing is set because deployment is not found",
				nil,
				apiserver.AutoscalingConfig{},
				nil,
			),
			Entry("nothing is set because HVPA is disabled",
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
				apiserver.AutoscalingConfig{HVPAEnabled: false},
				nil,
			),
			Entry("set the existing requirements because deployment found and HVPA enabled",
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
				apiserver.AutoscalingConfig{HVPAEnabled: true},
				&apiServerResources,
			),
		)

		DescribeTable("should correctly set the autoscaling replicas",
			func(prepTest func(), autoscalingConfig apiserver.AutoscalingConfig, expectedReplicas int32) {
				if prepTest != nil {
					prepTest()
				}

				kubeAPIServer.EXPECT().GetValues().Return(kubeapiserver.Values{
					Values: apiserver.Values{
						Autoscaling: autoscalingConfig,
					},
				})
				kubeAPIServer.EXPECT().SetAutoscalingReplicas(&expectedReplicas)
				kubeAPIServer.EXPECT().SetSNIConfig(gomock.Any())
				kubeAPIServer.EXPECT().SetETCDEncryptionConfig(gomock.Any())
				kubeAPIServer.EXPECT().SetExternalHostname(gomock.Any())
				kubeAPIServer.EXPECT().SetExternalServer(gomock.Any())
				kubeAPIServer.EXPECT().SetServerCertificateConfig(gomock.Any())
				kubeAPIServer.EXPECT().SetServiceAccountConfig(gomock.Any())
				kubeAPIServer.EXPECT().Deploy(ctx)

				Expect(DeployKubeAPIServer(ctx, runtimeClient, namespace, kubeAPIServer, apiServerConfig, serverCertificateConfig, sniConfig, externalHostname, externalServer, etcdEncryptionKeyRotationPhase, serviceAccountKeyRotationPhase, wantScaleDown)).To(Succeed())
			},

			Entry("no change due to already set",
				nil,
				apiserver.AutoscalingConfig{Replicas: pointer.Int32(1)},
				int32(1),
			),
			Entry("use minReplicas because deployment does not exist",
				nil,
				apiserver.AutoscalingConfig{MinReplicas: 2},
				int32(2),
			),
			Entry("use 0 because shoot is hibernated, even  if deployment does not exist",
				func() {
					wantScaleDown = true
				},
				apiserver.AutoscalingConfig{MinReplicas: 2},
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
							Replicas: pointer.Int32(3),
						},
					})).To(Succeed())
				},
				apiserver.AutoscalingConfig{},
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
							Replicas: pointer.Int32(0),
						},
					})).To(Succeed())
				},
				apiserver.AutoscalingConfig{},
				int32(0),
			),
		)

		DescribeTable("ETCDEncryptionConfig",
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
				kubeAPIServer.EXPECT().SetExternalServer(gomock.Any())
				kubeAPIServer.EXPECT().SetServerCertificateConfig(gomock.Any())
				kubeAPIServer.EXPECT().SetServiceAccountConfig(gomock.Any())
				kubeAPIServer.EXPECT().Deploy(ctx)

				Expect(DeployKubeAPIServer(ctx, runtimeClient, namespace, kubeAPIServer, apiServerConfig, serverCertificateConfig, sniConfig, externalHostname, externalServer, etcdEncryptionKeyRotationPhase, serviceAccountKeyRotationPhase, wantScaleDown)).To(Succeed())

				if finalizeTest != nil {
					finalizeTest()
				}
			},

			Entry("no rotation",
				gardencorev1beta1.CredentialsRotationPhase(""),
				nil,
				apiserver.ETCDEncryptionConfig{EncryptWithCurrentKey: true, Resources: []string{"secrets"}},
				nil,
			),
			Entry("preparing phase, new key already populated",
				gardencorev1beta1.RotationPreparing,
				func() {
					Expect(runtimeClient.Create(ctx, &appsv1.Deployment{
						ObjectMeta: metav1.ObjectMeta{
							Name:        "kube-apiserver",
							Namespace:   namespace,
							Annotations: map[string]string{"credentials.gardener.cloud/new-encryption-key-populated": "true"},
						},
					})).To(Succeed())
				},
				apiserver.ETCDEncryptionConfig{RotationPhase: gardencorev1beta1.RotationPreparing, EncryptWithCurrentKey: true, Resources: []string{"secrets"}},
				nil,
			),
			Entry("preparing phase, new key not yet populated",
				gardencorev1beta1.RotationPreparing,
				func() {
					Expect(runtimeClient.Create(ctx, &appsv1.Deployment{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "kube-apiserver",
							Namespace: namespace,
						},
					})).To(Succeed())

					kubeAPIServer.EXPECT().Wait(ctx)

					kubeAPIServer.EXPECT().SetETCDEncryptionConfig(apiserver.ETCDEncryptionConfig{
						RotationPhase:         gardencorev1beta1.RotationPreparing,
						EncryptWithCurrentKey: true,
						Resources:             []string{"secrets"},
					})
					kubeAPIServer.EXPECT().Deploy(ctx)
				},
				apiserver.ETCDEncryptionConfig{RotationPhase: gardencorev1beta1.RotationPreparing, EncryptWithCurrentKey: false, Resources: []string{"secrets"}},
				func() {
					deployment := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "kube-apiserver", Namespace: namespace}}
					Expect(runtimeClient.Get(ctx, client.ObjectKeyFromObject(deployment), deployment)).To(Succeed())
					Expect(deployment.Annotations).To(HaveKeyWithValue("credentials.gardener.cloud/new-encryption-key-populated", "true"))
				},
			),
			Entry("prepared phase",
				gardencorev1beta1.RotationPrepared,
				nil,
				apiserver.ETCDEncryptionConfig{RotationPhase: gardencorev1beta1.RotationPrepared, EncryptWithCurrentKey: true, Resources: []string{"secrets"}},
				nil,
			),
			Entry("completing phase",
				gardencorev1beta1.RotationCompleting,
				func() {
					Expect(runtimeClient.Create(ctx, &appsv1.Deployment{
						ObjectMeta: metav1.ObjectMeta{
							Name:        "kube-apiserver",
							Namespace:   namespace,
							Annotations: map[string]string{"credentials.gardener.cloud/new-encryption-key-populated": "true"},
						},
					})).To(Succeed())
				},
				apiserver.ETCDEncryptionConfig{RotationPhase: gardencorev1beta1.RotationCompleting, EncryptWithCurrentKey: true, Resources: []string{"secrets"}},
				func() {
					deployment := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "kube-apiserver", Namespace: namespace}}
					Expect(runtimeClient.Get(ctx, client.ObjectKeyFromObject(deployment), deployment)).To(Succeed())
					Expect(deployment.Annotations).NotTo(HaveKey("credentials.gardener.cloud/new-encryption-key-populated"))
				},
			),
			Entry("completed phase",
				gardencorev1beta1.RotationCompleted,
				nil,
				apiserver.ETCDEncryptionConfig{RotationPhase: gardencorev1beta1.RotationCompleted, EncryptWithCurrentKey: true, Resources: []string{"secrets"}},
				nil,
			),
		)

		Describe("External{Hostname,Server}", func() {
			It("should set the external {hostname,server} to the provided addresses", func() {
				kubeAPIServer.EXPECT().GetValues()
				kubeAPIServer.EXPECT().SetAutoscalingReplicas(gomock.Any())
				kubeAPIServer.EXPECT().SetSNIConfig(gomock.Any())
				kubeAPIServer.EXPECT().SetETCDEncryptionConfig(gomock.Any())
				kubeAPIServer.EXPECT().SetExternalHostname(externalHostname)
				kubeAPIServer.EXPECT().SetExternalServer(externalServer)
				kubeAPIServer.EXPECT().SetServerCertificateConfig(gomock.Any())
				kubeAPIServer.EXPECT().SetServiceAccountConfig(gomock.Any())
				kubeAPIServer.EXPECT().Deploy(ctx)

				Expect(DeployKubeAPIServer(ctx, runtimeClient, namespace, kubeAPIServer, apiServerConfig, serverCertificateConfig, sniConfig, externalHostname, externalServer, etcdEncryptionKeyRotationPhase, serviceAccountKeyRotationPhase, wantScaleDown)).To(Succeed())
			})
		})

		Describe("ServerCertificateConfig", func() {
			It("should set the field to the provided config", func() {
				kubeAPIServer.EXPECT().GetValues()
				kubeAPIServer.EXPECT().SetAutoscalingReplicas(gomock.Any())
				kubeAPIServer.EXPECT().SetSNIConfig(gomock.Any())
				kubeAPIServer.EXPECT().SetETCDEncryptionConfig(gomock.Any())
				kubeAPIServer.EXPECT().SetExternalHostname(gomock.Any())
				kubeAPIServer.EXPECT().SetExternalServer(gomock.Any())
				kubeAPIServer.EXPECT().SetServerCertificateConfig(serverCertificateConfig)
				kubeAPIServer.EXPECT().SetServiceAccountConfig(gomock.Any())
				kubeAPIServer.EXPECT().Deploy(ctx)

				Expect(DeployKubeAPIServer(ctx, runtimeClient, namespace, kubeAPIServer, apiServerConfig, serverCertificateConfig, sniConfig, externalHostname, externalServer, etcdEncryptionKeyRotationPhase, serviceAccountKeyRotationPhase, wantScaleDown)).To(Succeed())
			})
		})

		Describe("ServiceAccountConfig", func() {
			var (
				maxTokenExpiration    = metav1.Duration{Duration: time.Hour}
				extendTokenExpiration = false
				externalHostname      = "api.my-domain.com"
			)

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

					Expect(DeployKubeAPIServer(ctx, runtimeClient, namespace, kubeAPIServer, apiServerConfig, serverCertificateConfig, sniConfig, externalHostname, externalServer, etcdEncryptionKeyRotationPhase, serviceAccountKeyRotationPhase, wantScaleDown)).To(Succeed())
				},

				Entry("KubeAPIServerConfig is nil",
					nil,
					kubeapiserver.ServiceAccountConfig{Issuer: "https://" + externalHostname},
					false,
					Not(HaveOccurred()),
				),
				Entry("ServiceAccountConfig is nil",
					func() {
						apiServerConfig = &gardencorev1beta1.KubeAPIServerConfig{}
					},
					kubeapiserver.ServiceAccountConfig{Issuer: "https://" + externalHostname},
					false,
					Not(HaveOccurred()),
				),
				Entry("service account key rotation phase is set",
					func() {
						apiServerConfig = &gardencorev1beta1.KubeAPIServerConfig{}
						serviceAccountKeyRotationPhase = gardencorev1beta1.RotationCompleting
					},
					kubeapiserver.ServiceAccountConfig{
						Issuer:        "https://" + externalHostname,
						RotationPhase: gardencorev1beta1.RotationCompleting,
					},
					false,
					Not(HaveOccurred()),
				),
				Entry("Issuer is not provided",
					func() {
						apiServerConfig = &gardencorev1beta1.KubeAPIServerConfig{
							ServiceAccountConfig: &gardencorev1beta1.ServiceAccountConfig{
								ExtendTokenExpiration: &extendTokenExpiration,
								MaxTokenExpiration:    &maxTokenExpiration,
							},
						}
					},
					kubeapiserver.ServiceAccountConfig{
						Issuer:                "https://" + externalHostname,
						ExtendTokenExpiration: &extendTokenExpiration,
						MaxTokenExpiration:    &maxTokenExpiration,
					},
					false,
					Not(HaveOccurred()),
				),
				Entry("Issuer is provided",
					func() {
						apiServerConfig = &gardencorev1beta1.KubeAPIServerConfig{
							ServiceAccountConfig: &gardencorev1beta1.ServiceAccountConfig{
								Issuer: pointer.String("issuer"),
							},
						}
					},
					kubeapiserver.ServiceAccountConfig{
						Issuer:          "issuer",
						AcceptedIssuers: []string{"https://" + externalHostname},
					},
					false,
					Not(HaveOccurred()),
				),
				Entry("AcceptedIssuers is provided and Issuer is not",
					func() {
						apiServerConfig = &gardencorev1beta1.KubeAPIServerConfig{
							ServiceAccountConfig: &gardencorev1beta1.ServiceAccountConfig{
								AcceptedIssuers: []string{"issuer1", "issuer2"},
							},
						}
					},
					kubeapiserver.ServiceAccountConfig{
						Issuer:          "https://" + externalHostname,
						AcceptedIssuers: []string{"issuer1", "issuer2"},
					},
					false,
					Not(HaveOccurred()),
				),
				Entry("AcceptedIssuers and Issuer are provided",
					func() {
						apiServerConfig = &gardencorev1beta1.KubeAPIServerConfig{
							ServiceAccountConfig: &gardencorev1beta1.ServiceAccountConfig{
								Issuer:          pointer.String("issuer"),
								AcceptedIssuers: []string{"issuer1", "issuer2"},
							},
						}
					},
					kubeapiserver.ServiceAccountConfig{
						Issuer:          "issuer",
						AcceptedIssuers: []string{"issuer1", "issuer2", "https://" + externalHostname},
					},
					false,
					Not(HaveOccurred()),
				),
				Entry("Default Issuer is already part of AcceptedIssuers",
					func() {
						apiServerConfig = &gardencorev1beta1.KubeAPIServerConfig{
							ServiceAccountConfig: &gardencorev1beta1.ServiceAccountConfig{
								Issuer:          pointer.String("issuer"),
								AcceptedIssuers: []string{"https://" + externalHostname},
							},
						}
					},
					kubeapiserver.ServiceAccountConfig{
						Issuer:          "issuer",
						AcceptedIssuers: []string{"https://" + externalHostname},
					},
					false,
					Not(HaveOccurred()),
				),
				Entry("AcceptedIssuers is not provided",
					func() {
						apiServerConfig = &gardencorev1beta1.KubeAPIServerConfig{
							ServiceAccountConfig: &gardencorev1beta1.ServiceAccountConfig{},
						}
					},
					kubeapiserver.ServiceAccountConfig{Issuer: "https://" + externalHostname},
					false,
					Not(HaveOccurred()),
				),
			)
		})

		Describe("SNIConfig", func() {
			It("should set the field to the provided config", func() {
				kubeAPIServer.EXPECT().GetValues()
				kubeAPIServer.EXPECT().SetAutoscalingReplicas(gomock.Any())
				kubeAPIServer.EXPECT().SetSNIConfig(sniConfig)
				kubeAPIServer.EXPECT().SetETCDEncryptionConfig(gomock.Any())
				kubeAPIServer.EXPECT().SetExternalHostname(gomock.Any())
				kubeAPIServer.EXPECT().SetExternalServer(gomock.Any())
				kubeAPIServer.EXPECT().SetServerCertificateConfig(gomock.Any())
				kubeAPIServer.EXPECT().SetServiceAccountConfig(gomock.Any())
				kubeAPIServer.EXPECT().Deploy(ctx)

				Expect(DeployKubeAPIServer(ctx, runtimeClient, namespace, kubeAPIServer, apiServerConfig, serverCertificateConfig, sniConfig, externalHostname, externalServer, etcdEncryptionKeyRotationPhase, serviceAccountKeyRotationPhase, wantScaleDown)).To(Succeed())
			})
		})
	})
})
