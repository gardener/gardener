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
	"slices"

	"github.com/Masterminds/semver/v3"
	appsv1 "k8s.io/api/apps/v1"
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
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/imagevector"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component/apiserver"
	"github.com/gardener/gardener/pkg/component/kubeapiserver"
	"github.com/gardener/gardener/pkg/utils"
	imagevectorutils "github.com/gardener/gardener/pkg/utils/imagevector"
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
	resourceConfigClient client.Client,
	runtimeNamespace string,
	objectMeta metav1.ObjectMeta,
	runtimeVersion *semver.Version,
	targetVersion *semver.Version,
	secretsManager secretsmanager.Interface,
	namePrefix string,
	apiServerConfig *gardencorev1beta1.KubeAPIServerConfig,
	autoscalingConfig apiserver.AutoscalingConfig,
	serviceNetworkCIDR string,
	vpnConfig kubeapiserver.VPNConfig,
	priorityClassName string,
	isWorkerless bool,
	staticTokenKubeconfigEnabled *bool,
	auditWebhookConfig *apiserver.AuditWebhook,
	authenticationWebhookConfig *kubeapiserver.AuthenticationWebhook,
	authorizationWebhookConfig *kubeapiserver.AuthorizationWebhook,
	resourcesToStoreInETCDEvents []schema.GroupResource,
	fastRollout bool,
) (
	kubeapiserver.Interface,
	error,
) {
	images, err := computeKubeAPIServerImages(runtimeVersion, targetVersion, vpnConfig)
	if err != nil {
		return nil, err
	}

	var (
		enabledAdmissionPlugins             = kubernetesutils.GetAdmissionPluginsForVersion(targetVersion.String())
		disabledAdmissionPlugins            []gardencorev1beta1.AdmissionPlugin
		apiAudiences                        = []string{"kubernetes", "gardener"}
		auditConfig                         *apiserver.AuditConfig
		defaultNotReadyTolerationSeconds    *int64
		defaultUnreachableTolerationSeconds *int64
		eventTTL                            *metav1.Duration
		featureGates                        map[string]bool
		oidcConfig                          *gardencorev1beta1.OIDCConfig
		requests                            *gardencorev1beta1.APIServerRequests
		runtimeConfig                       map[string]bool
		watchCacheSizes                     *gardencorev1beta1.WatchCacheSizes
		logging                             *gardencorev1beta1.APIServerLogging
	)

	if apiServerConfig != nil {
		enabledAdmissionPlugins = computeEnabledKubeAPIServerAdmissionPlugins(enabledAdmissionPlugins, apiServerConfig.AdmissionPlugins, isWorkerless)
		disabledAdmissionPlugins = computeDisabledAPIServerAdmissionPlugins(apiServerConfig.AdmissionPlugins)

		enabledAdmissionPlugins, err = ensureKubeAPIServerAdmissionPluginConfig(enabledAdmissionPlugins)
		if err != nil {
			return nil, err
		}

		if apiServerConfig.APIAudiences != nil {
			apiAudiences = apiServerConfig.APIAudiences
			if !utils.ValueExists(v1beta1constants.GardenerAudience, apiAudiences) {
				apiAudiences = append(apiAudiences, v1beta1constants.GardenerAudience)
			}
		}

		auditConfig, err = computeAPIServerAuditConfig(ctx, resourceConfigClient, objectMeta, apiServerConfig.AuditConfig, auditWebhookConfig)
		if err != nil {
			return nil, err
		}

		defaultNotReadyTolerationSeconds = apiServerConfig.DefaultNotReadyTolerationSeconds
		defaultUnreachableTolerationSeconds = apiServerConfig.DefaultUnreachableTolerationSeconds
		eventTTL = apiServerConfig.EventTTL
		featureGates = apiServerConfig.FeatureGates
		logging = apiServerConfig.Logging
		oidcConfig = apiServerConfig.OIDCConfig
		requests = apiServerConfig.Requests
		runtimeConfig = apiServerConfig.RuntimeConfig
		watchCacheSizes = apiServerConfig.WatchCacheSizes
	}

	enabledAdmissionPluginConfigs, err := convertToAdmissionPluginConfigs(ctx, resourceConfigClient, objectMeta.Namespace, enabledAdmissionPlugins)
	if err != nil {
		return nil, err
	}

	return kubeapiserver.New(
		runtimeClientSet,
		runtimeNamespace,
		secretsManager,
		kubeapiserver.Values{
			Values: apiserver.Values{
				EnabledAdmissionPlugins:  enabledAdmissionPluginConfigs,
				DisabledAdmissionPlugins: disabledAdmissionPlugins,
				Audit:                    auditConfig,
				Autoscaling:              autoscalingConfig,
				FeatureGates:             featureGates,
				Logging:                  logging,
				Requests:                 requests,
				RuntimeVersion:           runtimeVersion,
				WatchCacheSizes:          watchCacheSizes,
			},
			AnonymousAuthenticationEnabled:      v1beta1helper.AnonymousAuthenticationEnabled(apiServerConfig),
			APIAudiences:                        apiAudiences,
			AuthenticationWebhook:               authenticationWebhookConfig,
			AuthorizationWebhook:                authorizationWebhookConfig,
			DefaultNotReadyTolerationSeconds:    defaultNotReadyTolerationSeconds,
			DefaultUnreachableTolerationSeconds: defaultUnreachableTolerationSeconds,
			EventTTL:                            eventTTL,
			Images:                              images,
			IsWorkerless:                        isWorkerless,
			NamePrefix:                          namePrefix,
			OIDC:                                oidcConfig,
			PriorityClassName:                   priorityClassName,
			ResourcesToStoreInETCDEvents:        resourcesToStoreInETCDEvents,
			RuntimeConfig:                       runtimeConfig,
			ServiceNetworkCIDR:                  serviceNetworkCIDR,
			StaticTokenKubeconfigEnabled:        staticTokenKubeconfigEnabled,
			Version:                             targetVersion,
			VPN:                                 vpnConfig,
			FastRollout:                         fastRollout,
		},
	), nil
}

