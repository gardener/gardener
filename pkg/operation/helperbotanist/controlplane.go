// Copyright 2018 The Gardener Authors.
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

package helperbotanist

import (
	"fmt"
	"path/filepath"

	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/utils"
	corev1 "k8s.io/api/core/v1"
)

var chartPathControlPlane = filepath.Join(common.ChartPath, "seed-controlplane", "charts")

// DeployETCD deploys two etcd clusters (either via StatefulSets or via the etcd-operator). The first etcd cluster
// (called 'main') is used for all the data the Shoot Kubernetes cluster needs to store, whereas the second etcd
// cluster (called 'events') is only used to store the events data. The objectstore is also set up to store the backups.
func (b *HelperBotanist) DeployETCD() error {
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

	// Some cloud botanists do not yet support backup and won't return backup config data.
	etcdConfig := map[string]interface{}{
		"kind": "StatefulSet",
	}

	if backupConfigData != nil {
		etcdConfig["kind"] = "EtcdCluster"
		etcdConfig["backup"] = backupConfigData
	}

	etcd, err := b.Botanist.InjectImages(etcdConfig, b.K8sSeedClient.Version(), map[string]string{"etcd": "etcd"})
	if err != nil {
		return err
	}

	for _, role := range []string{common.EtcdRoleMain, common.EtcdRoleEvents} {
		etcd["role"] = role
		if err := b.ApplyChartSeed(filepath.Join(chartPathControlPlane, "etcd"), fmt.Sprintf("etcd-%s", role), b.Shoot.SeedNamespace, nil, etcd); err != nil {
			return err
		}
	}
	return nil
}

// DeployCloudProviderConfig asks the Cloud Botanist to provide the cloud specific values for the cloud
// provider configuration. It will create a ConfigMap for it and store it in the Seed cluster.
func (b *HelperBotanist) DeployCloudProviderConfig() error {
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
func (b *HelperBotanist) DeployKubeAPIServer() error {
	name := "kube-apiserver"
	loadBalancer := b.Botanist.APIServerAddress
	loadBalancerIP, err := utils.WaitUntilDNSNameResolvable(loadBalancer)
	if err != nil {
		return err
	}

	defaultValues := map[string]interface{}{
		"advertiseAddress":  loadBalancerIP,
		"cloudProvider":     b.ShootCloudBotanist.GetCloudProviderName(),
		"kubernetesVersion": b.Shoot.Info.Spec.Kubernetes.Version,
		"podNetwork":        b.Shoot.GetPodNetwork(),
		"serviceNetwork":    b.Shoot.GetServiceNetwork(),
		"nodeNetwork":       b.Shoot.GetNodeNetwork(),
		"featureGates":      b.Shoot.Info.Spec.Kubernetes.KubeAPIServer.FeatureGates,
		"runtimeConfig":     b.Shoot.Info.Spec.Kubernetes.KubeAPIServer.RuntimeConfig,
		"podAnnotations": map[string]interface{}{
			"checksum/secret-ca":                        b.CheckSums["ca"],
			"checksum/secret-kube-apiserver":            b.CheckSums[name],
			"checksum/secret-kube-aggregator":           b.CheckSums["kube-aggregator"],
			"checksum/secret-kube-apiserver-kubelet":    b.CheckSums["kube-apiserver-kubelet"],
			"checksum/secret-kube-apiserver-basic-auth": b.CheckSums["kube-apiserver-basic-auth"],
			"checksum/secret-vpn-ssh-keypair":           b.CheckSums["vpn-ssh-keypair"],
			"checksum/secret-cloudprovider":             b.CheckSums["cloudprovider"],
			"checksum/configmap-cloud-provider-config":  b.CheckSums["cloud-provider-config"],
		},
	}
	cloudValues, err := b.ShootCloudBotanist.GenerateKubeAPIServerConfig()
	if err != nil {
		return err
	}
	oidcConfig := b.Shoot.Info.Spec.Kubernetes.KubeAPIServer.OIDCConfig
	if oidcConfig != nil {
		defaultValues["oidcConfig"] = oidcConfig
	}

	values, err := b.Botanist.InjectImages(defaultValues, b.K8sSeedClient.Version(), map[string]string{"hyperkube": "hyperkube", "vpn-seed": "vpn-seed"})
	if err != nil {
		return err
	}

	return b.ApplyChartSeed(filepath.Join(chartPathControlPlane, name), name, b.Shoot.SeedNamespace, values, cloudValues)
}

// DeployKubeControllerManager asks the Cloud Botanist to provide the cloud specific configuration values for the
// kube-controller-manager deployment.
func (b *HelperBotanist) DeployKubeControllerManager() error {
	name := "kube-controller-manager"

	defaultValues := map[string]interface{}{
		"cloudProvider":     b.ShootCloudBotanist.GetCloudProviderName(),
		"clusterName":       b.Shoot.SeedNamespace,
		"kubernetesVersion": b.Shoot.Info.Spec.Kubernetes.Version,
		"podNetwork":        b.Shoot.GetPodNetwork(),
		"serviceNetwork":    b.Shoot.GetServiceNetwork(),
		"configureRoutes":   true,
		"featureGates":      b.Shoot.Info.Spec.Kubernetes.KubeControllerManager.FeatureGates,
		"podAnnotations": map[string]interface{}{
			"checksum/secret-ca":                       b.CheckSums["ca"],
			"checksum/secret-kube-apiserver":           b.CheckSums["kube-apiserver"],
			"checksum/secret-kube-controller-manager":  b.CheckSums[name],
			"checksum/secret-cloudprovider":            b.CheckSums["cloudprovider"],
			"checksum/configmap-cloud-provider-config": b.CheckSums["cloud-provider-config"],
		},
	}
	cloudValues, err := b.ShootCloudBotanist.GenerateKubeControllerManagerConfig()
	if err != nil {
		return err
	}

	values, err := b.Botanist.InjectImages(defaultValues, b.K8sSeedClient.Version(), map[string]string{"hyperkube": "hyperkube"})
	if err != nil {
		return err
	}

	return b.ApplyChartSeed(filepath.Join(chartPathControlPlane, name), name, b.Shoot.SeedNamespace, values, cloudValues)
}

// DeployKubeScheduler asks the Cloud Botanist to provide the cloud specific configuration values for the
// kube-scheduler deployment.
func (b *HelperBotanist) DeployKubeScheduler() error {
	var (
		name          = "kube-scheduler"
		defaultValues = map[string]interface{}{
			"kubernetesVersion": b.Shoot.Info.Spec.Kubernetes.Version,
			"featureGates":      b.Shoot.Info.Spec.Kubernetes.KubeScheduler.FeatureGates,
			"podAnnotations": map[string]interface{}{
				"checksum/secret-kube-scheduler": b.CheckSums[name],
			},
		}
	)
	cloudValues, err := b.ShootCloudBotanist.GenerateKubeSchedulerConfig()
	if err != nil {
		return err
	}

	values, err := b.Botanist.InjectImages(defaultValues, b.K8sSeedClient.Version(), map[string]string{"hyperkube": "hyperkube"})
	if err != nil {
		return err
	}

	return b.ApplyChartSeed(filepath.Join(chartPathControlPlane, name), name, b.Shoot.SeedNamespace, values, cloudValues)
}
