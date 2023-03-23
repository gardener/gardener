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
	"fmt"

	"github.com/Masterminds/semver"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	admissionapiv1 "k8s.io/pod-security-admission/admission/api/v1"
	admissionapiv1alpha1 "k8s.io/pod-security-admission/admission/api/v1alpha1"
	admissionapiv1beta1 "k8s.io/pod-security-admission/admission/api/v1beta1"
	"k8s.io/utils/pointer"
	"k8s.io/utils/strings/slices"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/operation/botanist/component/kubeapiserver"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/images"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	"github.com/gardener/gardener/pkg/utils/version"
)

var (
	runtimeScheme *runtime.Scheme
	codec         runtime.Codec
)

func init() {
	runtimeScheme = runtime.NewScheme()
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
}

// NewKubeAPIServer returns a deployer for the kube-apiserver.
func NewKubeAPIServer(
	ctx context.Context,
	runtimeClientSet kubernetes.Interface,
	auditConfigClient client.Client,
	runtimeNamespace string,
	objectMeta metav1.ObjectMeta,
	runtimeVersion *semver.Version,
	targetVersion *semver.Version,
	imageVector imagevector.ImageVector,
	secretsManager secretsmanager.Interface,
	apiServerConfig *gardencorev1beta1.KubeAPIServerConfig,
	autoscalingConfig kubeapiserver.AutoscalingConfig,
	serviceNetworkCIDR string,
	vpnConfig kubeapiserver.VPNConfig,
	staticTokenKubeconfigEnabled *bool,
) (
	kubeapiserver.Interface,
	error,
) {
	images, err := computeKubeAPIServerImages(imageVector, runtimeVersion, targetVersion, vpnConfig)
	if err != nil {
		return nil, err
	}

	var (
		enabledAdmissionPlugins             = kubernetesutils.GetAdmissionPluginsForVersion(targetVersion.String())
		disabledAdmissionPlugins            []gardencorev1beta1.AdmissionPlugin
		apiAudiences                        = []string{"kubernetes", "gardener"}
		auditConfig                         *kubeapiserver.AuditConfig
		defaultNotReadyTolerationSeconds    *int64
		defaultUnreachableTolerationSeconds *int64
		eventTTL                            *metav1.Duration
		featureGates                        map[string]bool
		oidcConfig                          *gardencorev1beta1.OIDCConfig
		requests                            *gardencorev1beta1.KubeAPIServerRequests
		runtimeConfig                       map[string]bool
		watchCacheSizes                     *gardencorev1beta1.WatchCacheSizes
		logging                             *gardencorev1beta1.KubeAPIServerLogging
	)

	if apiServerConfig != nil {
		enabledAdmissionPlugins = computeEnabledKubeAPIServerAdmissionPlugins(enabledAdmissionPlugins, apiServerConfig.AdmissionPlugins)
		disabledAdmissionPlugins = computeDisabledKubeAPIServerAdmissionPlugins(apiServerConfig.AdmissionPlugins)

		enabledAdmissionPlugins, err = ensureAdmissionPluginConfig(enabledAdmissionPlugins)
		if err != nil {
			return nil, err
		}

		if apiServerConfig.APIAudiences != nil {
			apiAudiences = apiServerConfig.APIAudiences
			if !utils.ValueExists(v1beta1constants.GardenerAudience, apiAudiences) {
				apiAudiences = append(apiAudiences, v1beta1constants.GardenerAudience)
			}
		}

		auditConfig, err = computeKubeAPIServerAuditConfig(ctx, auditConfigClient, objectMeta, apiServerConfig.AuditConfig)
		if err != nil {
			return nil, err
		}

		defaultNotReadyTolerationSeconds = apiServerConfig.DefaultNotReadyTolerationSeconds
		defaultUnreachableTolerationSeconds = apiServerConfig.DefaultUnreachableTolerationSeconds
		eventTTL = apiServerConfig.EventTTL
		featureGates = apiServerConfig.FeatureGates
		oidcConfig = apiServerConfig.OIDCConfig
		requests = apiServerConfig.Requests
		runtimeConfig = apiServerConfig.RuntimeConfig

		watchCacheSizes = apiServerConfig.WatchCacheSizes
		logging = apiServerConfig.Logging
	}

	enabledAdmissionPluginConfigs, err := convertToAdmissionPluginConfigs(enabledAdmissionPlugins)
	if err != nil {
		return nil, err
	}

	return kubeapiserver.New(
		runtimeClientSet,
		runtimeNamespace,
		secretsManager,
		kubeapiserver.Values{
			EnabledAdmissionPlugins:             enabledAdmissionPluginConfigs,
			DisabledAdmissionPlugins:            disabledAdmissionPlugins,
			AnonymousAuthenticationEnabled:      v1beta1helper.AnonymousAuthenticationEnabled(apiServerConfig),
			APIAudiences:                        apiAudiences,
			Audit:                               auditConfig,
			Autoscaling:                         autoscalingConfig,
			DefaultNotReadyTolerationSeconds:    defaultNotReadyTolerationSeconds,
			DefaultUnreachableTolerationSeconds: defaultUnreachableTolerationSeconds,
			EventTTL:                            eventTTL,
			FeatureGates:                        featureGates,
			Images:                              images,
			IsNodeless:                          false,
			Logging:                             logging,
			OIDC:                                oidcConfig,
			Requests:                            requests,
			RuntimeConfig:                       runtimeConfig,
			RuntimeVersion:                      runtimeVersion,
			ServiceNetworkCIDR:                  serviceNetworkCIDR,
			StaticTokenKubeconfigEnabled:        staticTokenKubeconfigEnabled,
			Version:                             targetVersion,
			VPN:                                 vpnConfig,
			WatchCacheSizes:                     watchCacheSizes,
		},
	), nil
}

