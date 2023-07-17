// Copyright 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"fmt"
	"path/filepath"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/pointer"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/monitoring"
	"github.com/gardener/gardener/pkg/features"
	gardenlethelper "github.com/gardener/gardener/pkg/gardenlet/apis/config/helper"
	"github.com/gardener/gardener/pkg/operation/common"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/pkg/utils/images"
	"github.com/gardener/gardener/pkg/utils/secrets"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

// DefaultMonitoring creates a new monitoring component.
func (b *Botanist) DefaultMonitoring() (component.Deployer, error) {
	imageBlackboxExporter, err := b.ImageVector.FindImage(images.ImageNameBlackboxExporter)
	if err != nil {
		return nil, err
	}
	imageConfigmapReloader, err := b.ImageVector.FindImage(images.ImageNameConfigmapReloader)
	if err != nil {
		return nil, err
	}
	imagePrometheus, err := b.ImageVector.FindImage(images.ImageNamePrometheus)
	if err != nil {
		return nil, err
	}

	var alertingSecrets []*corev1.Secret
	for _, key := range b.GetSecretKeysOfRole(v1beta1constants.GardenRoleAlerting) {
		alertingSecrets = append(alertingSecrets, b.LoadSecret(key))
	}

	// Fetch component-specific monitoring configuration
	monitoringComponents := []component.MonitoringComponent{
		b.Shoot.Components.ControlPlane.EtcdMain,
		b.Shoot.Components.ControlPlane.EtcdEvents,
		b.Shoot.Components.ControlPlane.KubeAPIServer,
		b.Shoot.Components.ControlPlane.KubeControllerManager,
		b.Shoot.Components.ControlPlane.KubeStateMetrics,
		b.Shoot.Components.ControlPlane.ResourceManager,
	}

	if b.Shoot.IsShootControlPlaneLoggingEnabled(b.Config) && gardenlethelper.IsValiEnabled(b.Config) {
		monitoringComponents = append(monitoringComponents, b.Shoot.Components.Logging.Vali)
	}

	if !b.Shoot.IsWorkerless {
		monitoringComponents = append(monitoringComponents,
			b.Shoot.Components.SystemComponents.BlackboxExporter,
			b.Shoot.Components.ControlPlane.KubeScheduler,
			b.Shoot.Components.SystemComponents.CoreDNS,
			b.Shoot.Components.SystemComponents.KubeProxy,
			b.Shoot.Components.SystemComponents.NodeExporter,
			b.Shoot.Components.SystemComponents.VPNShoot,
			b.Shoot.Components.ControlPlane.VPNSeedServer,
		)

		if b.ShootUsesDNS() {
			monitoringComponents = append(monitoringComponents, b.Shoot.Components.SystemComponents.APIServerProxy)
		}

		if b.Shoot.NodeLocalDNSEnabled {
			monitoringComponents = append(monitoringComponents, b.Shoot.Components.SystemComponents.NodeLocalDNS)
		}

		if b.Shoot.WantsClusterAutoscaler {
			monitoringComponents = append(monitoringComponents, b.Shoot.Components.ControlPlane.ClusterAutoscaler)
		}

		if features.DefaultFeatureGate.Enabled(features.MachineControllerManagerDeployment) {
			monitoringComponents = append(monitoringComponents, b.Shoot.Components.ControlPlane.MachineControllerManager)
		}
	}

	var wildcardSecretName *string
	if b.ControlPlaneWildcardCert != nil {
		wildcardSecretName = &b.ControlPlaneWildcardCert.Name
	}

	values := monitoring.Values{
		AlertingSecrets:              alertingSecrets,
		AlertmanagerEnabled:          b.Shoot.WantsAlertmanager,
		APIServerDomain:              gardenerutils.GetAPIServerDomain(b.Shoot.InternalClusterDomain),
		APIServerHost:                b.SeedClientSet.RESTConfig().Host,
		Components:                   monitoringComponents,
		Config:                       b.Config.Monitoring,
		GardenletManagesMCM:          features.DefaultFeatureGate.Enabled(features.MachineControllerManagerDeployment),
		GlobalShootRemoteWriteSecret: b.LoadSecret(v1beta1constants.GardenRoleGlobalShootRemoteWriteMonitoring),
		IgnoreAlerts:                 b.Shoot.IgnoreAlerts,
		ImageBlackboxExporter:        imageBlackboxExporter.String(),
		ImageConfigmapReloader:       imageConfigmapReloader.String(),
		ImagePrometheus:              imagePrometheus.String(),
		IngressHost:                  b.ComputePrometheusHost(),
		IsWorkerless:                 b.Shoot.IsWorkerless,
		KubernetesVersion:            b.Shoot.GetInfo().Spec.Kubernetes.Version,
		NamespaceUID:                 b.SeedNamespaceObject.UID,
		NodeLocalDNSEnabled:          b.Shoot.NodeLocalDNSEnabled,
		ProjectName:                  b.Garden.Project.Name,
		Replicas:                     b.Shoot.GetReplicas(1),
		RuntimeProviderType:          b.Seed.GetInfo().Spec.Provider.Type,
		RuntimeRegion:                b.Seed.GetInfo().Spec.Provider.Region,
		TargetName:                   b.Shoot.GetInfo().Name,
		TargetProviderType:           b.Shoot.GetInfo().Spec.Provider.Type,
		WildcardCertName:             wildcardSecretName,
	}

	if b.Shoot.Networks != nil {
		if services := b.Shoot.Networks.Services; services != nil {
			values.ServiceNetworkCIDR = pointer.String(services.String())
		}
		if pods := b.Shoot.Networks.Pods; pods != nil {
			values.PodNetworkCIDR = pointer.String(pods.String())
		}
		if apiServer := b.Shoot.Networks.APIServer; apiServer != nil {
			values.APIServerServiceIP = pointer.String(apiServer.String())
		}
	}
	if b.Shoot.GetInfo().Spec.Networking != nil {
		values.NodeNetworkCIDR = b.Shoot.GetInfo().Spec.Networking.Nodes
	}

	return monitoring.New(
		b.SeedClientSet.Client(),
		b.SeedClientSet.ChartApplier(),
		b.SecretsManager,
		b.Shoot.SeedNamespace,
		values,
	), nil
}

