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

	v1alpha1constants "github.com/gardener/gardener/pkg/apis/core/v1alpha1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	controllermanagerfeatures "github.com/gardener/gardener/pkg/controllermanager/features"
	"github.com/gardener/gardener/pkg/features"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/secrets"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// DeploySeedMonitoring will install the Helm release "seed-monitoring" in the Seed clusters. It comprises components
// to monitor the Shoot cluster whose control plane runs in the Seed cluster.
func (b *Botanist) DeploySeedMonitoring(ctx context.Context) error {
	var (
		credentials         = b.Secrets["monitoring-ingress-credentials"]
		credentialsUsers    = b.Secrets["monitoring-ingress-credentials-users"]
		basicAuth           = utils.CreateSHA1Secret(credentials.Data[secrets.DataKeyUserName], credentials.Data[secrets.DataKeyPassword])
		basicAuthUsers      = utils.CreateSHA1Secret(credentialsUsers.Data[secrets.DataKeyUserName], credentialsUsers.Data[secrets.DataKeyPassword])
		prometheusHost      = b.ComputePrometheusHost()
		alertingRules       = strings.Builder{}
		scrapeConfigs       = strings.Builder{}
		operatorsDashboards = strings.Builder{}
		usersDashboards     = strings.Builder{}
	)

	// Find extensions provider-specific monitoring configuration
	existingConfigMaps := &corev1.ConfigMapList{}
	if err := b.K8sSeedClient.Client().List(ctx, existingConfigMaps,
		client.InNamespace(b.Shoot.SeedNamespace),
		client.MatchingLabels(map[string]string{v1alpha1constants.LabelExtensionConfiguration: v1alpha1constants.LabelMonitoring})); err != nil {
		return err
	}

	// Read extension monitoring configurations
	for _, cm := range existingConfigMaps.Items {
		alertingRules.WriteString(fmt.Sprintln(cm.Data[v1alpha1constants.PrometheusConfigMapAlertingRules]))
		scrapeConfigs.WriteString(fmt.Sprintln(cm.Data[v1alpha1constants.PrometheusConfigMapScrapeConfig]))
		operatorsDashboards.WriteString(fmt.Sprintln(cm.Data[v1alpha1constants.GrafanaConfigMapOperatorDashboard]))
		usersDashboards.WriteString(fmt.Sprintln(cm.Data[v1alpha1constants.GrafanaConfigMapUserDashboard]))
	}

	var (
		prometheusConfig = map[string]interface{}{
			"kubernetesVersion": b.Shoot.Info.Spec.Kubernetes.Version,
			"networks": map[string]interface{}{
				"pods":     b.Shoot.GetPodNetwork(),
				"services": b.Shoot.GetServiceNetwork(),
				"nodes":    b.Shoot.Info.Spec.Networking.Nodes,
			},
			"ingress": map[string]interface{}{
				"basicAuthSecret": basicAuth,
				"host":            prometheusHost,
			},
			"namespace": map[string]interface{}{
				"uid": b.SeedNamespaceObject.UID,
			},
			"objectCount": b.Shoot.GetNodeCount(),
			"podAnnotations": map[string]interface{}{
				"checksum/secret-prometheus":       b.CheckSums["prometheus"],
				"checksum/secret-vpn-seed":         b.CheckSums["vpn-seed"],
				"checksum/secret-vpn-seed-tlsauth": b.CheckSums["vpn-seed-tlsauth"],
			},
			"replicas":           b.Shoot.GetReplicas(1),
			"apiserverServiceIP": common.ComputeClusterIP(b.Shoot.GetServiceNetwork(), 1),
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
						"enabled": controllermanagerfeatures.FeatureGate.Enabled(features.Logging),
					},
					"hvpa": map[string]interface{}{
						"enabled": controllermanagerfeatures.FeatureGate.Enabled(features.HVPA),
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

	if err := b.ApplyChartSeed(filepath.Join(common.ChartPath, "seed-monitoring", "charts", "core"), b.Shoot.SeedNamespace, fmt.Sprintf("%s-monitoring", b.Shoot.SeedNamespace), nil, coreValues); err != nil {
		return err
	}

	if err := b.deployGrafanaCharts("operators", operatorsDashboards.String(), basicAuth, common.GrafanaOperatorsPrefix); err != nil {
		return err
	}

	if err := b.deployGrafanaCharts("users", usersDashboards.String(), basicAuthUsers, common.GrafanaUsersPrefix); err != nil {
		return err
	}

	// Check if we want to deploy an alertmanager into the shoot namespace.
	if b.Shoot.WantsAlertmanager {
		var (
			alertingSMTPKeys = b.GetSecretKeysOfRole(common.GardenRoleAlertingSMTP)
			emailConfigs     = []map[string]interface{}{}
		)
		if b.Shoot.Info.Spec.Monitoring != nil && b.Shoot.Info.Spec.Monitoring.Alerting != nil {
			for _, email := range b.Shoot.Info.Spec.Monitoring.Alerting.EmailReceivers {
				for _, key := range alertingSMTPKeys {
					secret := b.Secrets[key]
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
		alertManagerValues, err := b.InjectSeedShootImages(map[string]interface{}{
			"ingress": map[string]interface{}{
				"basicAuthSecret": basicAuthUsers,
				"host":            b.Seed.GetIngressFQDN("au", b.Shoot.Info.Name, b.Garden.Project.Name),
			},
			"replicas":     b.Shoot.GetReplicas(1),
			"storage":      b.Seed.GetValidVolumeSize("1Gi"),
			"emailConfigs": emailConfigs,
		}, common.AlertManagerImageName, common.ConfigMapReloaderImageName)
		if err != nil {
			return err
		}
		if err := b.ApplyChartSeed(filepath.Join(common.ChartPath, "seed-monitoring", "charts", "alertmanager"), b.Shoot.SeedNamespace, fmt.Sprintf("%s-monitoring", b.Shoot.SeedNamespace), nil, alertManagerValues); err != nil {
			return err
		}
	} else {
		if err := common.DeleteAlertmanager(ctx, b.K8sSeedClient.Client(), b.Shoot.SeedNamespace); err != nil {
			return err
		}
	}

	return nil
}

func (b *Botanist) deployGrafanaCharts(role, dashboards, basicAuth, subDomain string) error {
	values, err := b.InjectSeedShootImages(map[string]interface{}{
		"ingress": map[string]interface{}{
			"basicAuthSecret": basicAuth,
			"host":            b.Seed.GetIngressFQDN(subDomain, b.Shoot.Info.Name, b.Garden.Project.Name),
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
	return b.ApplyChartSeed(filepath.Join(common.ChartPath, "seed-monitoring", "charts", "grafana"), b.Shoot.SeedNamespace, fmt.Sprintf("%s-monitoring", b.Shoot.SeedNamespace), nil, values)
}

// DeleteSeedMonitoring will delete the monitoring stack from the Seed cluster to avoid phantom alerts
// during the deletion process. More precisely, the Alertmanager and Prometheus StatefulSets will be
// deleted.
func (b *Botanist) DeleteSeedMonitoring(ctx context.Context) error {
	alertManagerStatefulSet := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      v1alpha1constants.StatefulSetNameAlertManager,
			Namespace: b.Shoot.SeedNamespace,
		},
	}
	if err := b.K8sSeedClient.Client().Delete(ctx, alertManagerStatefulSet, kubernetes.DefaultDeleteOptionFuncs...); client.IgnoreNotFound(err) != nil {
		return err
	}

	prometheusStatefulSet := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      v1alpha1constants.StatefulSetNamePrometheus,
			Namespace: b.Shoot.SeedNamespace,
		},
	}
	return client.IgnoreNotFound(b.K8sSeedClient.Client().Delete(ctx, prometheusStatefulSet, kubernetes.DefaultDeleteOptionFuncs...))
}
