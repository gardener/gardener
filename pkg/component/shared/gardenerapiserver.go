// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shared

import (
	"context"

	"github.com/Masterminds/semver/v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/component-base/version"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/imagevector"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	"github.com/gardener/gardener/pkg/component/apiserver"
	gardenerapiserver "github.com/gardener/gardener/pkg/component/gardener/apiserver"
	"github.com/gardener/gardener/pkg/logger"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

// NewGardenerAPIServer returns a deployer for the gardener-apiserver.
func NewGardenerAPIServer(
	ctx context.Context,
	runtimeClient client.Client,
	runtimeNamespace string,
	objectMeta metav1.ObjectMeta,
	runtimeVersion *semver.Version,
	secretsManager secretsmanager.Interface,
	apiServerConfig *operatorv1alpha1.GardenerAPIServerConfig,
	autoscalingConfig gardenerapiserver.AutoscalingConfig,
	auditWebhookConfig *apiserver.AuditWebhook,
	topologyAwareRoutingEnabled bool,
	clusterIdentity,
	workloadIdentityTokenIssuer string,
	goAwayChance *float64,
) (
	gardenerapiserver.Interface,
	error,
) {
	image, err := imagevector.Containers().FindImage(imagevector.ContainerImageNameGardenerApiserver)
	if err != nil {
		return nil, err
	}
	image.WithOptionalTag(version.Get().GitVersion)

	var (
		auditConfig                       *apiserver.AuditConfig
		enabledAdmissionPlugins           []gardencorev1beta1.AdmissionPlugin
		disabledAdmissionPlugins          []gardencorev1beta1.AdmissionPlugin
		featureGates                      map[string]bool
		requests                          *gardencorev1beta1.APIServerRequests
		watchCacheSizes                   *gardencorev1beta1.WatchCacheSizes
		logging                           *gardencorev1beta1.APIServerLogging
		shootAdminKubeconfigMaxExpiration *metav1.Duration
	)

	if apiServerConfig != nil {
		auditConfig, err = computeAPIServerAuditConfig(ctx, runtimeClient, objectMeta, apiServerConfig.AuditConfig, auditWebhookConfig)
		if err != nil {
			return nil, err
		}

		enabledAdmissionPlugins = computeEnabledAPIServerAdmissionPlugins(enabledAdmissionPlugins, apiServerConfig.AdmissionPlugins)
		disabledAdmissionPlugins = computeDisabledAPIServerAdmissionPlugins(apiServerConfig.AdmissionPlugins)
		featureGates = apiServerConfig.FeatureGates
		logging = apiServerConfig.Logging
		requests = apiServerConfig.Requests
		watchCacheSizes = apiServerConfig.WatchCacheSizes
		shootAdminKubeconfigMaxExpiration = apiServerConfig.ShootAdminKubeconfigMaxExpiration
	}

	logLevel := logger.InfoLevel
	if logging != nil && ptr.Deref(logging.Verbosity, 0) > 2 {
		logLevel = logger.DebugLevel
	}

	enabledAdmissionPluginConfigs, err := convertToAdmissionPluginConfigs(ctx, runtimeClient, runtimeNamespace, enabledAdmissionPlugins)
	if err != nil {
		return nil, err
	}

	return gardenerapiserver.New(
		runtimeClient,
		runtimeNamespace,
		secretsManager,
		gardenerapiserver.Values{
			Values: apiserver.Values{
				EnabledAdmissionPlugins:  enabledAdmissionPluginConfigs,
				DisabledAdmissionPlugins: disabledAdmissionPlugins,
				Audit:                    auditConfig,
				FeatureGates:             featureGates,
				Logging:                  logging,
				Requests:                 requests,
				RuntimeVersion:           runtimeVersion,
				WatchCacheSizes:          watchCacheSizes,
			},
			Autoscaling:                       autoscalingConfig,
			ClusterIdentity:                   clusterIdentity,
			Image:                             image.String(),
			LogLevel:                          logLevel,
			LogFormat:                         logger.FormatJSON,
			GoAwayChance:                      goAwayChance,
			ShootAdminKubeconfigMaxExpiration: shootAdminKubeconfigMaxExpiration,
			TopologyAwareRoutingEnabled:       topologyAwareRoutingEnabled,
			WorkloadIdentityTokenIssuer:       workloadIdentityTokenIssuer,
		},
	), nil
}

// DeployGardenerAPIServer deploys the Gardener API server.
func DeployGardenerAPIServer(
	ctx context.Context,
	runtimeClient client.Client,
	runtimeNamespace string,
	gardenerAPIServer gardenerapiserver.Interface,
	resourcesToEncrypt []string,
	encryptedResources []string,
	etcdEncryptionKeyRotationPhase gardencorev1beta1.CredentialsRotationPhase,
	workloadIdentityKeyRotationPhase gardencorev1beta1.CredentialsRotationPhase,
) error {
	etcdEncryptionConfig, err := computeAPIServerETCDEncryptionConfig(
		ctx,
		runtimeClient,
		runtimeNamespace,
		gardenerapiserver.DeploymentName,
		etcdEncryptionKeyRotationPhase,
		append(resourcesToEncrypt, sets.List(gardenerutils.DefaultGardenerResourcesForEncryption())...),
		append(encryptedResources, sets.List(gardenerutils.DefaultGardenerResourcesForEncryption())...),
	)
	if err != nil {
		return err
	}
	gardenerAPIServer.SetETCDEncryptionConfig(etcdEncryptionConfig)
	gardenerAPIServer.SetWorkloadIdentityKeyRotationPhase(workloadIdentityKeyRotationPhase)

	if err := gardenerAPIServer.Deploy(ctx); err != nil {
		return err
	}

	return handleETCDEncryptionKeyRotation(ctx, runtimeClient, runtimeNamespace, gardenerapiserver.DeploymentName, gardenerAPIServer, etcdEncryptionConfig, etcdEncryptionKeyRotationPhase)
}
