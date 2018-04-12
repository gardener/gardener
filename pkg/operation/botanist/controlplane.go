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
	"fmt"
	"path/filepath"

	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DeployNamespace creates a namespace in the Seed cluster which is used to deploy all the control plane
// components for the Shoot cluster. Moreover, the cloud provider configuration and all the secrets will be
// stored as ConfigMaps/Secrets.
func (b *Botanist) DeployNamespace() error {
	namespace, err := b.K8sSeedClient.CreateNamespace(&corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: b.Operation.Shoot.SeedNamespace,
			Labels: map[string]string{
				common.GardenRole: "shoot",
			},
		},
	}, true)
	if err != nil {
		return err
	}
	b.SeedNamespaceObject = namespace
	return nil
}

// DeleteNamespace deletes the namespace in the Seed cluster which holds the control plane components. The built-in
// garbage collection in Kubernetes will automatically delete all resources which belong to this namespace. This
// comprises volumes and load balancers as well.
func (b *Botanist) DeleteNamespace() error {
	err := b.K8sSeedClient.DeleteNamespace(b.Operation.Shoot.SeedNamespace)
	if apierrors.IsNotFound(err) || apierrors.IsConflict(err) {
		return nil
	}
	return err
}

// DeployKubeAPIServerService creates a Service of type 'LoadBalancer' in the Seed cluster which is used to expose the
// kube-apiserver deployment (of the Shoot cluster). It waits until the load balancer is available and stores the address
// on the Botanist's APIServerAddress attribute.
func (b *Botanist) DeployKubeAPIServerService() error {
	return b.ApplyChartSeed(filepath.Join(common.ChartPath, "seed-controlplane", "charts", "kube-apiserver-service"), "kube-apiserver-service", b.Operation.Shoot.SeedNamespace, nil, map[string]interface{}{
		"cloudProvider": b.Seed.CloudProvider,
	})
}

// DeleteKubeAPIServer deletes the kube-apiserver deployment in the Seed cluster which holds the Shoot's control plane.
func (b *Botanist) DeleteKubeAPIServer() error {
	err := b.K8sSeedClient.DeleteDeployment(b.Operation.Shoot.SeedNamespace, common.KubeAPIServerDeploymentName)
	if apierrors.IsNotFound(err) {
		return nil
	}
	return err
}

// DeleteKubeAddonManager deletes the kube-addon-manager deployment in the Seed cluster which holds the Shoot's control plane. It
// needs to be deleted before trying to remove any resources in the Shoot cluster, othwewise it will automatically recreate
// them and block the infrastructure deletion.
func (b *Botanist) DeleteKubeAddonManager() error {
	err := b.K8sSeedClient.DeleteDeployment(b.Operation.Shoot.SeedNamespace, common.KubeAddonManagerDeploymentName)
	if apierrors.IsNotFound(err) {
		return nil
	}
	return err
}

// DeployMachineControllerManager deploys the machine-controller-manager into the Shoot namespace in the Seed cluster. It is responsible
// for managing the worker nodes of the Shoot.
func (b *Botanist) DeployMachineControllerManager() error {
	var (
		name          = "machine-controller-manager"
		defaultValues = map[string]interface{}{
			"podAnnotations": map[string]interface{}{
				"checksum/secret-machine-controller-manager": b.CheckSums[name],
			},
			"namespace": map[string]interface{}{
				"uid": b.SeedNamespaceObject.UID,
			},
		}
	)

	values, err := b.InjectImages(defaultValues, b.K8sSeedClient.Version(), map[string]string{name: name})
	if err != nil {
		return err
	}

	if err := b.ApplyChartShoot(filepath.Join(common.ChartPath, "shoot-machines"), name, metav1.NamespaceSystem, nil, nil); err != nil {
		return err
	}

	return b.ApplyChartSeed(filepath.Join(common.ChartPath, "seed-controlplane", "charts", name), name, b.Operation.Shoot.SeedNamespace, nil, values)
}

