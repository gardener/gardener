// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"strings"

	"github.com/gardener/gardener/charts"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/features"
	gardenletfeatures "github.com/gardener/gardener/pkg/gardenlet/features"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/utils"
	gutil "github.com/gardener/gardener/pkg/utils/gardener"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/secrets"
	versionutils "github.com/gardener/gardener/pkg/utils/version"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	autoscalingv1beta2 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// DeploySeedMonitoring installs the Helm release "seed-monitoring" in the Seed clusters. It comprises components
// to monitor the Shoot cluster whose control plane runs in the Seed cluster.
func (b *Botanist) DeploySeedMonitoring(ctx context.Context) error {
	if b.Shoot.Purpose == gardencorev1beta1.ShootPurposeTesting {
		return b.DeleteSeedMonitoring(ctx)
	}

	var (
		credentials      = b.LoadSecret(common.MonitoringIngressCredentials)
		credentialsUsers = b.LoadSecret(common.MonitoringIngressCredentialsUsers)
		basicAuth        = utils.CreateSHA1Secret(credentials.Data[secrets.DataKeyUserName], credentials.Data[secrets.DataKeyPassword])
		basicAuthUsers   = utils.CreateSHA1Secret(credentialsUsers.Data[secrets.DataKeyUserName], credentialsUsers.Data[secrets.DataKeyPassword])
		alertingRules    = strings.Builder{}
		scrapeConfigs    = strings.Builder{}
	)

	// Fetch component-specific monitoring configuration
	monitoringComponents := []component.MonitoringComponent{
		b.Shoot.Components.ControlPlane.EtcdMain,
		b.Shoot.Components.ControlPlane.EtcdEvents,
		b.Shoot.Components.ControlPlane.KubeAPIServer,
		b.Shoot.Components.ControlPlane.KubeScheduler,
		b.Shoot.Components.ControlPlane.KubeControllerManager,
		b.Shoot.Components.SystemComponents.CoreDNS,
		b.Shoot.Components.SystemComponents.VPNShoot,
	}

	if b.Shoot.WantsClusterAutoscaler {
		monitoringComponents = append(monitoringComponents, b.Shoot.Components.ControlPlane.ClusterAutoscaler)
	}

	for _, component := range monitoringComponents {
		componentsScrapeConfigs, err := component.ScrapeConfigs()
		if err != nil {
			return err
		}
		for _, config := range componentsScrapeConfigs {
			scrapeConfigs.WriteString(fmt.Sprintf("- %s\n", utils.Indent(config, 2)))
		}

		componentsAlertingRules, err := component.AlertingRules()
		if err != nil {
			return err
		}
		for filename, rule := range componentsAlertingRules {
			alertingRules.WriteString(fmt.Sprintf("%s: |\n  %s\n", filename, utils.Indent(rule, 2)))
		}
	}

	// Fetch extensions provider-specific monitoring configuration
	existingConfigMaps := &corev1.ConfigMapList{}
	if err := b.K8sSeedClient.Client().List(ctx, existingConfigMaps,
		client.InNamespace(b.Shoot.SeedNamespace),
		client.MatchingLabels{v1beta1constants.LabelExtensionConfiguration: v1beta1constants.LabelMonitoring}); err != nil {
		return err
	}

	// Need stable order before passing the dashboards to Grafana config to avoid unnecessary changes
	kutil.ByName().Sort(existingConfigMaps)

	// Read extension monitoring configurations
	for _, cm := range existingConfigMaps.Items {
		alertingRules.WriteString(fmt.Sprintln(cm.Data[v1beta1constants.PrometheusConfigMapAlertingRules]))
		scrapeConfigs.WriteString(fmt.Sprintln(cm.Data[v1beta1constants.PrometheusConfigMapScrapeConfig]))
	}

	// Create shoot token secret for kube-state-metrics and prometheus components
	for _, name := range []string{
		v1beta1constants.DeploymentNameKubeStateMetricsShoot,
		v1beta1constants.StatefulSetNamePrometheus,
	} {
		if err := gutil.NewShootAccessSecret(name, b.Shoot.SeedNamespace).Reconcile(ctx, b.K8sSeedClient.Client()); err != nil {
			return err
		}
	}

	alerting, err := b.getCustomAlertingConfigs(ctx, b.GetSecretKeysOfRole(v1beta1constants.GardenRoleAlerting))
	if err != nil {
		return err
	}

	prometheusTLSOverride := common.PrometheusTLS
	if b.ControlPlaneWildcardCert != nil {
		prometheusTLSOverride = b.ControlPlaneWildcardCert.GetName()
	}

	hosts := []map[string]interface{}{
		{
			"hostName":   b.ComputePrometheusHost(),
			"secretName": prometheusTLSOverride,
		},
	}

	ingressClass, err := getIngressClass(b.Seed.GetInfo())
	if err != nil {
		return err
	}

	var (
		networks = map[string]interface{}{
			"pods":     b.Shoot.Networks.Pods.String(),
			"services": b.Shoot.Networks.Services.String(),
		}
		prometheusConfig = map[string]interface{}{
			"kubernetesVersion": b.Shoot.GetInfo().Spec.Kubernetes.Version,
			"nodeLocalDNS": map[string]interface{}{
				"enabled": b.Shoot.NodeLocalDNSEnabled,
			},
			"reversedVPN": map[string]interface{}{
				"enabled": b.Shoot.ReversedVPNEnabled,
			},
			"ingress": map[string]interface{}{
				"class":           ingressClass,
				"basicAuthSecret": basicAuth,
				"hosts":           hosts,
			},
			"namespace": map[string]interface{}{
				"uid": b.SeedNamespaceObject.UID,
			},
			"replicas":           b.Shoot.GetReplicas(1),
			"apiserverServiceIP": b.Shoot.Networks.APIServer.String(),
			"seed": map[string]interface{}{
				"apiserver": b.K8sSeedClient.RESTConfig().Host,
				"region":    b.Seed.GetInfo().Spec.Provider.Region,
				"provider":  b.Seed.GetInfo().Spec.Provider.Type,
			},
			"rules": map[string]interface{}{
				"optional": map[string]interface{}{
					"alertmanager": map[string]interface{}{
						"enabled": b.Shoot.WantsAlertmanager,
					},
					"loki": map[string]interface{}{
						"enabled": gardenletfeatures.FeatureGate.Enabled(features.Logging),
					},
					"lokiTelegraf": map[string]interface{}{
						"enabled": b.isShootNodeLoggingEnabled(),
					},
					"hvpa": map[string]interface{}{
						"enabled": gardenletfeatures.FeatureGate.Enabled(features.HVPA),
					},
				},
			},
			"shoot": map[string]interface{}{
				"apiserver":           fmt.Sprintf("https://%s", gutil.GetAPIServerDomain(b.Shoot.InternalClusterDomain)),
				"apiserverServerName": gutil.GetAPIServerDomain(b.Shoot.InternalClusterDomain),
				"sniEnabled":          b.APIServerSNIEnabled(),
				"provider":            b.Shoot.GetInfo().Spec.Provider.Type,
				"name":                b.Shoot.GetInfo().Name,
				"project":             b.Garden.Project.Name,
			},
			"ignoreAlerts":            b.Shoot.IgnoreAlerts,
			"alerting":                alerting,
			"additionalRules":         alertingRules.String(),
			"additionalScrapeConfigs": scrapeConfigs.String(),
		}
		kubeStateMetricsShootConfig = map[string]interface{}{
			"replicas": b.Shoot.GetReplicas(1),
		}
	)

	if v := b.Shoot.GetInfo().Spec.Networking.Nodes; v != nil {
		networks["nodes"] = *v
	}
	prometheusConfig["networks"] = networks

	var (
		prometheusImages = []string{
			charts.ImageNamePrometheus,
			charts.ImageNameConfigmapReloader,
			charts.ImageNameBlackboxExporter,
		}
		podAnnotations = map[string]interface{}{
			"checksum/secret-prometheus": b.LoadCheckSum("prometheus"),
		}
	)

	prometheusConfig["podAnnotations"] = podAnnotations

	// Add remotewrite to prometheus when enabled
	if b.Config.Monitoring != nil &&
		b.Config.Monitoring.Shoot != nil &&
		b.Config.Monitoring.Shoot.RemoteWrite != nil &&
		b.Config.Monitoring.Shoot.RemoteWrite.URL != "" {
		// if remoteWrite Url is set add config into values
		remoteWriteConfig := map[string]interface{}{
			"url": b.Config.Monitoring.Shoot.RemoteWrite.URL,
		}
		// get secret for basic_auth in remote write
		remoteWriteBasicAuth := b.LoadSecret(v1beta1constants.GardenRoleGlobalShootRemoteWriteMonitoring)
		if remoteWriteBasicAuth != nil {
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
		if len(b.Config.Monitoring.Shoot.RemoteWrite.Keep) != 0 {
			remoteWriteConfig["keep"] = b.Config.Monitoring.Shoot.RemoteWrite.Keep
		}
		// add queue_config if set
		if b.Config.Monitoring.Shoot.RemoteWrite.QueueConfig != nil &&
			len(*b.Config.Monitoring.Shoot.RemoteWrite.QueueConfig) != 0 {
			remoteWriteConfig["queue_config"] = b.Config.Monitoring.Shoot.RemoteWrite.QueueConfig
		}
		prometheusConfig["remoteWrite"] = remoteWriteConfig
	}

	// set externalLabels
	if b.Config.Monitoring != nil &&
		b.Config.Monitoring.Shoot != nil &&
		len(b.Config.Monitoring.Shoot.ExternalLabels) != 0 {
		prometheusConfig["externalLabels"] = b.Config.Monitoring.Shoot.ExternalLabels
	}

	prometheus, err := b.InjectSeedShootImages(prometheusConfig, prometheusImages...)
	if err != nil {
		return err
	}
	kubeStateMetricsShoot, err := b.InjectSeedShootImages(kubeStateMetricsShootConfig, charts.ImageNameKubeStateMetrics)
	if err != nil {
		return err
	}

	coreValues := map[string]interface{}{
		"global": map[string]interface{}{
			"shootKubeVersion": map[string]interface{}{
				"gitVersion": b.Shoot.GetInfo().Spec.Kubernetes.Version,
			},
		},
		"prometheus":               prometheus,
		"kube-state-metrics-shoot": kubeStateMetricsShoot,
	}

	if err := b.K8sSeedClient.ChartApplier().Apply(ctx, filepath.Join(charts.Path, "seed-monitoring", "charts", "core"), b.Shoot.SeedNamespace, fmt.Sprintf("%s-monitoring", b.Shoot.SeedNamespace), kubernetes.Values(coreValues)); err != nil {
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

		alertManagerTLSOverride := common.AlertManagerTLS
		if b.ControlPlaneWildcardCert != nil {
			alertManagerTLSOverride = b.ControlPlaneWildcardCert.GetName()
		}

		hosts := []map[string]interface{}{
			{
				"hostName":   b.ComputeAlertManagerHost(),
				"secretName": alertManagerTLSOverride,
			},
		}

		alertManagerValues, err := b.InjectSeedShootImages(map[string]interface{}{
			"ingress": map[string]interface{}{
				"class":           ingressClass,
				"basicAuthSecret": basicAuthUsers,
				"hosts":           hosts,
			},
			"replicas":     b.Shoot.GetReplicas(1),
			"storage":      b.Seed.GetValidVolumeSize("1Gi"),
			"emailConfigs": emailConfigs,
		}, charts.ImageNameAlertmanager, charts.ImageNameConfigmapReloader)
		if err != nil {
			return err
		}
		if err := b.K8sSeedClient.ChartApplier().Apply(ctx, filepath.Join(charts.Path, "seed-monitoring", "charts", "alertmanager"), b.Shoot.SeedNamespace, fmt.Sprintf("%s-monitoring", b.Shoot.SeedNamespace), kubernetes.Values(alertManagerValues)); err != nil {
			return err
		}
	} else {
		if err := common.DeleteAlertmanager(ctx, b.K8sSeedClient.Client(), b.Shoot.SeedNamespace); err != nil {
			return err
		}
	}

	return kutil.DeleteObjects(ctx, b.K8sSeedClient.Client(),
		// TODO(rfranzke): Remove in a future release.
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "kube-state-metrics", Namespace: b.Shoot.SeedNamespace}},
		// TODO(rfranzke): Uncomment this in a future release once all monitoring configurations of extensions have been
		// adapted.
		//&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "prometheus", Namespace: b.Shoot.SeedNamespace}},
	)
}

