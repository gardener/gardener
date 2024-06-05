// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/observability/monitoring"
	"github.com/gardener/gardener/pkg/component/observability/monitoring/alertmanager"
	"github.com/gardener/gardener/pkg/component/observability/monitoring/prometheus"
	shootprometheus "github.com/gardener/gardener/pkg/component/observability/monitoring/prometheus/shoot"
	monitoringutils "github.com/gardener/gardener/pkg/component/observability/monitoring/utils"
	sharedcomponent "github.com/gardener/gardener/pkg/component/shared"
	gardenlethelper "github.com/gardener/gardener/pkg/gardenlet/apis/config/helper"
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

	return sharedcomponent.NewAlertmanager(b.Logger, b.SeedClientSet.Client(), b.Shoot.SeedNamespace, alertmanager.Values{
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
		return b.Shoot.Components.Monitoring.Alertmanager.Destroy(ctx)
	}

	ingressAuthSecret, found := b.SecretsManager.Get(v1beta1constants.SecretNameObservabilityIngressUsers)
	if !found {
		return fmt.Errorf("secret %q not found", v1beta1constants.SecretNameObservabilityIngressUsers)
	}

	b.Shoot.Components.Monitoring.Alertmanager.SetIngressAuthSecret(ingressAuthSecret)
	b.Shoot.Components.Monitoring.Alertmanager.SetIngressWildcardCertSecret(b.ControlPlaneWildcardCert)

	return b.Shoot.Components.Monitoring.Alertmanager.Deploy(ctx)
}

// DefaultPrometheus creates a new prometheus deployer.
func (b *Botanist) DefaultPrometheus() (prometheus.Interface, error) {
	externalLabels := map[string]string{
		"cluster":       b.Shoot.SeedNamespace,
		"project":       b.Garden.Project.Name,
		"shoot_name":    b.Shoot.GetInfo().Name,
		"name":          b.Shoot.GetInfo().Name,
		"seed_api":      b.SeedClientSet.RESTConfig().Host,
		"seed_region":   b.Seed.GetInfo().Spec.Provider.Region,
		"seed_provider": b.Seed.GetInfo().Spec.Provider.Type,
		"shoot_infra":   b.Shoot.GetInfo().Spec.Provider.Type,
		"ignoreAlerts":  strconv.FormatBool(b.Shoot.IgnoreAlerts),
	}

	if b.Config.Monitoring != nil && b.Config.Monitoring.Shoot != nil {
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
		DataMigration: monitoring.DataMigration{
			StatefulSetName: "prometheus",
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

	if b.Config.Monitoring != nil && b.Config.Monitoring.Shoot != nil && b.Config.Monitoring.Shoot.RemoteWrite != nil {
		values.RemoteWrite = &prometheus.RemoteWriteValues{
			URL:                          b.Config.Monitoring.Shoot.RemoteWrite.URL,
			KeptMetrics:                  b.Config.Monitoring.Shoot.RemoteWrite.Keep,
			GlobalShootRemoteWriteSecret: b.LoadSecret(v1beta1constants.GardenRoleGlobalShootRemoteWriteMonitoring),
		}
	}

	return sharedcomponent.NewPrometheus(b.Logger, b.SeedClientSet.Client(), b.Shoot.SeedNamespace, values)
}

// MigratePrometheus migrate the shoot Prometheus to prometheus-operator.
// TODO(rfranzke): Remove this function after v1.97 has been released.
func (b *Botanist) MigratePrometheus(ctx context.Context) error {
	oldStatefulSet := &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: "prometheus", Namespace: b.Shoot.SeedNamespace}}
	if err := b.SeedClientSet.Client().Get(ctx, client.ObjectKeyFromObject(oldStatefulSet), oldStatefulSet); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("failed reading old Prometheus StatefulSet %s: %w", client.ObjectKeyFromObject(oldStatefulSet), err)
	}

	if err := b.DeployPrometheus(ctx); err != nil {
		return err
	}

	return b.ReconcileBlackboxExporterControlPlane(ctx)
}