func computeKubeAPIServerImages(
	imageVector imagevector.ImageVector,
	runtimeVersion *semver.Version,
	targetVersion *semver.Version,
	vpnConfig kubeapiserver.VPNConfig,
) (
	kubeapiserver.Images,
	error,
) {
	var result kubeapiserver.Images

	imageKubeAPIServer, err := imageVector.FindImage(images.ImageNameKubeApiserver, imagevector.RuntimeVersion(runtimeVersion.String()), imagevector.TargetVersion(targetVersion.String()))
	if err != nil {
		return kubeapiserver.Images{}, err
	}
	result.KubeAPIServer = imageKubeAPIServer.String()

	imageApiserverProxyPodWebhook, err := imageVector.FindImage(images.ImageNameApiserverProxyPodWebhook, imagevector.RuntimeVersion(runtimeVersion.String()), imagevector.TargetVersion(targetVersion.String()))
	if err != nil {
		return kubeapiserver.Images{}, err
	}
	result.APIServerProxyPodWebhook = imageApiserverProxyPodWebhook.String()

	if version.ConstraintK8sEqual124.Check(targetVersion) {
		imageWatchdog, err := imageVector.FindImage(images.ImageNameAlpine, imagevector.RuntimeVersion(runtimeVersion.String()), imagevector.TargetVersion(targetVersion.String()))
		if err != nil {
			return kubeapiserver.Images{}, err
		}
		result.Watchdog = imageWatchdog.String()
	}

	if vpnConfig.HighAvailabilityEnabled {
		imageVPNClient, err := imageVector.FindImage(images.ImageNameVpnShootClient, imagevector.RuntimeVersion(runtimeVersion.String()), imagevector.TargetVersion(targetVersion.String()))
		if err != nil {
			return kubeapiserver.Images{}, err
		}
		result.VPNClient = imageVPNClient.String()
	}

	return result, nil
}

func ensureAdmissionPluginConfig(plugins []gardencorev1beta1.AdmissionPlugin) ([]gardencorev1beta1.AdmissionPlugin, error) {
	var index = -1

	for i, plugin := range plugins {
		if plugin.Name == "PodSecurity" {
			index = i
			break
		}
	}

	if index == -1 {
		return plugins, nil
	}

	// If user has set a config in the shoot spec, retrieve it
	if plugins[index].Config != nil {
		var (
			admissionConfigData []byte
			err                 error
		)

		config, err := runtime.Decode(codec, plugins[index].Config.Raw)
		if err != nil {
			return nil, err
		}

		// Add kube-system to exempted namespaces
		switch admissionConfig := config.(type) {
		case *admissionapiv1alpha1.PodSecurityConfiguration:
			if !slices.Contains(admissionConfig.Exemptions.Namespaces, metav1.NamespaceSystem) {
				admissionConfig.Exemptions.Namespaces = append(admissionConfig.Exemptions.Namespaces, metav1.NamespaceSystem)
			}
			admissionConfigData, err = runtime.Encode(codec, admissionConfig)
		case *admissionapiv1beta1.PodSecurityConfiguration:
			if !slices.Contains(admissionConfig.Exemptions.Namespaces, metav1.NamespaceSystem) {
				admissionConfig.Exemptions.Namespaces = append(admissionConfig.Exemptions.Namespaces, metav1.NamespaceSystem)
			}
			admissionConfigData, err = runtime.Encode(codec, admissionConfig)
		case *admissionapiv1.PodSecurityConfiguration:
			if !slices.Contains(admissionConfig.Exemptions.Namespaces, metav1.NamespaceSystem) {
				admissionConfig.Exemptions.Namespaces = append(admissionConfig.Exemptions.Namespaces, metav1.NamespaceSystem)
			}
			admissionConfigData, err = runtime.Encode(codec, admissionConfig)
		default:
			err = fmt.Errorf("expected admissionapiv1alpha1.PodSecurityConfiguration, admissionapiv1beta1.PodSecurityConfiguration or admissionapiv1.PodSecurityConfiguration in PodSecurity plugin configuration but got %T", config)
		}

		if err != nil {
			return nil, err
		}

		plugins[index].Config = &runtime.RawExtension{Raw: admissionConfigData}
	}

	return plugins, nil
}

