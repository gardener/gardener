// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package monitoring

import (
	"context"
	"embed"
	"fmt"
	"path/filepath"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	gardenletconfig "github.com/gardener/gardener/pkg/gardenlet/apis/config"
	"github.com/gardener/gardener/pkg/utils"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

var (
	//go:embed charts/seed-monitoring/charts/core
	chartCore     embed.FS
	chartPathCore = filepath.Join("charts", "seed-monitoring", "charts", "core")
)

// Interface contains functions for a monitoring deployer.
type Interface interface {
	component.Deployer
	// SetNamespaceUID sets the UID of the namespace into which the monitoring components shall be deployed.
	SetNamespaceUID(types.UID)
	// SetComponents sets the monitoring components.
	SetComponents([]component.MonitoringComponent)
	// SetWildcardCertName sets the WildcardCertName components.
	SetWildcardCertName(*string)
}

// Values is a set of configuration values for the monitoring components.
type Values struct {
	// AlertingSecrets is a list of alerting secrets.
	AlertingSecrets []*corev1.Secret
	// AlertmanagerEnabled specifies whether Alertmanager is enabled.
	AlertmanagerEnabled bool
	// APIServerDomain is the domain of the API server.
	APIServerDomain string
	// APIServerHost is the host of the API server.
	APIServerHost string
	// APIServerServiceIP is the service IP of the API server.
	APIServerServiceIP *string
	// Components is a list of monitoring components.
	Components []component.MonitoringComponent
	// Config is the monitoring config.
	Config *gardenletconfig.MonitoringConfig
	// GlobalShootRemoteWriteSecret is the global secret for remote write config.
	GlobalShootRemoteWriteSecret *corev1.Secret
	// IgnoreAlerts specifies whether alerts should be ignored.
	IgnoreAlerts bool
	// ImageBlackboxExporter is the image of BlackboxExporter.
	ImageBlackboxExporter string
	// ImageConfigmapReloader is the image of ConfigmapReloader.
	ImageConfigmapReloader string
	// ImagePrometheus is the image of Prometheus.
	ImagePrometheus string
	// IngressHostPrometheus is the host name of Prometheus.
	IngressHostPrometheus string
	// IsWorkerless specifies whether the cluster is workerless.
	IsWorkerless bool
	// KubernetesVersion is the Kubernetes version of the target cluster.
	KubernetesVersion string
	// MonitoringConfig is the monitoring config.
	MonitoringConfig *gardencorev1beta1.Monitoring
	// NamespaceUID is the UID of the namespace in the runtime cluster.
	NamespaceUID types.UID
	// NodeLocalDNSEnabled specifies whether node-local-dns is enabled.
	NodeLocalDNSEnabled bool
	// ProjectName is the name of the project.
	ProjectName string
	// PodNetworkCIDR is the CIDR of the pod network.
	PodNetworkCIDR *string
	// ServiceNetworkCIDR is the CIDR of the service network.
	ServiceNetworkCIDR *string
	// NodeNetworkCIDR is the CIDR of the node network.
	NodeNetworkCIDR *string
	// Replicas is the number of replicas.
	Replicas int32
	// RuntimeProviderType is the provider type of the runtime cluster.
	RuntimeProviderType string
	// RuntimeRegion is the region of the runtime cluster.
	RuntimeRegion string
	// TargetName is the name of the target cluster.
	TargetName string
	// TargetProviderType is the provider type of the target cluster.
	TargetProviderType string
	// WildcardCertName is name of wildcard tls certificate which is issued for the seed's ingress domain.
	WildcardCertName *string
}

// New creates a new instance of Interface for the monitoring components.
func New(
	client client.Client,
	chartApplier kubernetes.ChartApplier,
	secretsManager secretsmanager.Interface,
	namespace string,
	values Values,
) Interface {
	return &monitoring{
		client:         client,
		chartApplier:   chartApplier,
		namespace:      namespace,
		secretsManager: secretsManager,
		values:         values,
	}
}

type monitoring struct {
	client         client.Client
	chartApplier   kubernetes.ChartApplier
	namespace      string
	secretsManager secretsmanager.Interface
	values         Values
}

func (m *monitoring) Deploy(ctx context.Context) error {
	alertingRules, scrapeConfigs, err := m.getAlertingRulesAndScrapeConfigs(ctx)
	if err != nil {
		return err
	}

	var (
		networks         = map[string]interface{}{}
		prometheusConfig = map[string]interface{}{
			"images": map[string]string{
				"configmap-reloader": m.values.ImageConfigmapReloader,
				"prometheus":         m.values.ImagePrometheus,
			},
			"kubernetesVersion": m.values.KubernetesVersion,
			"nodeLocalDNS": map[string]interface{}{
				"enabled": m.values.NodeLocalDNSEnabled,
			},
			"namespace": map[string]interface{}{
				"uid": m.values.NamespaceUID,
			},
			"replicas": m.values.Replicas,
			"seed": map[string]interface{}{
				"apiserver": m.values.APIServerHost,
				"region":    m.values.RuntimeRegion,
				"provider":  m.values.RuntimeProviderType,
			},
			"rules": map[string]interface{}{
				"optional": map[string]interface{}{
					"alertmanager": map[string]interface{}{
						"enabled": m.values.AlertmanagerEnabled,
					},
				},
			},
			"shoot": map[string]interface{}{
				"apiserver":           fmt.Sprintf("https://%s", m.values.APIServerDomain),
				"apiserverServerName": m.values.APIServerDomain,
				"provider":            m.values.TargetProviderType,
				"name":                m.values.TargetName,
				"project":             m.values.ProjectName,
				"workerless":          m.values.IsWorkerless,
			},
			"ignoreAlerts":            m.values.IgnoreAlerts,
			"additionalRules":         alertingRules.String(),
			"additionalScrapeConfigs": scrapeConfigs.String(),
		}
	)

	if services := m.values.ServiceNetworkCIDR; services != nil {
		networks["services"] = services
	}
	if pods := m.values.PodNetworkCIDR; pods != nil {
		networks["pods"] = pods
	}
	if apiServer := m.values.APIServerServiceIP; apiServer != nil {
		prometheusConfig["apiserverServiceIP"] = apiServer
	}
	if m.values.NodeNetworkCIDR != nil {
		networks["nodes"] = *m.values.NodeNetworkCIDR
	}

	prometheusConfig["networks"] = networks

	coreValues := map[string]interface{}{
		"global": map[string]interface{}{
			"shootKubeVersion": map[string]interface{}{
				"gitVersion": m.values.KubernetesVersion,
			},
		},
		"prometheus": prometheusConfig,
	}

	if err := m.chartApplier.ApplyFromEmbeddedFS(ctx, chartCore, chartPathCore, m.namespace, "core", kubernetes.Values(coreValues)); err != nil {
		return err
	}

	return nil
}

func (m *monitoring) Destroy(ctx context.Context) error {
	objects := []client.Object{
		&networkingv1.NetworkPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: m.namespace,
				Name:      "allow-from-prometheus",
			},
		},
		&networkingv1.NetworkPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: m.namespace,
				Name:      "allow-prometheus",
			},
		},
		m.newShootAccessSecret().Secret,
		&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: m.namespace,
				Name:      "prometheus-config",
			},
		},
		&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: m.namespace,
				Name:      "prometheus-rules",
			},
		},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: m.namespace,
				Name:      "prometheus-basic-auth",
			},
		},
		&networkingv1.Ingress{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: m.namespace,
				Name:      "prometheus",
			},
		},
		&vpaautoscalingv1.VerticalPodAutoscaler{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: m.namespace,
				Name:      "prometheus-vpa",
			},
		},
		&corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: m.namespace,
				Name:      "prometheus",
			},
		},
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: m.namespace,
				Name:      "prometheus-web",
			},
		},
		&appsv1.StatefulSet{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: m.namespace,
				Name:      "prometheus",
			},
		},
		&rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: m.namespace,
				Name:      "prometheus-" + m.namespace,
			},
		},
		&corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: m.namespace,
				Name:      "prometheus-db-prometheus-0",
			},
		},
	}

	return kubernetesutils.DeleteObjects(ctx, m.client, objects...)
}

