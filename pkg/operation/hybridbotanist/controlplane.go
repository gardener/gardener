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

	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/utils"
	corev1 "k8s.io/api/core/v1"
)

var chartPathControlPlane = filepath.Join(common.ChartPath, "seed-controlplane", "charts")

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

	etcdConfig := map[string]interface{}{}

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
	}
	return nil
}

// DeployCloudProviderConfig asks the Cloud Botanist to provide the cloud specific values for the cloud
// provider configuration. It will create a ConfigMap for it and store it in the Seed cluster.
func (b *HybridBotanist) DeployCloudProviderConfig() error {
	name := "cloud-provider-config"
	cloudProviderConfig, err := b.ShootCloudBotanist.GenerateCloudProviderConfig()
	if err != nil {
		return err
	}
	b.Botanist.CheckSums[name] = utils.ComputeSHA256Hex([]byte(cloudProviderConfig))

	defaultValues := map[string]interface{}{
		"cloudProviderConfig": cloudProviderConfig,
	}

	return b.ApplyChartSeed(filepath.Join(chartPathControlPlane, name), name, b.Shoot.SeedNamespace, nil, defaultValues)
}

// DeployKubeAPIServer asks the Cloud Botanist to provide the cloud specific configuration values for the
// kube-apiserver deployment.
func (b *HybridBotanist) DeployKubeAPIServer() error {
	var (
		name          = "kube-apiserver"
		loadBalancer  = b.Botanist.APIServerAddress
		basicAuthData = b.Secrets["kubecfg"].Data
	)

	loadBalancerIP, err := utils.WaitUntilDNSNameResolvable(loadBalancer)
	if err != nil {
		return err
	}

	defaultValues := map[string]interface{}{
		"advertiseAddress":         loadBalancerIP,
		"cloudProvider":            b.ShootCloudBotanist.GetCloudProviderName(),
		"kubernetesVersion":        b.Shoot.Info.Spec.Kubernetes.Version,
		"podNetwork":               b.Shoot.GetPodNetwork(),
		"serviceNetwork":           b.Shoot.GetServiceNetwork(),
		"securePort":               443,
		"livenessProbeCredentials": utils.EncodeBase64([]byte(fmt.Sprintf("%s:%s", basicAuthData["username"], basicAuthData["password"]))),
		"nodeNetwork":              b.Shoot.GetNodeNetwork(),
		"podAnnotations": map[string]interface{}{
			"checksum/secret-ca":                        b.CheckSums["ca"],
			"checksum/secret-kube-apiserver":            b.CheckSums[name],
			"checksum/secret-kube-aggregator":           b.CheckSums["kube-aggregator"],
			"checksum/secret-kube-apiserver-kubelet":    b.CheckSums["kube-apiserver-kubelet"],
			"checksum/secret-kube-apiserver-basic-auth": b.CheckSums["kube-apiserver-basic-auth"],
			"checksum/secret-vpn-ssh-keypair":           b.CheckSums["vpn-ssh-keypair"],
			"checksum/secret-service-account-key":       b.CheckSums["service-account-key"],
			"checksum/secret-cloudprovider":             b.CheckSums["cloudprovider"],
			"checksum/configmap-cloud-provider-config":  b.CheckSums["cloud-provider-config"],
		},
	}
	cloudValues, err := b.ShootCloudBotanist.GenerateKubeAPIServerConfig()
	if err != nil {
		return err
	}

	apiServerConfig := b.Shoot.Info.Spec.Kubernetes.KubeAPIServer
	if apiServerConfig != nil {
		defaultValues["featureGates"] = apiServerConfig.FeatureGates
		defaultValues["runtimeConfig"] = apiServerConfig.RuntimeConfig

		if apiServerConfig.OIDCConfig != nil {
			defaultValues["oidcConfig"] = apiServerConfig.OIDCConfig
		}
	}

	values, err := b.Botanist.InjectImages(defaultValues, b.K8sSeedClient.Version(), map[string]string{
		"hyperkube":         "hyperkube",
		"vpn-seed":          "vpn-seed",
		"blackbox-exporter": "blackbox-exporter",
	})
	if err != nil {
		return err
	}

	return b.ApplyChartSeed(filepath.Join(chartPathControlPlane, name), name, b.Shoot.SeedNamespace, values, cloudValues)
}

// DeployKubeControllerManager asks the Cloud Botanist to provide the cloud specific configuration values for the
// kube-controller-manager deployment.
func (b *HybridBotanist) DeployKubeControllerManager() error {
	name := "kube-controller-manager"

	defaultValues := map[string]interface{}{
		"cloudProvider":     b.ShootCloudBotanist.GetCloudProviderName(),
		"clusterName":       b.Shoot.SeedNamespace,
		"kubernetesVersion": b.Shoot.Info.Spec.Kubernetes.Version,
		"podNetwork":        b.Shoot.GetPodNetwork(),
		"serviceNetwork":    b.Shoot.GetServiceNetwork(),
		"configureRoutes":   true,
		"podAnnotations": map[string]interface{}{
			"checksum/secret-ca":                       b.CheckSums["ca"],
			"checksum/secret-kube-controller-manager":  b.CheckSums[name],
			"checksum/secret-service-account-key":      b.CheckSums["service-account-key"],
			"checksum/secret-cloudprovider":            b.CheckSums["cloudprovider"],
			"checksum/configmap-cloud-provider-config": b.CheckSums["cloud-provider-config"],
		},
	}
	cloudValues, err := b.ShootCloudBotanist.GenerateKubeControllerManagerConfig()
	if err != nil {
		return err
	}

	controllerManagerConfig := b.Shoot.Info.Spec.Kubernetes.KubeControllerManager
	if controllerManagerConfig != nil {
		defaultValues["featureGates"] = controllerManagerConfig.FeatureGates
	}

	values, err := b.Botanist.InjectImages(defaultValues, b.K8sSeedClient.Version(), map[string]string{"hyperkube": "hyperkube"})
	if err != nil {
		return err
	}

	return b.ApplyChartSeed(filepath.Join(chartPathControlPlane, name), name, b.Shoot.SeedNamespace, values, cloudValues)
}

// DeployKubeScheduler asks the Cloud Botanist to provide the cloud specific configuration values for the
// kube-scheduler deployment.
func (b *HybridBotanist) DeployKubeScheduler() error {
	var (
		name          = "kube-scheduler"
		defaultValues = map[string]interface{}{
			"kubernetesVersion": b.Shoot.Info.Spec.Kubernetes.Version,
			"podAnnotations": map[string]interface{}{
				"checksum/secret-kube-scheduler": b.CheckSums[name],
			},
		}
	)
	cloudValues, err := b.ShootCloudBotanist.GenerateKubeSchedulerConfig()
	if err != nil {
		return err
	}

	schedulerConfig := b.Shoot.Info.Spec.Kubernetes.KubeScheduler
	if schedulerConfig != nil {
		defaultValues["featureGates"] = schedulerConfig.FeatureGates
	}

	values, err := b.Botanist.InjectImages(defaultValues, b.K8sSeedClient.Version(), map[string]string{"hyperkube": "hyperkube"})
	if err != nil {
		return err
	}

	return b.ApplyChartSeed(filepath.Join(chartPathControlPlane, name), name, b.Shoot.SeedNamespace, values, cloudValues)
}