// DeploySeedMonitoring will install the Helm release "seed-monitoring" in the Seed clusters. It comprises components
// to monitor the Shoot cluster whose control plane runs in the Seed cluster.
func (b *Botanist) DeploySeedMonitoring() error {
	var (
		kubecfgSecret    = b.Secrets["kubecfg"]
		basicAuth        = utils.CreateSHA1Secret(kubecfgSecret.Data["username"], kubecfgSecret.Data["password"])
		alertManagerHost = b.Seed.GetIngressFQDN("a", b.Shoot.Info.Name, b.Garden.ProjectName)
		grafanaHost      = b.Seed.GetIngressFQDN("g", b.Shoot.Info.Name, b.Garden.ProjectName)
		prometheusHost   = b.Seed.GetIngressFQDN("p", b.Shoot.Info.Name, b.Garden.ProjectName)
		replicas         = 1
	)

	if b.Shoot.Hibernated {
		replicas = 0
	}

	var (
		alertManagerConfig = map[string]interface{}{
			"ingress": map[string]interface{}{
				"basicAuthSecret": basicAuth,
				"host":            alertManagerHost,
			},
			"replicas": replicas,
		}
		grafanaConfig = map[string]interface{}{
			"ingress": map[string]interface{}{
				"basicAuthSecret": basicAuth,
				"host":            grafanaHost,
			},
			"replicas": replicas,
		}
		prometheusConfig = map[string]interface{}{
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
			"podAnnotations": map[string]interface{}{
				"checksum/secret-prometheus":                b.CheckSums["prometheus"],
				"checksum/secret-kube-apiserver-basic-auth": b.CheckSums["kube-apiserver-basic-auth"],
				"checksum/secret-vpn-ssh-keypair":           b.CheckSums["vpn-ssh-keypair"],
			},
			"replicas":           replicas,
			"apiserverServiceIP": common.ComputeClusterIP(b.Shoot.GetServiceNetwork(), 1),
		}
		kubeStateMetricsSeedConfig = map[string]interface{}{
			"replicas": replicas,
		}
		kubeStateMetricsShootConfig = map[string]interface{}{
			"replicas": replicas,
		}
	)

	alertManager, err := b.InjectImages(alertManagerConfig, b.K8sSeedClient.Version(), map[string]string{"alertmanager": "alertmanager", "configmap-reloader": "configmap-reloader"})
	if err != nil {
		return err
	}
	grafana, err := b.InjectImages(grafanaConfig, b.K8sSeedClient.Version(), map[string]string{"grafana": "grafana", "busybox": "busybox", "grafana-watcher": "grafana-watcher"})
	if err != nil {
		return err
	}
	prometheus, err := b.InjectImages(prometheusConfig, b.K8sSeedClient.Version(), map[string]string{
		"prometheus":         "prometheus",
		"configmap-reloader": "configmap-reloader",
		"vpn-seed":           "vpn-seed",
		"blackbox-exporter":  "blackbox-exporter",
	})
	if err != nil {
		return err
	}
	kubeStateMetricsSeed, err := b.InjectImages(kubeStateMetricsSeedConfig, b.K8sSeedClient.Version(), map[string]string{"kube-state-metrics": "kube-state-metrics"})
	if err != nil {
		return err
	}
	kubeStateMetricsShoot, err := b.InjectImages(kubeStateMetricsShootConfig, b.K8sSeedClient.Version(), map[string]string{"kube-state-metrics": "kube-state-metrics"})
	if err != nil {
		return err
	}

	values := map[string]interface{}{
		"global": map[string]interface{}{
			"shootKubeVersion": map[string]interface{}{
				"gitVersion": b.K8sShootClient.Version(),
			},
		},
		"alertmanager":             alertManager,
		"grafana":                  grafana,
		"prometheus":               prometheus,
		"kube-state-metrics-seed":  kubeStateMetricsSeed,
		"kube-state-metrics-shoot": kubeStateMetricsShoot,
	}

	alertingSMTPKeys := b.GetSecretKeysOfRole(common.GardenRoleAlertingSMTP)
	if len(alertingSMTPKeys) > 0 {
		emailConfigs := []map[string]interface{}{}
		for _, key := range alertingSMTPKeys {
			var (
				secret = b.Secrets[key]
				to     = string(secret.Data["to"])
			)
			if operatedBy, ok := b.Shoot.Info.Annotations[common.GardenOperatedBy]; ok && utils.TestEmail(operatedBy) {
				to = operatedBy
			}
			emailConfigs = append(emailConfigs, map[string]interface{}{
				"to":            to,
				"from":          string(secret.Data["from"]),
				"smarthost":     string(secret.Data["smarthost"]),
				"auth_username": string(secret.Data["auth_username"]),
				"auth_identity": string(secret.Data["auth_identity"]),
				"auth_password": string(secret.Data["auth_password"]),
			})
		}
		values["alertmanager"].(map[string]interface{})["email_configs"] = emailConfigs
	}

	return b.ApplyChartSeed(filepath.Join(common.ChartPath, "seed-monitoring"), fmt.Sprintf("%s-monitoring", b.Operation.Shoot.SeedNamespace), b.Operation.Shoot.SeedNamespace, nil, values)
}

// DeleteSeedMonitoring will delete the monitoring stack from the Seed cluster to avoid phantom alerts
// during the deletion process. More precisely, the Alertmanager and Prometheus StatefulSets will be
// deleted.
func (b *Botanist) DeleteSeedMonitoring() error {
	err := b.K8sSeedClient.DeleteStatefulSet(b.Operation.Shoot.SeedNamespace, common.AlertManagerDeploymentName)
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}
	err = b.K8sSeedClient.DeleteStatefulSet(b.Operation.Shoot.SeedNamespace, common.PrometheusDeploymentName)
	if apierrors.IsNotFound(err) {
		return nil
	}
	return err
}