// DeployPrometheus reconciles the shoot Prometheus.
func (b *Botanist) DeployPrometheus(ctx context.Context) error {
	if !b.IsShootMonitoringEnabled() {
		return b.Shoot.Components.Monitoring.Prometheus.Destroy(ctx)
	}

	if err := gardenerutils.NewShootAccessSecret(shootprometheus.AccessSecretName, b.Shoot.SeedNamespace).Reconcile(ctx, b.SeedClientSet.Client()); err != nil {
		return fmt.Errorf("failed reconciling access secret for prometheus: %w", err)
	}

	ingressAuthSecret, found := b.SecretsManager.Get(v1beta1constants.SecretNameObservabilityIngressUsers)
	if !found {
		return fmt.Errorf("secret %q not found", v1beta1constants.SecretNameObservabilityIngressUsers)
	}

	b.Shoot.Components.Monitoring.Prometheus.SetIngressAuthSecret(ingressAuthSecret)
	b.Shoot.Components.Monitoring.Prometheus.SetIngressWildcardCertSecret(b.ControlPlaneWildcardCert)
	b.Shoot.Components.Monitoring.Prometheus.SetNamespaceUID(b.SeedNamespaceObject.UID)

	caSecret, found := b.SecretsManager.Get(v1beta1constants.SecretNameCACluster)
	if !found {
		return fmt.Errorf("secret %q not found", v1beta1constants.SecretNameCACluster)
	}
	b.Shoot.Components.Monitoring.Prometheus.SetCentralScrapeConfigs(shootprometheus.CentralScrapeConfigs(b.Shoot.SeedNamespace, caSecret.Name, b.Shoot.IsWorkerless))

	// TODO(rfranzke): Remove this block after v1.100 got released.
	{
		prometheusRule, rawScrapeConfigs, err := b.getPrometheusRuleAndRawScrapeConfigs(ctx)
		if err != nil {
			return err
		}
		b.Shoot.Components.Monitoring.Prometheus.SetAdditionalScrapeConfigs(rawScrapeConfigs)
		b.Shoot.Components.Monitoring.Prometheus.SetAdditionalResources(prometheusRule)
	}

	if err := b.Shoot.Components.Monitoring.Prometheus.Deploy(ctx); err != nil {
		return err
	}

	// TODO(rfranzke): Remove this after v1.97 has been released.
	return kubernetesutils.DeleteObjects(ctx, b.SeedClientSet.Client(),
		&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "blackbox-exporter-config-prometheus", Namespace: b.Shoot.SeedNamespace}},
		&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "prometheus-config", Namespace: b.Shoot.SeedNamespace}},
		&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "prometheus-rules", Namespace: b.Shoot.SeedNamespace}},
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "shoot-access-prometheus", Namespace: b.Shoot.SeedNamespace}},
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "prometheus-remote-am-tls", Namespace: b.Shoot.SeedNamespace}},
		&networkingv1.Ingress{ObjectMeta: metav1.ObjectMeta{Name: "prometheus", Namespace: b.Shoot.SeedNamespace}},
		&corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "prometheus", Namespace: b.Shoot.SeedNamespace}},
		&corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "prometheus-web", Namespace: b.Shoot.SeedNamespace}},
		&appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: "prometheus", Namespace: b.Shoot.SeedNamespace}},
		&vpaautoscalingv1.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Name: "prometheus-vpa", Namespace: b.Shoot.SeedNamespace}},
		&rbacv1.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{Name: "prometheus-" + b.Shoot.SeedNamespace, Namespace: b.Shoot.SeedNamespace}},
		&corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "prometheus-db-prometheus-0", Namespace: b.Shoot.SeedNamespace}},
		&resourcesv1alpha1.ManagedResource{ObjectMeta: metav1.ObjectMeta{Name: "shoot-core-prometheus", Namespace: b.Shoot.SeedNamespace}},
	)
}

// DestroyPrometheus destroys the shoot Prometheus.
func (b *Botanist) DestroyPrometheus(ctx context.Context) error {
	if err := b.Shoot.Components.Monitoring.Prometheus.Destroy(ctx); err != nil {
		return err
	}

	return kubernetesutils.DeleteObject(ctx, b.SeedClientSet.Client(), gardenerutils.NewShootAccessSecret(shootprometheus.AccessSecretName, b.Shoot.SeedNamespace).Secret)
}