// DeploySeedGrafana deploys the grafana charts to the Seed cluster.
func (b *Botanist) DeploySeedGrafana(ctx context.Context) error {
	if b.Shoot.Purpose == gardencorev1beta1.ShootPurposeTesting {
		return b.DeleteGrafana(ctx)
	}

	var (
		credentials         = b.LoadSecret(common.MonitoringIngressCredentials)
		credentialsUsers    = b.LoadSecret(common.MonitoringIngressCredentialsUsers)
		basicAuth           = utils.CreateSHA1Secret(credentials.Data[secrets.DataKeyUserName], credentials.Data[secrets.DataKeyPassword])
		basicAuthUsers      = utils.CreateSHA1Secret(credentialsUsers.Data[secrets.DataKeyUserName], credentialsUsers.Data[secrets.DataKeyPassword])
		operatorsDashboards = strings.Builder{}
		usersDashboards     = strings.Builder{}
	)

	// Fetch extensions provider-specific monitoring configuration
	existingConfigMaps := &corev1.ConfigMapList{}
	if err := b.K8sSeedClient.Client().List(ctx, existingConfigMaps,
		client.InNamespace(b.Shoot.SeedNamespace),
		client.MatchingLabels{v1beta1constants.LabelExtensionConfiguration: v1beta1constants.LabelMonitoring}); err != nil {
		return err
	}

	// Need stable order before passing the dashboards to Grafana config to avoid unnecessary changes
	kutil.ByName().Sort(existingConfigMaps)

	// Read extension monitoring configurations
	for _, cm := range existingConfigMaps.Items {
		if operatorsDashboard, ok := cm.Data[v1beta1constants.GrafanaConfigMapOperatorDashboard]; ok && operatorsDashboard != "" {
			operatorsDashboards.WriteString(fmt.Sprintln(operatorsDashboard))
		}
		if usersDashboard, ok := cm.Data[v1beta1constants.GrafanaConfigMapUserDashboard]; ok && usersDashboard != "" {
			usersDashboards.WriteString(fmt.Sprintln(usersDashboard))
		}
	}

	if err := b.deployGrafanaCharts(ctx, common.GrafanaOperatorsRole, operatorsDashboards.String(), basicAuth, common.GrafanaOperatorsPrefix); err != nil {
		return err
	}

	return b.deployGrafanaCharts(ctx, common.GrafanaUsersRole, usersDashboards.String(), basicAuthUsers, common.GrafanaUsersPrefix)
}

