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

package hybridbotanist

import (
	"fmt"
	"path/filepath"

	"github.com/gardener/gardener/pkg/apis/garden/v1beta1/helper"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

var chartPathControlPlane = filepath.Join(common.ChartPath, "seed-controlplane", "charts")

// getResourcesForAPIServer returns the cpu and memory requirements for API server based on nodeCount
func getResourcesForAPIServer(nodeCount int) (string, string, string, string) {
	cpuRequest := "0m"
	memoryRequest := "0Mi"
	cpuLimit := "0m"
	memoryLimit := "0Mi"

	switch {
	case nodeCount <= 2:
		cpuRequest = "100m"
		memoryRequest = "512Mi"

		cpuLimit = "400m"
		memoryLimit = "700Mi"
	case nodeCount <= 10:
		cpuRequest = "400m"
		memoryRequest = "700Mi"

		cpuLimit = "800m"
		memoryLimit = "1000Mi"
	case nodeCount <= 50:
		cpuRequest = "700m"
		memoryRequest = "1000Mi"

		cpuLimit = "1300m"
		memoryLimit = "2000Mi"
	case nodeCount <= 100:
		cpuRequest = "1500m"
		memoryRequest = "3000Mi"

		cpuLimit = "3000m"
		memoryLimit = "4000Mi"
	default:
		cpuRequest = "2500m"
		memoryRequest = "4000Mi"

		cpuLimit = "4000m"
		memoryLimit = "6000Mi"
	}

	return cpuRequest, memoryRequest, cpuLimit, memoryLimit
}

// DeployETCD deploys two etcd clusters via StatefulSets. The first etcd cluster (called 'main') is used for all the
/// data the Shoot Kubernetes cluster needs to store, whereas the second etcd luster (called 'events') is only used to
// store the events data. The objectstore is also set up to store the backups.
func (b *HybridBotanist) DeployETCD() error {
	secretData, backupConfigData, err := b.SeedCloudBotanist.GenerateEtcdBackupConfig()
	if err != nil {
		return err
	}

	// Some cloud botanists do not yet support backup and won't return secret data.
	if secretData != nil {
		_, err = b.K8sSeedClient.CreateSecret(b.Shoot.SeedNamespace, common.BackupSecretName, corev1.SecretTypeOpaque, secretData, true)
		if err != nil {
			return err
		}
	}

	etcdConfig := map[string]interface{}{
		"podAnnotations": map[string]interface{}{
			"checksum/secret-ca":              b.CheckSums["ca"],
			"checksum/secret-etcd-server-tls": b.CheckSums["etcd-server-tls"],
			"checksum/secret-etcd-client-tls": b.CheckSums["etcd-client-tls"],
		},
	}

	// Some cloud botanists do not yet support backup and won't return backup config data.
	if backupConfigData != nil {
		etcdConfig["backup"] = backupConfigData
	}

	etcd, err := b.Botanist.InjectImages(etcdConfig, b.K8sSeedClient.Version(), map[string]string{"etcd": "etcd", "etcd-backup-restore": "etcd-backup-restore"})
	if err != nil {
		return err
	}

	for _, role := range []string{common.EtcdRoleMain, common.EtcdRoleEvents} {
		etcd["role"] = role
		if role == common.EtcdRoleEvents {
			etcd["backup"] = map[string]interface{}{
				"storageProvider": "", //No storage provider means no backup
			}
		}
		if err := b.ApplyChartSeed(filepath.Join(chartPathControlPlane, "etcd"), fmt.Sprintf("etcd-%s", role), b.Shoot.SeedNamespace, nil, etcd); err != nil {
			return err
		}
		if err := b.K8sSeedClient.DeleteService(b.Shoot.SeedNamespace, fmt.Sprintf("etcd-%s", role)); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	}

	return nil
}

// DeployCloudProviderConfig asks the Cloud Botanist to provide the cloud specific values for the cloud
// provider configuration. It will create a ConfigMap for it and store it in the Seed cluster.
func (b *HybridBotanist) DeployCloudProviderConfig() error {
	cloudProviderConfig, err := b.ShootCloudBotanist.GenerateCloudProviderConfig()
	if err != nil {
		return err
	}
	b.Botanist.CheckSums[common.CloudProviderConfigName] = utils.ComputeSHA256Hex([]byte(cloudProviderConfig))

	defaultValues := map[string]interface{}{
		"cloudProviderConfig": cloudProviderConfig,
	}

	return b.ApplyChartSeed(filepath.Join(chartPathControlPlane, common.CloudProviderConfigName), common.CloudProviderConfigName, b.Shoot.SeedNamespace, nil, defaultValues)
}

// RefreshCloudProviderConfig asks the Cloud Botanist to refresh the cloud provider config in case it stores
// the cloud provider credentials. The Cloud Botanist is expected to return the complete updated cloud config.
func (b *HybridBotanist) RefreshCloudProviderConfig() error {
	currentConfig, err := b.K8sSeedClient.GetConfigMap(b.Shoot.SeedNamespace, common.CloudProviderConfigName)
	if err != nil {
		return err
	}

	newConfig := b.ShootCloudBotanist.RefreshCloudProviderConfig(currentConfig.Data)
	_, err = b.K8sSeedClient.UpdateConfigMap(b.Shoot.SeedNamespace, common.CloudProviderConfigName, newConfig)
	return err
}

// DeployKubeAPIServer asks the Cloud Botanist to provide the cloud specific configuration values for the
// kube-apiserver deployment.
func (b *HybridBotanist) DeployKubeAPIServer() error {
	loadBalancerIP, err := utils.WaitUntilDNSNameResolvable(b.Botanist.APIServerAddress)
	if err != nil {
		return err
	}

	defaultValues := map[string]interface{}{
		"etcdServicePort":       2379,
		"etcdMainServiceFqdn":   fmt.Sprintf("etcd-%s-client.%s.svc", common.EtcdRoleMain, b.Shoot.SeedNamespace),
		"etcdEventsServiceFqdn": fmt.Sprintf("etcd-%s-client.%s.svc", common.EtcdRoleEvents, b.Shoot.SeedNamespace),
		"advertiseAddress":      loadBalancerIP,
		"cloudProvider":         b.ShootCloudBotanist.GetCloudProviderName(),
		"kubernetesVersion":     b.Shoot.Info.Spec.Kubernetes.Version,
		"shootNetworks": map[string]interface{}{
			"service": b.Shoot.GetServiceNetwork(),
		},
		"seedNetworks": map[string]interface{}{
			"service": b.Seed.Info.Spec.Networks.Services,
			"pod":     b.Seed.Info.Spec.Networks.Pods,
			"node":    b.Seed.Info.Spec.Networks.Nodes,
		},
		"securePort":       443,
		"probeCredentials": utils.EncodeBase64([]byte(fmt.Sprintf("%s:%s", b.Secrets["kubecfg"].Data["username"], b.Secrets["kubecfg"].Data["password"]))),
		"podAnnotations": map[string]interface{}{
			"checksum/secret-ca":                        b.CheckSums["ca"],
			"checksum/secret-kube-apiserver":            b.CheckSums[common.KubeAPIServerDeploymentName],
			"checksum/secret-kube-aggregator":           b.CheckSums["kube-aggregator"],
			"checksum/secret-kube-apiserver-kubelet":    b.CheckSums["kube-apiserver-kubelet"],
			"checksum/secret-kube-apiserver-basic-auth": b.CheckSums["kube-apiserver-basic-auth"],
			"checksum/secret-vpn-seed":                  b.CheckSums["vpn-seed"],
			"checksum/secret-vpn-seed-tlsauth":          b.CheckSums["vpn-seed-tlsauth"],
			"checksum/secret-service-account-key":       b.CheckSums["service-account-key"],
			"checksum/secret-cloudprovider":             b.CheckSums[common.CloudProviderSecretName],
			"checksum/configmap-cloud-provider-config":  b.CheckSums["cloud-provider-config"],
			"checksum/secret-etcd-client-tls":           b.CheckSums["etcd-client-tls"],
		},
	}
	cloudValues, err := b.ShootCloudBotanist.GenerateKubeAPIServerConfig()
	if err != nil {
		return err
	}

	if shootUsedAsSeed, _, _ := helper.IsUsedAsSeed(b.Shoot.Info); shootUsedAsSeed {
		defaultValues["replicas"] = 3
		defaultValues["minReplicas"] = 3
		defaultValues["maxReplicas"] = 3
		defaultValues["apiServerResources"] = map[string]interface{}{
			"limits": map[string]interface{}{
				"cpu":    "1500m",
				"memory": "4000Mi",
			},
		}
	} else {
		maxNodes := b.Shoot.GetNodeCount()
		// Get current kube-apiserver deployment
		existingAPIServerDeployment, err := b.K8sSeedClient.GetDeployment(b.Shoot.SeedNamespace, common.KubeAPIServerDeploymentName)
		if err == nil {
			// As kube-apiserver HPA manages the number of replicas in big clusters
			// maintain current number of replicas
			// otherwise keep the value to default
			if maxNodes >= common.EnableHPANodeCount && existingAPIServerDeployment.Spec.Replicas != nil && *existingAPIServerDeployment.Spec.Replicas > 0 {
				defaultValues["replicas"] = *existingAPIServerDeployment.Spec.Replicas
				defaultValues["maxReplicas"] = 4
			}
		}

		cpuRequest, memoryRequest, cpuLimit, memoryLimit := getResourcesForAPIServer(maxNodes)
		defaultValues["apiServerResources"] = map[string]interface{}{
			"limits": map[string]interface{}{
				"cpu":    cpuLimit,
				"memory": memoryLimit,
			},
			"requests": map[string]interface{}{
				"cpu":    cpuRequest,
				"memory": memoryRequest,
			},
		}
	}

	var (
		apiServerConfig  = b.Shoot.Info.Spec.Kubernetes.KubeAPIServer
		admissionPlugins = kubernetes.GetAdmissionPluginsForVersion(b.Shoot.Info.Spec.Kubernetes.Version)
	)

	if apiServerConfig != nil {
		defaultValues["featureGates"] = apiServerConfig.FeatureGates
		defaultValues["runtimeConfig"] = apiServerConfig.RuntimeConfig

		if apiServerConfig.OIDCConfig != nil {
			defaultValues["oidcConfig"] = apiServerConfig.OIDCConfig
		}

		for _, plugin := range apiServerConfig.AdmissionPlugins {
			pluginOverwritesDefault := false

			for i, defaultPlugin := range admissionPlugins {
				if defaultPlugin.Name == plugin.Name {
					pluginOverwritesDefault = true
					admissionPlugins[i] = plugin
					break
				}
			}

			if !pluginOverwritesDefault {
				admissionPlugins = append(admissionPlugins, plugin)
			}
		}
	}
	defaultValues["admissionPlugins"] = admissionPlugins

	values, err := b.Botanist.InjectImages(defaultValues, b.K8sSeedClient.Version(), map[string]string{
		"hyperkube":         "hyperkube",
		"vpn-seed":          "vpn-seed",
		"blackbox-exporter": "blackbox-exporter",
	})
	if err != nil {
		return err
	}

	// Delete keypair for VPN implementation based on SSH (only meaningful for old clusters).
	if err := b.K8sSeedClient.DeleteSecret(b.Shoot.SeedNamespace, "vpn-ssh-keypair"); err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	return b.ApplyChartSeed(filepath.Join(chartPathControlPlane, common.KubeAPIServerDeploymentName), common.KubeAPIServerDeploymentName, b.Shoot.SeedNamespace, values, cloudValues)
}

// DeployKubeControllerManager asks the Cloud Botanist to provide the cloud specific configuration values for the
// kube-controller-manager deployment.
func (b *HybridBotanist) DeployKubeControllerManager() error {
	defaultValues := map[string]interface{}{
		"cloudProvider":     b.ShootCloudBotanist.GetCloudProviderName(),
		"clusterName":       b.Shoot.SeedNamespace,
		"kubernetesVersion": b.Shoot.Info.Spec.Kubernetes.Version,
		"podNetwork":        b.Shoot.GetPodNetwork(),
		"serviceNetwork":    b.Shoot.GetServiceNetwork(),
		"configureRoutes":   true,
		"podAnnotations": map[string]interface{}{
			"checksum/secret-ca":                       b.CheckSums["ca"],
			"checksum/secret-kube-controller-manager":  b.CheckSums[common.KubeControllerManagerDeploymentName],
			"checksum/secret-service-account-key":      b.CheckSums["service-account-key"],
			"checksum/secret-cloudprovider":            b.CheckSums[common.CloudProviderSecretName],
			"checksum/configmap-cloud-provider-config": b.CheckSums["cloud-provider-config"],
		},
	}
	cloudValues, err := b.ShootCloudBotanist.GenerateKubeControllerManagerConfig()
	if err != nil {
		return err
	}

	if shootUsedAsSeed, _, _ := helper.IsUsedAsSeed(b.Shoot.Info); shootUsedAsSeed {
		defaultValues["resources"] = map[string]interface{}{
			"limits": map[string]interface{}{
				"cpu":    "750m",
				"memory": "1Gi",
			},
		}
	}

	controllerManagerConfig := b.Shoot.Info.Spec.Kubernetes.KubeControllerManager
	if controllerManagerConfig != nil {
		defaultValues["featureGates"] = controllerManagerConfig.FeatureGates
	}

	values, err := b.Botanist.InjectImages(defaultValues, b.K8sSeedClient.Version(), map[string]string{"hyperkube": "hyperkube"})
	if err != nil {
		return err
	}

	return b.ApplyChartSeed(filepath.Join(chartPathControlPlane, common.KubeControllerManagerDeploymentName), common.KubeControllerManagerDeploymentName, b.Shoot.SeedNamespace, values, cloudValues)
}

// DeployKubeScheduler asks the Cloud Botanist to provide the cloud specific configuration values for the
// kube-scheduler deployment.
func (b *HybridBotanist) DeployKubeScheduler() error {
	defaultValues := map[string]interface{}{
		"kubernetesVersion": b.Shoot.Info.Spec.Kubernetes.Version,
		"podAnnotations": map[string]interface{}{
			"checksum/secret-kube-scheduler": b.CheckSums[common.KubeSchedulerDeploymentName],
		},
	}
	cloudValues, err := b.ShootCloudBotanist.GenerateKubeSchedulerConfig()
	if err != nil {
		return err
	}

	if shootUsedAsSeed, _, _ := helper.IsUsedAsSeed(b.Shoot.Info); shootUsedAsSeed {
		defaultValues["resources"] = map[string]interface{}{
			"limits": map[string]interface{}{
				"cpu":    "300m",
				"memory": "350Mi",
			},
		}
	}

	schedulerConfig := b.Shoot.Info.Spec.Kubernetes.KubeScheduler
	if schedulerConfig != nil {
		defaultValues["featureGates"] = schedulerConfig.FeatureGates
	}

	values, err := b.Botanist.InjectImages(defaultValues, b.K8sSeedClient.Version(), map[string]string{"hyperkube": "hyperkube"})
	if err != nil {
		return err
	}

	return b.ApplyChartSeed(filepath.Join(chartPathControlPlane, common.KubeSchedulerDeploymentName), common.KubeSchedulerDeploymentName, b.Shoot.SeedNamespace, values, cloudValues)
}
