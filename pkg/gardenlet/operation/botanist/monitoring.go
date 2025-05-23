// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"
	"fmt"
	"strconv"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/utils/ptr"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/observability/monitoring/alertmanager"
	"github.com/gardener/gardener/pkg/component/observability/monitoring/prometheus"
	shootprometheus "github.com/gardener/gardener/pkg/component/observability/monitoring/prometheus/shoot"
	sharedcomponent "github.com/gardener/gardener/pkg/component/shared"
	"github.com/gardener/gardener/pkg/utils"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

// DefaultAlertmanager creates a new alertmanager deployer.
func (b *Botanist) DefaultAlertmanager() (alertmanager.Interface, error) {
	var emailReceivers []string
	if monitoring := b.Shoot.GetInfo().Spec.Monitoring; monitoring != nil && monitoring.Alerting != nil {
		emailReceivers = monitoring.Alerting.EmailReceivers
	}

	return sharedcomponent.NewAlertmanager(b.Logger, b.SeedClientSet.Client(), b.Shoot.ControlPlaneNamespace, alertmanager.Values{
		Name:               "shoot",
		ClusterType:        component.ClusterTypeShoot,
		PriorityClassName:  v1beta1constants.PriorityClassNameShootControlPlane100,
		StorageCapacity:    resource.MustParse(b.Seed.GetValidVolumeSize("1Gi")),
		Replicas:           b.Shoot.GetReplicas(1),
		AlertingSMTPSecret: b.LoadSecret(v1beta1constants.GardenRoleAlerting),
		EmailReceivers:     emailReceivers,
		Ingress: &alertmanager.IngressValues{
			Host:           b.ComputeAlertManagerHost(),
			SecretsManager: b.SecretsManager,
			SigningCA:      v1beta1constants.SecretNameCACluster,
		},
	})
}

// DeployAlertManager reconciles the shoot alert manager.
func (b *Botanist) DeployAlertManager(ctx context.Context) error {
	if !b.Shoot.WantsAlertmanager || !b.IsShootMonitoringEnabled() {
		return b.Shoot.Components.ControlPlane.Alertmanager.Destroy(ctx)
	}

	ingressAuthSecret, found := b.SecretsManager.Get(v1beta1constants.SecretNameObservabilityIngressUsers)
	if !found {
		return fmt.Errorf("secret %q not found", v1beta1constants.SecretNameObservabilityIngressUsers)
	}

	b.Shoot.Components.ControlPlane.Alertmanager.SetIngressAuthSecret(ingressAuthSecret)
	b.Shoot.Components.ControlPlane.Alertmanager.SetIngressWildcardCertSecret(b.ControlPlaneWildcardCert)

	return b.Shoot.Components.ControlPlane.Alertmanager.Deploy(ctx)
}

