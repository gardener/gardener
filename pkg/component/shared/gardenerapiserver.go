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

	"github.com/Masterminds/semver/v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/component-base/version"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/imagevector"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	"github.com/gardener/gardener/pkg/component/apiserver"
	"github.com/gardener/gardener/pkg/component/gardenerapiserver"
	"github.com/gardener/gardener/pkg/logger"
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
	autoscalingConfig apiserver.AutoscalingConfig,
	auditWebhookConfig *apiserver.AuditWebhook,
	topologyAwareRoutingEnabled bool,
	clusterIdentity string,
) (
	gardenerapiserver.Interface,
	error,
) {
	image, err := imagevector.ImageVector().FindImage(imagevector.ImageNameGardenerApiserver)
	if err != nil {
		return nil, err
	}
	image.WithOptionalTag(version.Get().GitVersion)

	var (
		auditConfig              *apiserver.AuditConfig
		enabledAdmissionPlugins  []gardencorev1beta1.AdmissionPlugin
		disabledAdmissionPlugins []gardencorev1beta1.AdmissionPlugin
		featureGates             map[string]bool
		requests                 *gardencorev1beta1.APIServerRequests
		watchCacheSizes          *gardencorev1beta1.WatchCacheSizes
		logging                  *gardencorev1beta1.APIServerLogging
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
	}

	logLevel := logger.InfoLevel
	if logging != nil && pointer.Int32Deref(logging.Verbosity, 0) > 2 {
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
				Autoscaling:              autoscalingConfig,
				FeatureGates:             featureGates,
				Logging:                  logging,
				Requests:                 requests,
				RuntimeVersion:           runtimeVersion,
				WatchCacheSizes:          watchCacheSizes,
			},
			ClusterIdentity:             clusterIdentity,
			Image:                       image.String(),
			LogLevel:                    logLevel,
			LogFormat:                   logger.FormatJSON,
			TopologyAwareRoutingEnabled: topologyAwareRoutingEnabled,
		},
	), nil
}

// DeployGardenerAPIServer deploys the Gardener API server.
func DeployGardenerAPIServer(
	ctx context.Context,
	runtimeClient client.Client,
	runtimeNamespace string,
	gardenerAPIServer gardenerapiserver.Interface,
	etcdEncryptionKeyRotationPhase gardencorev1beta1.CredentialsRotationPhase,
) error {
	etcdEncryptionConfig, err := computeAPIServerETCDEncryptionConfig(ctx, runtimeClient, runtimeNamespace, gardenerapiserver.DeploymentName, etcdEncryptionKeyRotationPhase, []string{
		gardencorev1beta1.Resource("controllerdeployments").String(),
		gardencorev1beta1.Resource("controllerregistrations").String(),
		gardencorev1beta1.Resource("internalsecrets").String(),
		gardencorev1beta1.Resource("shootstates").String(),
	})
	if err != nil {
		return err
	}
	gardenerAPIServer.SetETCDEncryptionConfig(etcdEncryptionConfig)

	if err := gardenerAPIServer.Deploy(ctx); err != nil {
		return err
	}

	return handleETCDEncryptionKeyRotation(ctx, runtimeClient, runtimeNamespace, gardenerapiserver.DeploymentName, gardenerAPIServer, etcdEncryptionConfig, etcdEncryptionKeyRotationPhase)
}