// TODO(rfranzke): Remove this function after v1.100 has been released.
func (b *Botanist) getPrometheusRuleAndRawScrapeConfigs(ctx context.Context) (prometheusRule *monitoringv1.PrometheusRule, rawScrapeConfigs []string, err error) {
	var rawAlertingRules []string

	for _, component := range b.getMonitoringComponents() {
		componentsScrapeConfigs, err := component.ScrapeConfigs()
		if err != nil {
			return prometheusRule, rawScrapeConfigs, err
		}
		rawScrapeConfigs = append(rawScrapeConfigs, componentsScrapeConfigs...)

		componentsAlertingRules, err := component.AlertingRules()
		if err != nil {
			return prometheusRule, rawScrapeConfigs, err
		}
		for _, rule := range componentsAlertingRules {
			rawAlertingRules = append(rawAlertingRules, rule)
		}
	}

	// Fetch extensions provider-specific monitoring configuration
	existingConfigMaps := &corev1.ConfigMapList{}
	if err := b.SeedClientSet.Client().List(ctx, existingConfigMaps,
		client.InNamespace(b.Shoot.SeedNamespace),
		client.MatchingLabels{v1beta1constants.LabelExtensionConfiguration: v1beta1constants.LabelMonitoring}); err != nil {
		return prometheusRule, rawScrapeConfigs, err
	}

	// Need stable order before passing the dashboards to Prometheus config to avoid unnecessary changes
	kubernetesutils.ByName().Sort(existingConfigMaps)

	// Read extension monitoring configurations
	for _, cm := range existingConfigMaps.Items {
		if len(cm.Data[v1beta1constants.PrometheusConfigMapAlertingRules]) > 0 {
			rawAlertingRules = append(rawAlertingRules, normalizeAlertingRules(fmt.Sprintln(cm.Data[v1beta1constants.PrometheusConfigMapAlertingRules]))...)
		}
		if len(cm.Data[v1beta1constants.PrometheusConfigMapScrapeConfig]) > 0 {
			rawScrapeConfigs = append(rawScrapeConfigs, normalizeScrapeConfigs(fmt.Sprintln(cm.Data[v1beta1constants.PrometheusConfigMapScrapeConfig]))...)
		}
	}

	rawPrometheusRule := `apiVersion: monitoring.coreos.com/v1
kind: PrometheusRule
metadata:
  name: shoot-additional-rules
  namespace: ` + b.Shoot.SeedNamespace + `
  labels:
    prometheus: shoot
spec:
  groups:
`

	for _, group := range rawAlertingRules {
		for _, line := range strings.Split(group, "\n") {
			if line != "groups:" {
				rawPrometheusRule += "  " + line + "\n"
			}
		}
	}

	prometheusRule = &monitoringv1.PrometheusRule{}
	if err := runtime.DecodeInto(monitoringutils.Decoder, []byte(rawPrometheusRule), prometheusRule); err != nil {
		return prometheusRule, rawScrapeConfigs, err
	}

	return
}

func (b *Botanist) getMonitoringComponents() []component.MonitoringComponent {
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
			b.Shoot.Components.ControlPlane.KubeScheduler,
			b.Shoot.Components.ControlPlane.MachineControllerManager,
			b.Shoot.Components.ControlPlane.VPNSeedServer,
			b.Shoot.Components.SystemComponents.BlackboxExporter,
			b.Shoot.Components.SystemComponents.CoreDNS,
			b.Shoot.Components.SystemComponents.KubeProxy,
			b.Shoot.Components.SystemComponents.NodeExporter,
			b.Shoot.Components.SystemComponents.VPNShoot,
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
	}

	return monitoringComponents
}

var alertingRuleHeaderLineRegexp = regexp.MustCompile(`^\S+: \|`)

// TODO(rfranzke): Remove this function after v1.100 has been released.
func normalizeAlertingRules(input string) []string {
	var cleanedLines []string
	for _, line := range strings.Split(input, "\n") {
		if alertingRuleHeaderLineRegexp.MatchString(line) {
			continue
		}
		if len(line) >= 2 && line[:2] == "  " {
			cleanedLines = append(cleanedLines, line[2:])
		} else {
			cleanedLines = append(cleanedLines, line)
		}
	}

	var blocks []string
	for _, group := range strings.Split(strings.Join(cleanedLines, "\n"), "groups:") {
		group = strings.TrimSpace(group)
		if group != "" {
			blocks = append(blocks, "groups:\n"+group)
		}
	}

	return blocks
}

// TODO(rfranzke): Remove this function after v1.100 has been released.
func normalizeScrapeConfigs(input string) []string {
	cleanLines := func(lines []string) []string {
		var cleanedLines []string
		for _, line := range lines {
			line = strings.TrimPrefix(line, "- ")
			if len(line) >= 2 && line[:2] == "  " {
				cleanedLines = append(cleanedLines, line[2:])
			} else {
				cleanedLines = append(cleanedLines, line)
			}
		}
		return cleanedLines
	}

	var (
		cleanedBlocks   []string
		blockStartIndex = 0
		lines           = strings.Split(input, "\n")
	)

	for i := 0; i < len(lines); i++ {
		if strings.HasPrefix(lines[i], "- ") {
			if i > blockStartIndex {
				block := strings.Join(cleanLines(lines[blockStartIndex:i]), "\n")
				cleanedBlocks = append(cleanedBlocks, block)
			}
			blockStartIndex = i
		}
	}

	if blockStartIndex < len(lines) {
		block := strings.Join(cleanLines(lines[blockStartIndex:]), "\n")
		cleanedBlocks = append(cleanedBlocks, block)
	}

	return cleanedBlocks
}