func (m *monitoring) SetNamespaceUID(uid types.UID)                   { m.values.NamespaceUID = uid }
func (m *monitoring) SetComponents(c []component.MonitoringComponent) { m.values.Components = c }
func (m *monitoring) SetWildcardCertName(secretName *string)          { m.values.WildcardCertName = secretName }

func (m *monitoring) newShootAccessSecret() *gardenerutils.AccessSecret {
	return gardenerutils.NewShootAccessSecret(v1beta1constants.StatefulSetNamePrometheus, m.namespace)
}

func (m *monitoring) getAlertingRulesAndScrapeConfigs(ctx context.Context) (alertingRules, scrapeConfigs strings.Builder, err error) {
	for _, component := range m.values.Components {
		componentsScrapeConfigs, err := component.ScrapeConfigs()
		if err != nil {
			return alertingRules, scrapeConfigs, err
		}
		for _, config := range componentsScrapeConfigs {
			scrapeConfigs.WriteString(fmt.Sprintf("- %s\n", utils.Indent(config, 2)))
		}

		componentsAlertingRules, err := component.AlertingRules()
		if err != nil {
			return alertingRules, scrapeConfigs, err
		}
		for filename, rule := range componentsAlertingRules {
			alertingRules.WriteString(fmt.Sprintf("%s: |\n  %s\n", filename, utils.Indent(rule, 2)))
		}
	}

	// Fetch extensions provider-specific monitoring configuration
	existingConfigMaps := &corev1.ConfigMapList{}
	if err := m.client.List(ctx, existingConfigMaps,
		client.InNamespace(m.namespace),
		client.MatchingLabels{v1beta1constants.LabelExtensionConfiguration: v1beta1constants.LabelMonitoring}); err != nil {
		return alertingRules, scrapeConfigs, err
	}

	// Need stable order before passing the dashboards to Prometheus config to avoid unnecessary changes
	kubernetesutils.ByName().Sort(existingConfigMaps)

	// Read extension monitoring configurations
	for _, cm := range existingConfigMaps.Items {
		alertingRules.WriteString(fmt.Sprintln(cm.Data[v1beta1constants.PrometheusConfigMapAlertingRules]))
		scrapeConfigs.WriteString(fmt.Sprintln(cm.Data[v1beta1constants.PrometheusConfigMapScrapeConfig]))
	}

	return
}