func getIngressClass(seed *gardencorev1beta1.Seed) (string, error) {
	managedIngress := seed.Spec.Ingress != nil && seed.Spec.Ingress.Controller.Kind == v1beta1constants.IngressKindNginx
	if !managedIngress {
		return v1beta1constants.ShootNginxIngressClass, nil
	}

	if seed.Status.KubernetesVersion == nil {
		return "", fmt.Errorf("kubernetes version is missing in status for seed %q", seed.Name)
	}

	greaterEqual122, err := versionutils.CompareVersions(*seed.Status.KubernetesVersion, ">=", "1.22")
	if err != nil {
		return "", err
	}

	if greaterEqual122 {
		return v1beta1constants.SeedNginxIngressClass122, nil
	}

	return v1beta1constants.SeedNginxIngressClass, nil
}

func (b *Botanist) getCustomAlertingConfigs(ctx context.Context, alertingSecretKeys []string) (map[string]interface{}, error) {
	configs := map[string]interface{}{
		"auth_type": map[string]interface{}{},
	}

	for _, key := range alertingSecretKeys {
		secret := b.LoadSecret(key)

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
						Namespace: b.Shoot.SeedNamespace,
					},
				}

				if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, b.K8sSeedClient.Client(), amSecret, func() error {
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

func (b *Botanist) deployGrafanaCharts(ctx context.Context, role, dashboards, basicAuth, subDomain string) error {
	grafanaTLSOverride := common.GrafanaTLS
	if b.ControlPlaneWildcardCert != nil {
		grafanaTLSOverride = b.ControlPlaneWildcardCert.GetName()
	}

	hosts := []map[string]interface{}{
		{
			"hostName":   b.ComputeIngressHost(subDomain),
			"secretName": grafanaTLSOverride,
		},
	}

	ingressClass, err := getIngressClass(b.Seed.GetInfo())
	if err != nil {
		return err
	}

	values, err := b.InjectSeedShootImages(map[string]interface{}{
		"ingress": map[string]interface{}{
			"class":           ingressClass,
			"basicAuthSecret": basicAuth,
			"hosts":           hosts,
		},
		"replicas": b.Shoot.GetReplicas(1),
		"role":     role,
		"extensions": map[string]interface{}{
			"dashboards": dashboards,
		},
		"vpaEnabled": b.Shoot.WantsVerticalPodAutoscaler,
		"sni": map[string]interface{}{
			"enabled": b.APIServerSNIEnabled(),
		},
		"nodeLocalDNS": map[string]interface{}{
			"enabled": b.Shoot.NodeLocalDNSEnabled,
		},
	}, charts.ImageNameGrafana)
	if err != nil {
		return err
	}

	if err := b.K8sSeedClient.ChartApplier().Apply(ctx, filepath.Join(charts.Path, "seed-monitoring", "charts", "grafana"), b.Shoot.SeedNamespace, fmt.Sprintf("%s-monitoring", b.Shoot.SeedNamespace), kubernetes.Values(values)); err != nil {
		return err
	}

	// TODO(rfranzke): Remove in a future release.
	return kutil.DeleteObjects(ctx, b.K8sSeedClient.Client(),
		&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: b.Shoot.SeedNamespace, Name: "grafana-operators-dashboard-providers"}},
		&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: b.Shoot.SeedNamespace, Name: "grafana-operators-datasources"}},
		&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: b.Shoot.SeedNamespace, Name: "grafana-operators-dashboards"}},
		&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: b.Shoot.SeedNamespace, Name: "grafana-users-dashboard-providers"}},
		&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: b.Shoot.SeedNamespace, Name: "grafana-users-datasources"}},
		&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: b.Shoot.SeedNamespace, Name: "grafana-users-dashboards"}},
	)
}