// DeployKubeAPIServer deploys the Kubernetes API server.
func DeployKubeAPIServer(
	ctx context.Context,
	runtimeClient client.Client,
	runtimeNamespace string,
	kubeAPIServer kubeapiserver.Interface,
	apiServerConfig *gardencorev1beta1.KubeAPIServerConfig,
	serverCertificateConfig kubeapiserver.ServerCertificateConfig,
	sniConfig kubeapiserver.SNIConfig,
	externalHostname string,
	externalServer string,
	etcdEncryptionKeyRotationPhase gardencorev1beta1.CredentialsRotationPhase,
	serviceAccountKeyRotationPhase gardencorev1beta1.CredentialsRotationPhase,
	wantScaleDown bool,
) error {
	var (
		values         = kubeAPIServer.GetValues()
		deploymentName = values.NamePrefix + v1beta1constants.DeploymentNameKubeAPIServer
	)

	deployment := &appsv1.Deployment{}
	if err := runtimeClient.Get(ctx, kubernetesutils.Key(runtimeNamespace, deploymentName), deployment); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
		deployment = nil
	}

	kubeAPIServer.SetAutoscalingReplicas(computeKubeAPIServerReplicas(values.Autoscaling, deployment, wantScaleDown))

	if deployment != nil && values.Autoscaling.HVPAEnabled {
		for _, container := range deployment.Spec.Template.Spec.Containers {
			if container.Name == kubeapiserver.ContainerNameKubeAPIServer {
				// Only set requests to allow limits to be removed
				kubeAPIServer.SetAutoscalingAPIServerResources(corev1.ResourceRequirements{Requests: container.Resources.Requests})
				break
			}
		}
	}

	kubeAPIServer.SetServerCertificateConfig(serverCertificateConfig)
	kubeAPIServer.SetServiceAccountConfig(computeKubeAPIServerServiceAccountConfig(apiServerConfig, externalHostname, serviceAccountKeyRotationPhase))
	kubeAPIServer.SetSNIConfig(sniConfig)
	kubeAPIServer.SetExternalHostname(externalHostname)
	kubeAPIServer.SetExternalServer(externalServer)

	etcdEncryptionConfig, err := computeAPIServerETCDEncryptionConfig(ctx, runtimeClient, runtimeNamespace, deploymentName, etcdEncryptionKeyRotationPhase, []string{corev1.Resource("secrets").String()})
	if err != nil {
		return err
	}
	kubeAPIServer.SetETCDEncryptionConfig(etcdEncryptionConfig)

	if err := kubeAPIServer.Deploy(ctx); err != nil {
		return err
	}

	return handleETCDEncryptionKeyRotation(ctx, runtimeClient, runtimeNamespace, deploymentName, kubeAPIServer, etcdEncryptionConfig, etcdEncryptionKeyRotationPhase)
}