// DeployMonitoring installs the Helm release "seed-monitoring" in the Seed clusters. It comprises components
// to monitor the Shoot cluster whose control plane runs in the Seed cluster.
func (b *Botanist) DeployMonitoring(ctx context.Context) error {
	if !b.IsShootMonitoringEnabled() {
		return b.Shoot.Components.Monitoring.Monitoring.Destroy(ctx)
	}

	if err := b.Shoot.Components.Monitoring.Monitoring.Deploy(ctx); err != nil {
		return err
	}

	// Check if we want to deploy an alertmanager into the shoot namespace.
	if b.Shoot.WantsAlertmanager {
		var (
			alertingSMTPKeys = b.GetSecretKeysOfRole(v1beta1constants.GardenRoleAlerting)
			emailConfigs     = []map[string]interface{}{}
		)

		if b.Shoot.GetInfo().Spec.Monitoring != nil && b.Shoot.GetInfo().Spec.Monitoring.Alerting != nil {
			for _, email := range b.Shoot.GetInfo().Spec.Monitoring.Alerting.EmailReceivers {
				for _, key := range alertingSMTPKeys {
					secret := b.LoadSecret(key)

					if string(secret.Data["auth_type"]) != "smtp" {
						continue
					}
					emailConfigs = append(emailConfigs, map[string]interface{}{
						"to":            email,
						"from":          string(secret.Data["from"]),
						"smarthost":     string(secret.Data["smarthost"]),
						"auth_username": string(secret.Data["auth_username"]),
						"auth_identity": string(secret.Data["auth_identity"]),
						"auth_password": string(secret.Data["auth_password"]),
					})
				}
			}
		}

		var alertManagerIngressTLSSecretName string
		if b.ControlPlaneWildcardCert != nil {
			alertManagerIngressTLSSecretName = b.ControlPlaneWildcardCert.GetName()
		} else {
			ingressTLSSecret, err := b.SecretsManager.Generate(ctx, &secrets.CertificateSecretConfig{
				Name:                        "alertmanager-tls",
				CommonName:                  "alertmanager",
				Organization:                []string{"gardener.cloud:monitoring:ingress"},
				DNSNames:                    b.ComputeAlertManagerHosts(),
				CertType:                    secrets.ServerCert,
				Validity:                    pointer.Duration(v1beta1constants.IngressTLSCertificateValidity),
				SkipPublishingCACertificate: true,
			}, secretsmanager.SignedByCA(v1beta1constants.SecretNameCACluster))
			if err != nil {
				return err
			}
			alertManagerIngressTLSSecretName = ingressTLSSecret.Name
		}

		alertManagerValues, err := b.InjectSeedShootImages(map[string]interface{}{
			"ingress": map[string]interface{}{
				"class":          v1beta1constants.SeedNginxIngressClass,
				"authSecretName": credentialsSecret.Name,
				"hosts": []map[string]interface{}{
					{
						"hostName":   b.ComputeAlertManagerHost(),
						"secretName": alertManagerIngressTLSSecretName,
					},
				},
			},
			"replicas":     b.Shoot.GetReplicas(1),
			"storage":      b.Seed.GetValidVolumeSize("1Gi"),
			"emailConfigs": emailConfigs,
		}, images.ImageNameAlertmanager, images.ImageNameConfigmapReloader)
		if err != nil {
			return err
		}

		return b.SeedClientSet.ChartApplier().Apply(ctx, filepath.Join(ChartsPath, "seed-monitoring", "charts", "alertmanager"), b.Shoot.SeedNamespace, fmt.Sprintf("%s-monitoring", b.Shoot.SeedNamespace), kubernetes.Values(alertManagerValues))
	}

	return common.DeleteAlertmanager(ctx, b.SeedClientSet.Client(), b.Shoot.SeedNamespace)
}
