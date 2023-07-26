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
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/etcd"
	"github.com/gardener/gardener/pkg/controllerutils"
	gardenletconfig "github.com/gardener/gardener/pkg/gardenlet/apis/config"
	"github.com/gardener/gardener/pkg/utils"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/managedresources"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

var (
	//go:embed charts/seed-monitoring/charts/alertmanager
	chartAlertmanager     embed.FS
	chartPathAlertmanager = filepath.Join("charts", "seed-monitoring", "charts", "alertmanager")

	//go:embed charts/seed-monitoring/charts/core
	chartCore     embed.FS
	chartPathCore = filepath.Join("charts", "seed-monitoring", "charts", "core")

	managedResourceNamePrometheus = "shoot-core-prometheus"
)

// Interface contains functions for a monitoring deployer.
type Interface interface {
	component.Deployer
	// SetNamespaceUID sets the UID of the namespace into which the monitoring components shall be deployed.
	SetNamespaceUID(types.UID)
	// SetComponents sets the monitoring components.
	SetComponents([]component.MonitoringComponent)
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
	// GardenletManagesMCM specifies whether MCM is managed by gardenlet.
	GardenletManagesMCM bool
	// GlobalShootRemoteWriteSecret is the global secret for remote write config.
	GlobalShootRemoteWriteSecret *corev1.Secret
	// IgnoreAlerts specifies whether alerts should be ignored.
	IgnoreAlerts bool
	// ImageAlertmanager is the image of Alertmanager.
	ImageAlertmanager string
	// ImageBlackboxExporter is the image of BlackboxExporter.
	ImageBlackboxExporter string
	// ImageConfigmapReloader is the image of ConfigmapReloader.
	ImageConfigmapReloader string
	// ImagePrometheus is the image of Prometheus.
	ImagePrometheus string
	// IngressHostAlertmanager is the host name of Alertmanager.
	IngressHostAlertmanager string
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
	// StorageCapacityAlertmanager is the storage capacity of Alertmanager.
	StorageCapacityAlertmanager string
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
	credentialsSecret, found := m.secretsManager.Get(v1beta1constants.SecretNameObservabilityIngressUsers)
	if !found {
		return fmt.Errorf("secret %q not found", v1beta1constants.SecretNameObservabilityIngressUsers)
	}

	alerting, err := m.getCustomAlertingConfigs(ctx)
	if err != nil {
		return err
	}

	alertingRules, scrapeConfigs, err := m.getAlertingRulesAndScrapeConfigs(ctx)
	if err != nil {
		return err
	}

	// Create shoot token secret for prometheus component
	shootAccessSecret := m.newShootAccessSecret()
	if err := shootAccessSecret.Reconcile(ctx, m.client); err != nil {
		return err
	}

	var ingressTLSSecretName string
	if m.values.WildcardCertName != nil {
		ingressTLSSecretName = *m.values.WildcardCertName
	} else {
		ingressTLSSecret, err := m.secretsManager.Generate(ctx, &secretsutils.CertificateSecretConfig{
			Name:                        "prometheus-tls",
			CommonName:                  "prometheus",
			Organization:                []string{"gardener.cloud:monitoring:ingress"},
			DNSNames:                    []string{m.values.IngressHostPrometheus},
			CertType:                    secretsutils.ServerCert,
			Validity:                    pointer.Duration(v1beta1constants.IngressTLSCertificateValidity),
			SkipPublishingCACertificate: true,
		}, secretsmanager.SignedByCA(v1beta1constants.SecretNameCACluster))
		if err != nil {
			return err
		}
		ingressTLSSecretName = ingressTLSSecret.Name
	}

	clusterCASecret, found := m.secretsManager.Get(v1beta1constants.SecretNameCACluster)
	if !found {
		return fmt.Errorf("secret %q not found", v1beta1constants.SecretNameCACluster)
	}

	etcdCASecret, found := m.secretsManager.Get(v1beta1constants.SecretNameCAETCD)
	if !found {
		return fmt.Errorf("secret %q not found", v1beta1constants.SecretNameCAETCD)
	}

	etcdClientSecret, found := m.secretsManager.Get(etcd.SecretNameClient)
	if !found {
		return fmt.Errorf("secret %q not found", etcd.SecretNameClient)
	}

	var (
		networks         = map[string]interface{}{}
		prometheusConfig = map[string]interface{}{
			"images": map[string]string{
				"blackbox-exporter":  m.values.ImageBlackboxExporter,
				"configmap-reloader": m.values.ImageConfigmapReloader,
				"prometheus":         m.values.ImagePrometheus,
			},
			"secretNameClusterCA":      clusterCASecret.Name,
			"secretNameEtcdCA":         etcdCASecret.Name,
			"secretNameEtcdClientCert": etcdClientSecret.Name,
			"kubernetesVersion":        m.values.KubernetesVersion,
			"nodeLocalDNS": map[string]interface{}{
				"enabled": m.values.NodeLocalDNSEnabled,
			},
			"gardenletManagesMCM": m.values.GardenletManagesMCM,
			"ingress": map[string]interface{}{
				"class":          v1beta1constants.SeedNginxIngressClass,
				"authSecretName": credentialsSecret.Name,
				"hosts": []map[string]interface{}{
					{
						"hostName":   m.values.IngressHostPrometheus,
						"secretName": ingressTLSSecretName,
					},
				},
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
			"alerting":                alerting,
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

	// Add remotewrite to prometheus when enabled
	if m.values.Config != nil &&
		m.values.Config.Shoot != nil &&
		m.values.Config.Shoot.RemoteWrite != nil &&
		m.values.Config.Shoot.RemoteWrite.URL != "" {
		// if remoteWrite Url is set add config into values
		remoteWriteConfig := map[string]interface{}{
			"url": m.values.Config.Shoot.RemoteWrite.URL,
		}
		// get secret for basic_auth in remote write
		if remoteWriteBasicAuth := m.values.GlobalShootRemoteWriteSecret; remoteWriteBasicAuth != nil {
			remoteWriteUsername := string(remoteWriteBasicAuth.Data["username"])
			remoteWritePassword := string(remoteWriteBasicAuth.Data["password"])
			if remoteWriteUsername != "" &&
				remoteWritePassword != "" {
				remoteWriteConfig["basic_auth"] = map[string]interface{}{
					"username": remoteWriteUsername,
					"password": remoteWritePassword,
				}
			}
		}
		// add list with keep metrics if set
		if len(m.values.Config.Shoot.RemoteWrite.Keep) != 0 {
			remoteWriteConfig["keep"] = m.values.Config.Shoot.RemoteWrite.Keep
		}
		// add queue_config if set
		if m.values.Config.Shoot.RemoteWrite.QueueConfig != nil &&
			len(*m.values.Config.Shoot.RemoteWrite.QueueConfig) != 0 {
			remoteWriteConfig["queue_config"] = m.values.Config.Shoot.RemoteWrite.QueueConfig
		}
		prometheusConfig["remoteWrite"] = remoteWriteConfig
	}

	// set externalLabels
	if m.values.Config != nil && m.values.Config.Shoot != nil && len(m.values.Config.Shoot.ExternalLabels) != 0 {
		prometheusConfig["externalLabels"] = m.values.Config.Shoot.ExternalLabels
	}

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

	if err := m.reconcilePrometheusShootResources(ctx, shootAccessSecret.ServiceAccountName); err != nil {
		return err
	}

	// Check if we want to deploy an alertmanager into the shoot namespace.
	if m.values.AlertmanagerEnabled {
		var emailConfigs []map[string]interface{}
		if m.values.MonitoringConfig != nil && m.values.MonitoringConfig.Alerting != nil {
			for _, email := range m.values.MonitoringConfig.Alerting.EmailReceivers {
				for _, secret := range m.values.AlertingSecrets {
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
		if m.values.WildcardCertName != nil {
			alertManagerIngressTLSSecretName = *m.values.WildcardCertName
		} else {
			ingressTLSSecret, err := m.secretsManager.Generate(ctx, &secretsutils.CertificateSecretConfig{
				Name:                        "alertmanager-tls",
				CommonName:                  "alertmanager",
				Organization:                []string{"gardener.cloud:monitoring:ingress"},
				DNSNames:                    []string{m.values.IngressHostAlertmanager},
				CertType:                    secretsutils.ServerCert,
				Validity:                    pointer.Duration(v1beta1constants.IngressTLSCertificateValidity),
				SkipPublishingCACertificate: true,
			}, secretsmanager.SignedByCA(v1beta1constants.SecretNameCACluster))
			if err != nil {
				return err
			}
			alertManagerIngressTLSSecretName = ingressTLSSecret.Name
		}

		alertManagerValues := map[string]interface{}{
			"images": map[string]string{
				"alertmanager":       m.values.ImageAlertmanager,
				"configmap-reloader": m.values.ImageConfigmapReloader,
			},
			"ingress": map[string]interface{}{
				"class":          v1beta1constants.SeedNginxIngressClass,
				"authSecretName": credentialsSecret.Name,
				"hosts": []map[string]interface{}{
					{
						"hostName":   m.values.IngressHostAlertmanager,
						"secretName": alertManagerIngressTLSSecretName,
					},
				},
			},
			"replicas":     m.values.Replicas,
			"storage":      m.values.StorageCapacityAlertmanager,
			"emailConfigs": emailConfigs,
		}

		return m.chartApplier.ApplyFromEmbeddedFS(ctx, chartAlertmanager, chartPathAlertmanager, m.namespace, "alertmanager", kubernetes.Values(alertManagerValues))
	}

	return deleteAlertmanager(ctx, m.client, m.namespace)
}

func (m *monitoring) Destroy(ctx context.Context) error {
	if err := deleteAlertmanager(ctx, m.client, m.namespace); err != nil {
		return err
	}

	if err := managedresources.DeleteForShoot(ctx, m.client, m.namespace, managedResourceNamePrometheus); err != nil {
		return err
	}

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
		&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: m.namespace,
				Name:      "blackbox-exporter-config-prometheus",
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

func (m *monitoring) newShootAccessSecret() *gardenerutils.AccessSecret {
	return gardenerutils.NewShootAccessSecret(v1beta1constants.StatefulSetNamePrometheus, m.namespace)
}

func (m *monitoring) reconcilePrometheusShootResources(ctx context.Context, serviceAccountName string) error {
	var (
		registry = managedresources.NewRegistry(kubernetes.ShootScheme, kubernetes.ShootCodec, kubernetes.ShootSerializer)

		clusterRole = &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: "gardener.cloud:monitoring:prometheus-target",
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{corev1.GroupName},
					Resources: []string{"nodes", "services", "endpoints", "pods"},
					Verbs:     []string{"get", "list", "watch"},
				},
				{
					APIGroups: []string{corev1.GroupName},
					Resources: []string{"nodes/metrics", "pods/log", "nodes/proxy", "services/proxy", "pods/proxy"},
					Verbs:     []string{"get"},
				},
				{
					NonResourceURLs: []string{"/metrics"},
					Verbs:           []string{"get"},
				},
			},
		}
		clusterRoleBinding = &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "gardener.cloud:monitoring:prometheus-target",
				Annotations: map[string]string{resourcesv1alpha1.DeleteOnInvalidUpdate: "true"},
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: rbacv1.GroupName,
				Kind:     "ClusterRole",
				Name:     clusterRole.Name,
			},
			Subjects: []rbacv1.Subject{{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      serviceAccountName,
				Namespace: metav1.NamespaceSystem,
			}},
		}
	)

	data, err := registry.AddAllAndSerialize(clusterRole, clusterRoleBinding)
	if err != nil {
		return err
	}

	return managedresources.CreateForShoot(ctx, m.client, m.namespace, managedResourceNamePrometheus, managedresources.LabelValueGardener, false, data)
}

func (m *monitoring) getCustomAlertingConfigs(ctx context.Context) (map[string]interface{}, error) {
	configs := map[string]interface{}{
		"auth_type": map[string]interface{}{},
	}

	for _, secret := range m.values.AlertingSecrets {
		if string(secret.Data["auth_type"]) == "none" {

			if url, ok := secret.Data["url"]; ok {
				configs["auth_type"] = map[string]interface{}{
					"none": map[string]interface{}{
						"url": string(url),
					},
				}
			}
			break
		}

		if string(secret.Data["auth_type"]) == "basic" {
			url, urlOk := secret.Data["url"]
			username, usernameOk := secret.Data["username"]
			password, passwordOk := secret.Data["password"]

			if urlOk && usernameOk && passwordOk {
				configs["auth_type"] = map[string]interface{}{
					"basic": map[string]interface{}{
						"url":      string(url),
						"username": string(username),
						"password": string(password),
					},
				}
			}
			break
		}

		if string(secret.Data["auth_type"]) == "certificate" {
			data := map[string][]byte{}
			url, urlOk := secret.Data["url"]
			ca, caOk := secret.Data["ca.crt"]
			cert, certOk := secret.Data["tls.crt"]
			key, keyOk := secret.Data["tls.key"]
			insecure, insecureOk := secret.Data["insecure_skip_verify"]

			if urlOk && caOk && certOk && keyOk && insecureOk {
				configs["auth_type"] = map[string]interface{}{
					"certificate": map[string]interface{}{
						"url":                  string(url),
						"insecure_skip_verify": string(insecure),
					},
				}
				data["ca.crt"] = ca
				data["tls.crt"] = cert
				data["tls.key"] = key
				amSecret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "prometheus-remote-am-tls",
						Namespace: m.namespace,
					},
				}

				if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, m.client, amSecret, func() error {
					amSecret.Data = data
					amSecret.Type = corev1.SecretTypeOpaque
					return nil
				}); err != nil {
					return nil, err
				}
			}
			break
		}
	}

	return configs, nil
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
