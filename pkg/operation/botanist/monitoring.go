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

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/features"
	gardenletfeatures "github.com/gardener/gardener/pkg/gardenlet/features"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/secrets"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// DeploySeedMonitoring will install the Helm release "seed-monitoring" in the Seed clusters. It comprises components
// to monitor the Shoot cluster whose control plane runs in the Seed cluster.
func (b *Botanist) DeploySeedMonitoring(ctx context.Context) error {
	if b.Shoot.GetPurpose() == gardencorev1beta1.ShootPurposeTesting {
		return b.DeleteSeedMonitoring(ctx)
	}

	var (
		credentials         = b.Secrets["monitoring-ingress-credentials"]
		credentialsUsers    = b.Secrets["monitoring-ingress-credentials-users"]
		basicAuth           = utils.CreateSHA1Secret(credentials.Data[secrets.DataKeyUserName], credentials.Data[secrets.DataKeyPassword])
		basicAuthUsers      = utils.CreateSHA1Secret(credentialsUsers.Data[secrets.DataKeyUserName], credentialsUsers.Data[secrets.DataKeyPassword])
		alertingRules       = strings.Builder{}
		scrapeConfigs       = strings.Builder{}
		operatorsDashboards = strings.Builder{}
		usersDashboards     = strings.Builder{}
	)

	// Find extensions provider-specific monitoring configuration
	existingConfigMaps := &corev1.ConfigMapList{}
	if err := b.K8sSeedClient.Client().List(ctx, existingConfigMaps,
		client.InNamespace(b.Shoot.SeedNamespace),
		client.MatchingLabels{v1beta1constants.LabelExtensionConfiguration: v1beta1constants.LabelMonitoring}); err != nil {
		return err
	}

	// Read extension monitoring configurations
	for _, cm := range existingConfigMaps.Items {
		alertingRules.WriteString(fmt.Sprintln(cm.Data[v1beta1constants.PrometheusConfigMapAlertingRules]))
		scrapeConfigs.WriteString(fmt.Sprintln(cm.Data[v1beta1constants.PrometheusConfigMapScrapeConfig]))
		operatorsDashboards.WriteString(fmt.Sprintln(cm.Data[v1beta1constants.GrafanaConfigMapOperatorDashboard]))
		usersDashboards.WriteString(fmt.Sprintln(cm.Data[v1beta1constants.GrafanaConfigMapUserDashboard]))
	}

	alerting, err := b.getCustomAlertingConfigs(ctx, b.GetSecretKeysOfRole(common.GardenRoleAlerting))
	if err != nil {
		return err
	}

	prometheusTLSOverride := common.PrometheusTLS
	if b.ControlPlaneWildcardCert != nil {
		prometheusTLSOverride = b.ControlPlaneWildcardCert.GetName()
	}

	hosts := []map[string]interface{}{
		// TODO: timuthy - remove in the future. Old Prometheus host is retained for migration reasons.
		{
			"hostName":   b.ComputePrometheusHostDeprecated(),
			"secretName": common.PrometheusTLS,
		},
		{
			"hostName":   b.ComputePrometheusHost(),
			"secretName": prometheusTLSOverride,
		},
	}

	var (
		networks = map[string]interface{}{
			"pods":     b.Shoot.Networks.Pods.String(),
			"services": b.Shoot.Networks.Services.String(),
		}

		prometheusConfig = map[string]interface{}{
			"kubernetesVersion": b.Shoot.Info.Spec.Kubernetes.Version,
			"ingress": map[string]interface{}{
				"basicAuthSecret": basicAuth,
				"hosts":           hosts,
			},
			"namespace": map[string]interface{}{
				"uid": b.SeedNamespaceObject.UID,
			},
			"podAnnotations": map[string]interface{}{
				"checksum/secret-prometheus":       b.CheckSums["prometheus"],
				"checksum/secret-vpn-seed":         b.CheckSums["vpn-seed"],
				"checksum/secret-vpn-seed-tlsauth": b.CheckSums["vpn-seed-tlsauth"],
			},
			"replicas":           b.Shoot.GetReplicas(1),
			"apiserverServiceIP": b.Shoot.Networks.APIServer.String(),
			"seed": map[string]interface{}{
				"apiserver": b.K8sSeedClient.RESTConfig().Host,
				"region":    b.Seed.Info.Spec.Provider.Region,
				"provider":  b.Seed.Info.Spec.Provider.Type,
			},
			"rules": map[string]interface{}{
				"optional": map[string]interface{}{
					"cluster-autoscaler": map[string]interface{}{
						"enabled": b.Shoot.WantsClusterAutoscaler,
					},
					"alertmanager": map[string]interface{}{
						"enabled": b.Shoot.WantsAlertmanager,
					},
					"elasticsearch": map[string]interface{}{
						"enabled": gardenletfeatures.FeatureGate.Enabled(features.Logging),
					},
					"hvpa": map[string]interface{}{
						"enabled": gardenletfeatures.FeatureGate.Enabled(features.HVPA),
					},
				},
			},
			"shoot": map[string]interface{}{
				"apiserver": fmt.Sprintf("https://%s", common.GetAPIServerDomain(b.Shoot.InternalClusterDomain)),
				"provider":  b.Shoot.Info.Spec.Provider.Type,
				"name":      b.Shoot.Info.Name,
				"project":   b.Garden.Project.Name,
			},
			"ignoreAlerts": b.Shoot.IgnoreAlerts,
			"alerting":     alerting,
			"extensions": map[string]interface{}{
				"rules":         alertingRules.String(),
				"scrapeConfigs": scrapeConfigs.String(),
			},
		}
		kubeStateMetricsSeedConfig = map[string]interface{}{
			"replicas": b.Shoot.GetReplicas(1),
		}
		kubeStateMetricsShootConfig = map[string]interface{}{
			"replicas": b.Shoot.GetReplicas(1),
		}
	)

	if v := b.Shoot.GetNodeNetwork(); v != nil {
		networks["nodes"] = *v
	}
	prometheusConfig["networks"] = networks

	prometheus, err := b.InjectSeedShootImages(prometheusConfig,
		common.PrometheusImageName,
		common.ConfigMapReloaderImageName,
		common.VPNSeedImageName,
		common.BlackboxExporterImageName,
		common.AlpineIptablesImageName,
	)
	if err != nil {
		return err
	}
	kubeStateMetricsSeed, err := b.InjectSeedShootImages(kubeStateMetricsSeedConfig, common.KubeStateMetricsImageName)
	if err != nil {
		return err
	}
	kubeStateMetricsShoot, err := b.InjectSeedShootImages(kubeStateMetricsShootConfig, common.KubeStateMetricsImageName)
	if err != nil {
		return err
	}

	coreValues := map[string]interface{}{
		"global": map[string]interface{}{
			"shootKubeVersion": map[string]interface{}{
				"gitVersion": b.Shoot.Info.Spec.Kubernetes.Version,
			},
		},
		"prometheus":               prometheus,
		"kube-state-metrics-seed":  kubeStateMetricsSeed,
		"kube-state-metrics-shoot": kubeStateMetricsShoot,
	}

	if err := b.ChartApplierSeed.Apply(ctx, filepath.Join(common.ChartPath, "seed-monitoring", "charts", "core"), b.Shoot.SeedNamespace, fmt.Sprintf("%s-monitoring", b.Shoot.SeedNamespace), kubernetes.Values(coreValues)); err != nil {
		return err
	}

	if err := b.deployGrafanaCharts(ctx, common.GrafanaOperatorsRole, operatorsDashboards.String(), basicAuth, common.GrafanaOperatorsPrefix); err != nil {
		return err
	}

	if err := b.deployGrafanaCharts(ctx, common.GrafanaUsersRole, usersDashboards.String(), basicAuthUsers, common.GrafanaUsersPrefix); err != nil {
		return err
	}

	// Check if we want to deploy an alertmanager into the shoot namespace.
	if b.Shoot.WantsAlertmanager {
		var (
			alertingSMTPKeys = b.GetSecretKeysOfRole(common.GardenRoleAlerting)
			emailConfigs     = []map[string]interface{}{}
		)

		if b.Shoot.Info.Spec.Monitoring != nil && b.Shoot.Info.Spec.Monitoring.Alerting != nil {
			for _, email := range b.Shoot.Info.Spec.Monitoring.Alerting.EmailReceivers {
				for _, key := range alertingSMTPKeys {
					secret := b.Secrets[key]

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
			// TODO: timuthy - remove in the future. Old Prometheus host is retained for migration reasons.
			{
				"hostName":   b.ComputeAlertManagerHostDeprecated(),
				"secretName": common.AlertManagerTLS,
			},
			{
				"hostName":   b.ComputeAlertManagerHost(),
				"secretName": alertManagerTLSOverride,
			},
		}

		alertManagerValues, err := b.InjectSeedShootImages(map[string]interface{}{
			"ingress": map[string]interface{}{
				"basicAuthSecret": basicAuthUsers,
				"hosts":           hosts,
			},
			"replicas":     b.Shoot.GetReplicas(1),
			"storage":      b.Seed.GetValidVolumeSize("1Gi"),
			"emailConfigs": emailConfigs,
		}, common.AlertManagerImageName, common.ConfigMapReloaderImageName)
		if err != nil {
			return err
		}
		if err := b.ChartApplierSeed.Apply(ctx, filepath.Join(common.ChartPath, "seed-monitoring", "charts", "alertmanager"), b.Shoot.SeedNamespace, fmt.Sprintf("%s-monitoring", b.Shoot.SeedNamespace), kubernetes.Values(alertManagerValues)); err != nil {
			return err
		}
	} else {
		if err := common.DeleteAlertmanager(ctx, b.K8sSeedClient.Client(), b.Shoot.SeedNamespace); err != nil {
			return err
		}
	}

	return nil
}

func (b *Botanist) getCustomAlertingConfigs(ctx context.Context, alertingSecretKeys []string) (map[string]interface{}, error) {
	configs := map[string]interface{}{
		"auth_type": map[string]interface{}{},
	}

	for _, key := range alertingSecretKeys {
		secret := b.Secrets[key]

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

				if _, err := controllerutil.CreateOrUpdate(ctx, b.K8sSeedClient.Client(), amSecret, func() error {
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
			"hostName":   b.ComputeIngressHostDeprecated(subDomain),
			"secretName": common.GrafanaTLS,
		},
		{
			"hostName":   b.ComputeIngressHost(subDomain),
			"secretName": grafanaTLSOverride,
		},
	}

	values, err := b.InjectSeedShootImages(map[string]interface{}{
		"ingress": map[string]interface{}{
			"basicAuthSecret": basicAuth,
			"hosts":           hosts,
		},
		"replicas": b.Shoot.GetReplicas(1),
		"role":     role,
		"extensions": map[string]interface{}{
			"dashboards": dashboards,
		},
	}, common.GrafanaImageName, common.BusyboxImageName)
	if err != nil {
		return err
	}
	return b.ChartApplierSeed.Apply(ctx, filepath.Join(common.ChartPath, "seed-monitoring", "charts", "grafana"), b.Shoot.SeedNamespace, fmt.Sprintf("%s-monitoring", b.Shoot.SeedNamespace), kubernetes.Values(values))
}

// DeleteSeedMonitoring will delete the monitoring stack from the Seed cluster to avoid phantom alerts
// during the deletion process. More precisely, the Alertmanager and Prometheus StatefulSets will be
// deleted.
func (b *Botanist) DeleteSeedMonitoring(ctx context.Context) error {
	if err := common.DeleteAlertmanager(ctx, b.K8sSeedClient.Client(), b.Shoot.SeedNamespace); err != nil {
		return err
	}

	if err := common.DeleteGrafanaByRole(b.K8sSeedClient, b.Shoot.SeedNamespace, common.GrafanaOperatorsRole); err != nil {
		return err
	}

	if err := common.DeleteGrafanaByRole(b.K8sSeedClient, b.Shoot.SeedNamespace, common.GrafanaUsersRole); err != nil {
		return err
	}

	for _, obj := range []struct {
		apiGroup string
		version  string
		kind     string
		name     string
	}{
		{"", "v1", "ServiceAccount", "kube-state-metrics-seed"},
		{"", "v1", "RoleBinding", "kube-state-metrics-seed"},
		{"", "v1", "Service", "kube-state-metrics-seed"},
		{"apps", "v1", "Deployment", "kube-state-metrics-seed"},
		{"autoscaling.k8s.io", "v1beta2", "VerticalPodAutoscaler", "kube-state-metrics-seed-vpa"},

		{"", "v1", "Service", "kube-state-metrics"},
		{"autoscaling.k8s.io", "v1beta2", "VerticalPodAutoscaler", "kube-state-metrics-vpa"},
		{"apps", "v1", "Deployment", "kube-state-metrics"},

		{"networking", "v1", "NetworkPolicy", "allow-from-prometheus"},
		{"networking", "v1", "NetworkPolicy", "allow-prometheus"},
		{"", "v1", "ConfigMap", "prometheus-config"},
		{"", "v1", "ConfigMap", "prometheus-rules"},
		{"", "v1", "ConfigMap", "blackbox-exporter-config-prometheus"},
		{"", "v1", "Secret", "prometheus-basic-auth"},
		{"extensions", "v1beta1", "Ingress", "prometheus"},
		{"networking", "v1", "Ingress", "prometheus"},
		{"autoscaling.k8s.io", "v1beta2", "VerticalPodAutoscaler", "prometheus-vpa"},
		{"", "v1", "ServiceAccount", "prometheus"},
		{"", "v1", "Service", "prometheus"},
		{"", "v1", "Service", "prometheus-web"},
		{"apps", "v1", "StatefulSet", "prometheus"},
		{"rbac", "v1", "ClusterRoleBinding", "prometheus-" + b.Shoot.SeedNamespace},
		{"", "v1", "PersistentVolumeClaim", "prometheus-db-prometheus-0"},
	} {
		u := &unstructured.Unstructured{}
		u.SetName(obj.name)
		u.SetNamespace(b.Shoot.SeedNamespace)
		u.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   obj.apiGroup,
			Version: obj.version,
			Kind:    obj.kind,
		})
		if err := b.K8sSeedClient.Client().Delete(ctx, u); client.IgnoreNotFound(err) != nil && !meta.IsNoMatchError(err) {
			return err
		}
	}

	return nil
}