func computeKubeAPIServerImages(
	runtimeVersion *semver.Version,
	targetVersion *semver.Version,
	vpnConfig kubeapiserver.VPNConfig,
) (
	kubeapiserver.Images,
	error,
) {
	var result kubeapiserver.Images

	imageKubeAPIServer, err := imagevector.ImageVector().FindImage(imagevector.ImageNameKubeApiserver, imagevectorutils.RuntimeVersion(runtimeVersion.String()), imagevectorutils.TargetVersion(targetVersion.String()))
	if err != nil {
		return kubeapiserver.Images{}, err
	}
	result.KubeAPIServer = imageKubeAPIServer.String()

	if version.ConstraintK8sEqual124.Check(targetVersion) {
		imageWatchdog, err := imagevector.ImageVector().FindImage(imagevector.ImageNameAlpine, imagevectorutils.RuntimeVersion(runtimeVersion.String()), imagevectorutils.TargetVersion(targetVersion.String()))
		if err != nil {
			return kubeapiserver.Images{}, err
		}
		result.Watchdog = imageWatchdog.String()
	}

	if vpnConfig.HighAvailabilityEnabled {
		imageVPNClient, err := imagevector.ImageVector().FindImage(imagevector.ImageNameVpnShootClient, imagevectorutils.RuntimeVersion(runtimeVersion.String()), imagevectorutils.TargetVersion(targetVersion.String()))
		if err != nil {
			return kubeapiserver.Images{}, err
		}
		result.VPNClient = imageVPNClient.String()
	}

	return result, nil
}

func ensureKubeAPIServerAdmissionPluginConfig(plugins []gardencorev1beta1.AdmissionPlugin) ([]gardencorev1beta1.AdmissionPlugin, error) {
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

func computeEnabledKubeAPIServerAdmissionPlugins(defaultPlugins, configuredPlugins []gardencorev1beta1.AdmissionPlugin, isWorkerless bool) []gardencorev1beta1.AdmissionPlugin {
	var admissionPlugins []gardencorev1beta1.AdmissionPlugin
	for _, defaultPlugin := range computeEnabledAPIServerAdmissionPlugins(defaultPlugins, configuredPlugins) {
		// if it's a workerless cluster, we don't add the PodSecurityPolicy plugin, because the API is disabled already
		if isWorkerless && defaultPlugin.Name == "PodSecurityPolicy" {
			continue
		}
		admissionPlugins = append(admissionPlugins, defaultPlugin)
	}
	return admissionPlugins
}

func computeKubeAPIServerReplicas(autoscalingConfig apiserver.AutoscalingConfig, deployment *appsv1.Deployment, wantScaleDown bool) *int32 {
	switch {
	case autoscalingConfig.Replicas != nil:
		// If the replicas were already set then don't change them.
		return autoscalingConfig.Replicas
	case deployment == nil && !wantScaleDown:
		// If the Deployment does not yet exist then set the desired replicas to the minimum replicas.
		return &autoscalingConfig.MinReplicas
	case deployment != nil && deployment.Spec.Replicas != nil && *deployment.Spec.Replicas > 0:
		// If the Deployment exists then don't interfere with the replicas because they are controlled via HVPA or HPA.
		return deployment.Spec.Replicas
	case wantScaleDown && (deployment == nil || deployment.Spec.Replicas == nil || *deployment.Spec.Replicas == 0):
		// If the scale down is desired and the deployment has already been scaled down then we want to keep it scaled
		// down. If it has not yet been scaled down then above case applies (replicas are kept) - the scale-down will
		// happen at a later point in the flow.
		return pointer.Int32(0)
	default:
		// If none of the above cases applies then a default value has to be returned.
		return pointer.Int32(1)
	}
}

func computeKubeAPIServerServiceAccountConfig(
	config *gardencorev1beta1.KubeAPIServerConfig,
	externalHostname string,
	serviceAccountKeyRotationPhase gardencorev1beta1.CredentialsRotationPhase,
) kubeapiserver.ServiceAccountConfig {
	var (
		defaultIssuer = "https://" + externalHostname
		out           = kubeapiserver.ServiceAccountConfig{
			Issuer:        defaultIssuer,
			RotationPhase: serviceAccountKeyRotationPhase,
		}
	)

	if config == nil || config.ServiceAccountConfig == nil {
		return out
	}

	out.ExtendTokenExpiration = config.ServiceAccountConfig.ExtendTokenExpiration
	out.MaxTokenExpiration = config.ServiceAccountConfig.MaxTokenExpiration

	if config.ServiceAccountConfig.Issuer != nil {
		out.Issuer = *config.ServiceAccountConfig.Issuer
	}
	out.AcceptedIssuers = config.ServiceAccountConfig.AcceptedIssuers
	if out.Issuer != defaultIssuer && !utils.ValueExists(defaultIssuer, out.AcceptedIssuers) {
		out.AcceptedIssuers = append(out.AcceptedIssuers, defaultIssuer)
	}
	if config.ServiceAccountConfig.Issuer == nil {
		// ensure defaultIssuer is not duplicated in the accepted issuers
		for i, val := range out.AcceptedIssuers {
			if val == defaultIssuer {
				out.AcceptedIssuers = append(out.AcceptedIssuers[:i], out.AcceptedIssuers[i+1:]...)
				break
			}
		}
	}

	return out
}
