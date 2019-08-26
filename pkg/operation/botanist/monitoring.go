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

	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	controllermanagerfeatures "github.com/gardener/gardener/pkg/controllermanager/features"
	"github.com/gardener/gardener/pkg/features"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/secrets"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// DeploySeedMonitoring will install the Helm release "seed-monitoring" in the Seed clusters. It comprises components
// to monitor the Shoot cluster whose control plane runs in the Seed cluster.
func (b *Botanist) DeploySeedMonitoring(ctx context.Context) error {
	var (
		credentials      = b.Secrets["monitoring-ingress-credentials"]
		credentialsUsers = b.Secrets["monitoring-ingress-credentials-users"]
		basicAuth        = utils.CreateSHA1Secret(credentials.Data[secrets.DataKeyUserName], credentials.Data[secrets.DataKeyPassword])
		basicAuthUsers   = utils.CreateSHA1Secret(credentialsUsers.Data[secrets.DataKeyUserName], credentialsUsers.Data[secrets.DataKeyPassword])
		prometheusHost   = b.ComputePrometheusHost()
	)

	var (
		prometheusConfig = map[string]interface{}{
			"kubernetesVersion": b.Shoot.Info.Spec.Kubernetes.Version,
			"networks": map[string]interface{}{
				"pods":     b.Shoot.GetPodNetwork(),
				"services": b.Shoot.GetServiceNetwork(),
				"nodes":    b.Shoot.GetNodeNetwork(),
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
				"region":    b.Seed.Info.Spec.Cloud.Region,
				"profile":   b.Seed.Info.Spec.Cloud.Profile,
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
				},
			},
			"shoot": map[string]interface{}{
				"apiserver": fmt.Sprintf("https://%s", common.GetAPIServerDomain(b.Shoot.InternalClusterDomain)),
				"provider":  b.Shoot.CloudProvider,
			},
			"ignoreAlerts": b.Shoot.IgnoreAlerts,
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

	if err := b.deployGrafanaCharts("operators", basicAuth, common.GrafanaOperatorsPrefix); err != nil {
		return err
	}

	if err := b.deployGrafanaCharts("users", basicAuthUsers, common.GrafanaUsersPrefix); err != nil {
		return err
	}

	// Check if we want to deploy an alertmanager into the shoot namespace.
	if b.Shoot.WantsAlertmanager {
		var (
			alertingSMTPKeys = b.GetSecretKeysOfRole(common.GardenRoleAlertingSMTP)
			emailConfigs     = []map[string]interface{}{}
			to, _            = b.Shoot.Info.Annotations[common.GardenOperatedBy]
		)

		for _, key := range alertingSMTPKeys {
			secret := b.Secrets[key]
			emailConfigs = append(emailConfigs, map[string]interface{}{
				"to":            to,
				"from":          string(secret.Data["from"]),
				"smarthost":     string(secret.Data["smarthost"]),
				"auth_username": string(secret.Data["auth_username"]),
				"auth_identity": string(secret.Data["auth_identity"]),
				"auth_password": string(secret.Data["auth_password"]),
			})
		}

		alertManagerValues, err := b.InjectSeedShootImages(map[string]interface{}{
			"ingress": map[string]interface{}{
				"basicAuthSecret": basicAuth,
				"host":            b.Seed.GetIngressFQDN("a", b.Shoot.Info.Name, b.Garden.Project.Name),
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

func (b *Botanist) deployGrafanaCharts(role, basicAuth, subDomain string) error {
	values, err := b.InjectSeedShootImages(map[string]interface{}{
		"ingress": map[string]interface{}{
			"basicAuthSecret": basicAuth,
			"host":            b.Seed.GetIngressFQDN(subDomain, b.Shoot.Info.Name, b.Garden.Project.Name),
		},
		"replicas": b.Shoot.GetReplicas(1),
		"role":     role,
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
			Name:      common.AlertManagerStatefulSetName,
			Namespace: b.Shoot.SeedNamespace,
		},
	}
	if err := b.K8sSeedClient.Client().Delete(ctx, alertManagerStatefulSet, kubernetes.DefaultDeleteOptionFuncs...); client.IgnoreNotFound(err) != nil {
		return err
	}

	prometheusStatefulSet := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      gardencorev1alpha1.StatefulSetNamePrometheus,
			Namespace: b.Shoot.SeedNamespace,
		},
	}
	return client.IgnoreNotFound(b.K8sSeedClient.Client().Delete(ctx, prometheusStatefulSet, kubernetes.DefaultDeleteOptionFuncs...))
}