// DefaultPrometheus creates a new prometheus deployer.
func (b *Botanist) DefaultPrometheus() (prometheus.Interface, error) {
	externalLabels := map[string]string{
		"cluster":       b.Shoot.ControlPlaneNamespace,
		"project":       b.Garden.Project.Name,
		"shoot_name":    b.Shoot.GetInfo().Name,
		"name":          b.Shoot.GetInfo().Name,
		"seed_api":      b.SeedClientSet.RESTConfig().Host,
		"seed_region":   b.Seed.GetInfo().Spec.Provider.Region,
		"seed_provider": b.Seed.GetInfo().Spec.Provider.Type,
		"shoot_infra":   b.Shoot.GetInfo().Spec.Provider.Type,
		"ignoreAlerts":  strconv.FormatBool(b.Shoot.IgnoreAlerts),
	}

	if b.Config != nil && b.Config.Monitoring != nil && b.Config.Monitoring.Shoot != nil {
		externalLabels = utils.MergeStringMaps(externalLabels, b.Config.Monitoring.Shoot.ExternalLabels)
	}

	values := prometheus.Values{
		Name:                "shoot",
		PriorityClassName:   v1beta1constants.PriorityClassNameShootControlPlane100,
		StorageCapacity:     resource.MustParse(b.Seed.GetValidVolumeSize("20Gi")),
		ClusterType:         component.ClusterTypeShoot,
		Replicas:            b.Shoot.GetReplicas(1),
		Retention:           ptr.To(monitoringv1.Duration("30d")),
		RetentionSize:       "15GB",
		RestrictToNamespace: true,
		ResourceRequests: &corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("150m"),
			corev1.ResourceMemory: resource.MustParse("400M"),
		},
		AdditionalPodLabels: map[string]string{
			"networking.resources.gardener.cloud/to-" + v1beta1constants.LabelNetworkPolicyScrapeTargets:  v1beta1constants.LabelNetworkPolicyAllowed,
			gardenerutils.NetworkPolicyLabel(v1beta1constants.GardenNamespace+"-prometheus-cache", 9090):  v1beta1constants.LabelNetworkPolicyAllowed,
			gardenerutils.NetworkPolicyLabel(v1beta1constants.GardenNamespace+"-alertmanager-seed", 9093): v1beta1constants.LabelNetworkPolicyAllowed,
		},
		ExternalLabels: externalLabels,
		CentralConfigs: prometheus.CentralConfigs{
			PrometheusRules: shootprometheus.CentralPrometheusRules(b.Shoot.IsWorkerless, b.Shoot.WantsAlertmanager),
			ServiceMonitors: shootprometheus.CentralServiceMonitors(b.Shoot.WantsAlertmanager),
		},
		Alerting: &prometheus.AlertingValues{
			Alertmanagers: []*prometheus.Alertmanager{{
				Name:      "alertmanager-seed",
				Namespace: ptr.To(v1beta1constants.GardenNamespace)}}},
		Ingress: &prometheus.IngressValues{
			Host:                              b.ComputePrometheusHost(),
			SecretsManager:                    b.SecretsManager,
			SigningCA:                         v1beta1constants.SecretNameCACluster,
			BlockManagementAndTargetAPIAccess: true,
		},
		TargetCluster: &prometheus.TargetClusterValues{
			ServiceAccountName: shootprometheus.ServiceAccountName,
			ScrapesMetrics:     true,
		},
		VPAMinAllowed: &corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("150m"),
			corev1.ResourceMemory: resource.MustParse("100M"),
		},
	}

	if b.Shoot.WantsAlertmanager {
		values.Alerting.Alertmanagers = append(values.Alerting.Alertmanagers, &prometheus.Alertmanager{Name: "alertmanager-shoot"})

		if secret := b.LoadSecret(v1beta1constants.GardenRoleAlerting); secret != nil &&
			len(secret.Data["auth_type"]) > 0 &&
			string(secret.Data["auth_type"]) != "smtp" {
			values.Alerting.AdditionalAlertmanager = secret.Data
		}
	}

	if b.Config != nil && b.Config.Monitoring != nil && b.Config.Monitoring.Shoot != nil && b.Config.Monitoring.Shoot.RemoteWrite != nil {
		values.RemoteWrite = &prometheus.RemoteWriteValues{
			URL:                          b.Config.Monitoring.Shoot.RemoteWrite.URL,
			KeptMetrics:                  b.Config.Monitoring.Shoot.RemoteWrite.Keep,
			GlobalShootRemoteWriteSecret: b.LoadSecret(v1beta1constants.GardenRoleGlobalShootRemoteWriteMonitoring),
		}
	}

	return sharedcomponent.NewPrometheus(b.Logger, b.SeedClientSet.Client(), b.Shoot.ControlPlaneNamespace, values)
}

// DeployPrometheus reconciles the shoot Prometheus.
func (b *Botanist) DeployPrometheus(ctx context.Context) error {
	if !b.IsShootMonitoringEnabled() {
		return b.Shoot.Components.ControlPlane.Prometheus.Destroy(ctx)
	}

	if err := gardenerutils.NewShootAccessSecret(shootprometheus.AccessSecretName, b.Shoot.ControlPlaneNamespace).Reconcile(ctx, b.SeedClientSet.Client()); err != nil {
		return fmt.Errorf("failed reconciling access secret for prometheus: %w", err)
	}

	ingressAuthSecret, found := b.SecretsManager.Get(v1beta1constants.SecretNameObservabilityIngressUsers)
	if !found {
		return fmt.Errorf("secret %q not found", v1beta1constants.SecretNameObservabilityIngressUsers)
	}

	b.Shoot.Components.ControlPlane.Prometheus.SetIngressAuthSecret(ingressAuthSecret)
	b.Shoot.Components.ControlPlane.Prometheus.SetIngressWildcardCertSecret(b.ControlPlaneWildcardCert)
	b.Shoot.Components.ControlPlane.Prometheus.SetNamespaceUID(b.SeedNamespaceObject.UID)

	caSecret, found := b.SecretsManager.Get(v1beta1constants.SecretNameCACluster)
	if !found {
		return fmt.Errorf("secret %q not found", v1beta1constants.SecretNameCACluster)
	}
	b.Shoot.Components.ControlPlane.Prometheus.SetCentralScrapeConfigs(shootprometheus.CentralScrapeConfigs(b.Shoot.ControlPlaneNamespace, caSecret.Name, b.Shoot.IsWorkerless))

	return b.Shoot.Components.ControlPlane.Prometheus.Deploy(ctx)
}

// DestroyPrometheus destroys the shoot Prometheus.
func (b *Botanist) DestroyPrometheus(ctx context.Context) error {
	if err := b.Shoot.Components.ControlPlane.Prometheus.Destroy(ctx); err != nil {
		return err
	}

	return kubernetesutils.DeleteObject(ctx, b.SeedClientSet.Client(), gardenerutils.NewShootAccessSecret(shootprometheus.AccessSecretName, b.Shoot.ControlPlaneNamespace).Secret)
}