// DeleteGrafana will delete all grafana instances from the seed cluster.
func (b *Botanist) DeleteGrafana(ctx context.Context) error {
	if err := common.DeleteGrafanaByRole(ctx, b.K8sSeedClient, b.Shoot.SeedNamespace, common.GrafanaOperatorsRole); err != nil {
		return err
	}

	return common.DeleteGrafanaByRole(ctx, b.K8sSeedClient, b.Shoot.SeedNamespace, common.GrafanaUsersRole)
}

// DeleteSeedMonitoring will delete the monitoring stack from the Seed cluster to avoid phantom alerts
// during the deletion process. More precisely, the Alertmanager and Prometheus StatefulSets will be
// deleted.
func (b *Botanist) DeleteSeedMonitoring(ctx context.Context) error {
	if err := common.DeleteAlertmanager(ctx, b.K8sSeedClient.Client(), b.Shoot.SeedNamespace); err != nil {
		return err
	}

	objects := []client.Object{
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: b.Shoot.SeedNamespace,
				Name:      "kube-state-metrics",
			},
		},
		gutil.NewShootAccessSecret(v1beta1constants.DeploymentNameKubeStateMetricsShoot, b.Shoot.SeedNamespace).Secret,
		&autoscalingv1beta2.VerticalPodAutoscaler{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: b.Shoot.SeedNamespace,
				Name:      "kube-state-metrics-vpa",
			},
		},
		&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: b.Shoot.SeedNamespace,
				Name:      "kube-state-metrics",
			},
		},

		&networkingv1.NetworkPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: b.Shoot.SeedNamespace,
				Name:      "allow-from-prometheus",
			},
		},
		&networkingv1.NetworkPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: b.Shoot.SeedNamespace,
				Name:      "allow-prometheus",
			},
		},
		gutil.NewShootAccessSecret(v1beta1constants.StatefulSetNamePrometheus, b.Shoot.SeedNamespace).Secret,
		&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: b.Shoot.SeedNamespace,
				Name:      "prometheus-config",
			},
		},
		&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: b.Shoot.SeedNamespace,
				Name:      "prometheus-rules",
			},
		},
		&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: b.Shoot.SeedNamespace,
				Name:      "blackbox-exporter-config-prometheus",
			},
		},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: b.Shoot.SeedNamespace,
				Name:      "prometheus-basic-auth",
			},
		},
		&extensionsv1beta1.Ingress{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: b.Shoot.SeedNamespace,
				Name:      "prometheus",
			},
		},
		&networkingv1.Ingress{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: b.Shoot.SeedNamespace,
				Name:      "prometheus",
			},
		},
		&autoscalingv1beta2.VerticalPodAutoscaler{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: b.Shoot.SeedNamespace,
				Name:      "prometheus-vpa",
			},
		},
		&corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: b.Shoot.SeedNamespace,
				Name:      "prometheus",
			},
		},
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: b.Shoot.SeedNamespace,
				Name:      "prometheus-web",
			},
		},
		&appsv1.StatefulSet{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: b.Shoot.SeedNamespace,
				Name:      "prometheus",
			},
		},
		&rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: b.Shoot.SeedNamespace,
				Name:      "prometheus-" + b.Shoot.SeedNamespace,
			},
		},
		&corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: b.Shoot.SeedNamespace,
				Name:      "prometheus-db-prometheus-0",
			},
		},
	}

	return kutil.DeleteObjects(ctx, b.K8sSeedClient.Client(), objects...)
}