func convertToAdmissionPluginConfigs(plugins []gardencorev1beta1.AdmissionPlugin) ([]kubeapiserver.AdmissionPluginConfig, error) {
	var out []kubeapiserver.AdmissionPluginConfig

	for _, plugin := range plugins {
		out = append(out, kubeapiserver.AdmissionPluginConfig{
			AdmissionPlugin: plugin,
		})
	}

	return out, nil
}

func computeEnabledKubeAPIServerAdmissionPlugins(defaultPlugins, configuredPlugins []gardencorev1beta1.AdmissionPlugin) []gardencorev1beta1.AdmissionPlugin {
	for _, plugin := range configuredPlugins {
		pluginOverwritesDefault := false

		for i, defaultPlugin := range defaultPlugins {
			if defaultPlugin.Name == plugin.Name {
				pluginOverwritesDefault = true
				defaultPlugins[i] = plugin
				break
			}
		}

		if !pluginOverwritesDefault {
			defaultPlugins = append(defaultPlugins, plugin)
		}
	}

	var admissionPlugins []gardencorev1beta1.AdmissionPlugin
	for _, defaultPlugin := range defaultPlugins {
		if !pointer.BoolDeref(defaultPlugin.Disabled, false) {
			admissionPlugins = append(admissionPlugins, defaultPlugin)
		}
	}
	return admissionPlugins
}

func computeDisabledKubeAPIServerAdmissionPlugins(configuredPlugins []gardencorev1beta1.AdmissionPlugin) []gardencorev1beta1.AdmissionPlugin {
	var disabledAdmissionPlugins []gardencorev1beta1.AdmissionPlugin

	for _, plugin := range configuredPlugins {
		if pointer.BoolDeref(plugin.Disabled, false) {
			disabledAdmissionPlugins = append(disabledAdmissionPlugins, plugin)
		}
	}

	return disabledAdmissionPlugins
}

func computeKubeAPIServerAuditConfig(
	ctx context.Context,
	cl client.Client,
	objectMeta metav1.ObjectMeta,
	config *gardencorev1beta1.AuditConfig,
) (
	*kubeapiserver.AuditConfig,
	error,
) {
	if config == nil || config.AuditPolicy == nil || config.AuditPolicy.ConfigMapRef == nil {
		return nil, nil
	}

	out := &kubeapiserver.AuditConfig{}

	configMap := &corev1.ConfigMap{}
	if err := cl.Get(ctx, kubernetesutils.Key(objectMeta.Namespace, config.AuditPolicy.ConfigMapRef.Name), configMap); err != nil {
		// Ignore missing audit configuration on shoot deletion to prevent failing redeployments of the
		// kube-apiserver in case the end-user deleted the configmap before/simultaneously to the shoot
		// deletion.
		if !apierrors.IsNotFound(err) || objectMeta.DeletionTimestamp == nil {
			return nil, fmt.Errorf("retrieving audit policy from the ConfigMap '%v' failed with reason '%w'", config.AuditPolicy.ConfigMapRef.Name, err)
		}
	} else {
		policy, ok := configMap.Data["policy"]
		if !ok {
			return nil, fmt.Errorf("missing '.data.policy' in audit policy configmap %v/%v", objectMeta.Namespace, config.AuditPolicy.ConfigMapRef.Name)
		}
		out.Policy = &policy
	}

	return out, nil
}
