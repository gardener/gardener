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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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
	var basicAuthData = b.Secrets["kubecfg"].Data

	loadBalancerIP, err := utils.WaitUntilDNSNameResolvable(b.Botanist.APIServerAddress)
	if err != nil {
		return err
	}

	defaultValues := map[string]interface{}{
		"etcdServicePort":          2379,
		"etcdMainServiceFqdn":      fmt.Sprintf("etcd-%s-client.%s.svc", common.EtcdRoleMain, b.Shoot.SeedNamespace),
		"etcdEventsServiceFqdn":    fmt.Sprintf("etcd-%s-client.%s.svc", common.EtcdRoleEvents, b.Shoot.SeedNamespace),
		"advertiseAddress":         loadBalancerIP,
		"cloudProvider":            b.ShootCloudBotanist.GetCloudProviderName(),
		"kubernetesVersion":        b.Shoot.Info.Spec.Kubernetes.Version,
		"podNetwork":               b.Shoot.GetPodNetwork(),
		"serviceNetwork":           b.Shoot.GetServiceNetwork(),
		"nodeNetwork":              b.Shoot.GetNodeNetwork(),
		"securePort":               443,
		"livenessProbeCredentials": utils.EncodeBase64([]byte(fmt.Sprintf("%s:%s", basicAuthData["username"], basicAuthData["password"]))),
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
