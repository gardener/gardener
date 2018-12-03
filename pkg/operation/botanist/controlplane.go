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
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/gardener/gardener/pkg/features"
	"github.com/gardener/gardener/pkg/operation/certmanagement"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var chartPathControlPlane = filepath.Join(common.ChartPath, "seed-controlplane", "charts")

// DeployNamespace creates a namespace in the Seed cluster which is used to deploy all the control plane
// components for the Shoot cluster. Moreover, the cloud provider configuration and all the secrets will be
// stored as ConfigMaps/Secrets.
func (b *Botanist) DeployNamespace() error {
	namespace, err := b.K8sSeedClient.CreateNamespace(&corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: getShootAnnotations(b.Shoot.Info.Annotations, b.Shoot.Info.Status.UID),
			Name:        b.Shoot.SeedNamespace,
			Labels: map[string]string{
				common.GardenRole: common.GardenRoleShoot,
			},
		},
	}, true)
	if err != nil {
		return err
	}
	b.SeedNamespaceObject = namespace
	return nil
}

func getShootAnnotations(annotations map[string]string, uid types.UID) map[string]string {
	shootAnnotations := map[string]string{
		common.ShootUID: string(uid),
	}
	for key, value := range annotations {
		if strings.HasPrefix(key, common.AnnotateSeedNamespacePrefix) {
			shootAnnotations[key] = value
		}
	}
	return shootAnnotations
}

// DeployBackupNamespace creates a namespace in the Seed cluster from info in shoot object, which is used to deploy all the backup infrastructure
// realted resources for shoot cluster. Moreover, the terraform configuration and all the secrets will be
// stored as ConfigMaps/Secrets.
func (b *Botanist) DeployBackupNamespace() error {
	_, err := b.K8sSeedClient.CreateNamespace(&corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: common.GenerateBackupNamespaceName(b.BackupInfrastructure.Name),
			Labels: map[string]string{
				common.GardenRole: common.GardenRoleBackup,
			},
		},
	}, true)
	return err
}

// DeleteNamespace deletes the namespace in the Seed cluster which holds the control plane components. The built-in
// garbage collection in Kubernetes will automatically delete all resources which belong to this namespace. This
// comprises volumes and load balancers as well.
func (b *Botanist) DeleteNamespace() error {
	return b.deleteNamespace(b.Shoot.SeedNamespace)
}

// DeleteBackupNamespace deletes the namespace in the Seed cluster which holds the backup infrastructure state. The built-in
// garbage collection in Kubernetes will automatically delete all resources which belong to this namespace.
func (b *Botanist) DeleteBackupNamespace() error {
	return b.deleteNamespace(common.GenerateBackupNamespaceName(b.BackupInfrastructure.Name))
}

func (b *Botanist) deleteNamespace(name string) error {
	err := b.K8sSeedClient.DeleteNamespace(name)
	if apierrors.IsNotFound(err) || apierrors.IsConflict(err) {
		return nil
	}
	return err
}

// DeployCloudMetadataServiceNetworkPolicy creates a global network policy that allows access to the meta-data service only from
// the cloud-controller-manager and the kube-controller-manager
func (b *Botanist) DeployCloudMetadataServiceNetworkPolicy() error {
	return b.ApplyChartSeed(filepath.Join(chartPathControlPlane, "cloud-metadata-service"), "cloud-metadata-service", b.Shoot.SeedNamespace, nil, nil)
}

// DeleteKubeAPIServer deletes the kube-apiserver deployment in the Seed cluster which holds the Shoot's control plane.
func (b *Botanist) DeleteKubeAPIServer() error {
	err := b.K8sSeedClient.DeleteDeployment(b.Shoot.SeedNamespace, common.KubeAPIServerDeploymentName)
	if apierrors.IsNotFound(err) {
		return nil
	}
	return err
}

// RefreshCloudControllerManagerChecksums updates the cloud provider checksum in the cloud-controller-manager pod spec template.
func (b *Botanist) RefreshCloudControllerManagerChecksums() error {
	if _, err := b.K8sSeedClient.GetDeployment(b.Shoot.SeedNamespace, common.CloudControllerManagerDeploymentName); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}
	return b.patchDeploymentCloudProviderChecksums(common.CloudControllerManagerDeploymentName)
}

// RefreshKubeControllerManagerChecksums updates the cloud provider checksum in the kube-controller-manager pod spec template.
func (b *Botanist) RefreshKubeControllerManagerChecksums() error {
	if _, err := b.K8sSeedClient.GetDeployment(b.Shoot.SeedNamespace, common.KubeControllerManagerDeploymentName); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}
	return b.patchDeploymentCloudProviderChecksums(common.KubeControllerManagerDeploymentName)
}

// DeployBackupInfrastructure creates a BackupInfrastructure resource into the project namespace of shoot on garden cluster.
// BackupInfrastructure controller acting on resource will actually create required cloud resources and updates the status.
func (b *Botanist) DeployBackupInfrastructure() error {
	return b.ApplyChartGarden(filepath.Join(common.ChartPath, "garden-project", "charts", "backup-infrastructure"), "backup-infrastructure", b.Operation.Shoot.Info.Namespace, nil, map[string]interface{}{
		"backupInfrastructure": map[string]interface{}{
			"name": common.GenerateBackupInfrastructureName(b.Shoot.SeedNamespace, b.Shoot.Info.Status.UID),
		},
		"seed": map[string]interface{}{
			"name": b.Seed.Info.Name,
		},
		"shoot": map[string]interface{}{
			"name": b.Shoot.Info.Name,
			"uid":  b.Shoot.Info.Status.UID,
		},
	})
}

// DeleteBackupInfrastructure deletes the sets deletionTimestamp on the backupInfrastructure resource in the Garden namespace
// which is responsible for actual deletion of cloud resource for Shoot's backup infrastructure.
func (b *Botanist) DeleteBackupInfrastructure() error {
	err := b.K8sGardenClient.Garden().GardenV1beta1().BackupInfrastructures(b.Shoot.Info.Namespace).Delete(common.GenerateBackupInfrastructureName(b.Shoot.SeedNamespace, b.Shoot.Info.Status.UID), nil)
	if apierrors.IsNotFound(err) {
		return nil
	}
	return err
}

// DeleteKubeAddonManager deletes the kube-addon-manager deployment in the Seed cluster which holds the Shoot's control plane. It
// needs to be deleted before trying to remove any resources in the Shoot cluster, otherwise it will automatically recreate
// them and block the infrastructure deletion.
func (b *Botanist) DeleteKubeAddonManager() error {
	err := b.K8sSeedClient.DeleteDeployment(b.Shoot.SeedNamespace, common.KubeAddonManagerDeploymentName)
	if apierrors.IsNotFound(err) {
		return nil
	}
	return err
}

// DeployMachineControllerManager deploys the machine-controller-manager into the Shoot namespace in the Seed cluster. It is responsible
// for managing the worker nodes of the Shoot.
func (b *Botanist) DeployMachineControllerManager() error {
	var (
		name          = common.MachineControllerManagerDeploymentName
		defaultValues = map[string]interface{}{
			"podAnnotations": map[string]interface{}{
				"checksum/secret-machine-controller-manager": b.CheckSums[name],
			},
			"namespace": map[string]interface{}{
				"uid": b.SeedNamespaceObject.UID,
			},
		}
	)

	// If the shoot is hibernated then we want to scale down the machine-controller-manager. However, we want to first allow it to delete
	// all remaining worker nodes. Hence, we cannot set the replicas=0 here (otherwise it would be offline and not able to delete the nodes).
	if b.Shoot.IsHibernated {
		deployment, err := b.K8sSeedClient.GetDeployment(b.Shoot.SeedNamespace, common.MachineControllerManagerDeploymentName)
		if err != nil {
			return err
		}
		defaultValues["replicas"] = *deployment.Spec.Replicas
	}

	values, err := b.InjectImages(defaultValues, b.SeedVersion(), b.ShootVersion(), common.MachineControllerManagerImageName)
	if err != nil {
		return err
	}

	return b.ApplyChartSeed(filepath.Join(chartPathControlPlane, name), name, b.Shoot.SeedNamespace, nil, values)
}

// DeployClusterAutoscaler deploys the cluster-autoscaler into the Shoot namespace in the Seed cluster. It is responsible
// for automatically scaling the worker pools of the Shoot.
func (b *Botanist) DeployClusterAutoscaler() error {
	if !b.Shoot.WantsClusterAutoscaler {
		return b.DeleteClusterAutoscaler()
	}

	var (
		name        = "cluster-autoscaler"
		workerPools = []map[string]interface{}{}
	)

	for _, worker := range b.MachineDeployments {
		// Skip worker pools for which min=0. Auto scaler cannot handle worker pools having a min count of 0.
		if worker.Minimum == 0 {
			continue
		}

		workerPools = append(workerPools, map[string]interface{}{
			"name": worker.Name,
			"min":  worker.Minimum,
			"max":  worker.Maximum,
		})
	}

	defaultValues := map[string]interface{}{
		"podAnnotations": map[string]interface{}{
			"checksum/secret-cluster-autoscaler": b.CheckSums[name],
		},
		"namespace": map[string]interface{}{
			"uid": b.SeedNamespaceObject.UID,
		},
		"replicas":    b.Shoot.GetReplicas(1),
		"workerPools": workerPools,
	}

	values, err := b.InjectImages(defaultValues, b.SeedVersion(), b.ShootVersion(), common.ClusterAutoscalerImageName)
	if err != nil {
		return err
	}

	return b.ApplyChartSeed(filepath.Join(chartPathControlPlane, name), name, b.Shoot.SeedNamespace, nil, values)
}

// DeleteClusterAutoscaler deletes the cluster-autoscaler deployment in the Seed cluster which holds the Shoot's control plane.
func (b *Botanist) DeleteClusterAutoscaler() error {
	err := b.K8sSeedClient.DeleteDeployment(b.Shoot.SeedNamespace, common.ClusterAutoscalerDeploymentName)
	if apierrors.IsNotFound(err) {
		return nil
	}
	return err
}

// DeploySeedMonitoring will install the Helm release "seed-monitoring" in the Seed clusters. It comprises components
// to monitor the Shoot cluster whose control plane runs in the Seed cluster.
func (b *Botanist) DeploySeedMonitoring() error {
	var (
		credentials      = b.Secrets["monitoring-ingress-credentials"]
		basicAuth        = utils.CreateSHA1Secret(credentials.Data["username"], credentials.Data["password"])
		alertManagerHost = b.Seed.GetIngressFQDN("a", b.Shoot.Info.Name, b.Garden.Project.Name)
		grafanaHost      = b.Seed.GetIngressFQDN("g", b.Shoot.Info.Name, b.Garden.Project.Name)
		prometheusHost   = b.ComputePrometheusIngressFQDN()
	)

	var (
		alertManagerConfig = map[string]interface{}{
			"ingress": map[string]interface{}{
				"basicAuthSecret": basicAuth,
				"host":            alertManagerHost,
			},
			"replicas": b.Shoot.GetReplicas(1),
		}
		grafanaConfig = map[string]interface{}{
			"ingress": map[string]interface{}{
				"basicAuthSecret": basicAuth,
				"host":            grafanaHost,
			},
			"replicas": b.Shoot.GetReplicas(1),
		}
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
				"checksum/secret-prometheus":                b.CheckSums["prometheus"],
				"checksum/secret-kube-apiserver-basic-auth": b.CheckSums["kube-apiserver-basic-auth"],
				"checksum/secret-vpn-seed":                  b.CheckSums["vpn-seed"],
				"checksum/secret-vpn-seed-tlsauth":          b.CheckSums["vpn-seed-tlsauth"],
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
				},
			},
			"shoot": map[string]interface{}{
				"apiserver": b.K8sShootClient.RESTConfig().Host,
			},
		}
		kubeStateMetricsSeedConfig = map[string]interface{}{
			"replicas": b.Shoot.GetReplicas(1),
		}
		kubeStateMetricsShootConfig = map[string]interface{}{
			"replicas": b.Shoot.GetReplicas(1),
		}
	)

	alertManager, err := b.InjectImages(alertManagerConfig, b.SeedVersion(), b.ShootVersion(), common.AlertManagerImageName, common.ConfigMapReloaderImageName)
	if err != nil {
		return err
	}
	grafana, err := b.InjectImages(grafanaConfig, b.SeedVersion(), b.ShootVersion(), common.GrafanaImageName, common.BusyboxImageName)
	if err != nil {
		return err
	}
	prometheus, err := b.InjectImages(prometheusConfig, b.SeedVersion(), b.ShootVersion(),
		common.PrometheusImageName,
		common.ConfigMapReloaderImageName,
		common.VPNSeedImageName,
		common.BlackboxExporterImageName,
	)
	if err != nil {
		return err
	}
	kubeStateMetricsSeed, err := b.InjectImages(kubeStateMetricsSeedConfig, b.SeedVersion(), b.ShootVersion(), common.KubeStateMetricsImageName)
	if err != nil {
		return err
	}
	kubeStateMetricsShoot, err := b.InjectImages(kubeStateMetricsShootConfig, b.SeedVersion(), b.ShootVersion(), common.KubeStateMetricsImageName)
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
			to, ok := b.Shoot.Info.Annotations[common.GardenOperatedBy]
			if !ok || !utils.TestEmail(to) {
				continue
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
		values["alertmanager"].(map[string]interface{})["emailConfigs"] = emailConfigs
	}

	return b.ApplyChartSeed(filepath.Join(common.ChartPath, "seed-monitoring"), fmt.Sprintf("%s-monitoring", b.Shoot.SeedNamespace), b.Shoot.SeedNamespace, nil, values)
}

// DeleteSeedMonitoring will delete the monitoring stack from the Seed cluster to avoid phantom alerts
// during the deletion process. More precisely, the Alertmanager and Prometheus StatefulSets will be
// deleted.
func (b *Botanist) DeleteSeedMonitoring() error {
	err := b.K8sSeedClient.DeleteStatefulSet(b.Shoot.SeedNamespace, common.AlertManagerStatefulSetName)
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}
	err = b.K8sSeedClient.DeleteStatefulSet(b.Shoot.SeedNamespace, common.PrometheusStatefulSetName)
	if apierrors.IsNotFound(err) {
		return nil
	}
	return err
}

// patchDeploymentCloudProviderChecksums updates the cloud provider checksums on the given deployment.
func (b *Botanist) patchDeploymentCloudProviderChecksums(deploymentName string) error {
	type jsonPatch struct {
		Op    string `json:"op"`
		Path  string `json:"path"`
		Value string `json:"value"`
	}

	patch := []jsonPatch{
		{
			Op:    "replace",
			Path:  "/spec/template/metadata/annotations/checksum~1secret-cloudprovider",
			Value: b.CheckSums[common.CloudProviderSecretName],
		},
		{
			Op:    "replace",
			Path:  "/spec/template/metadata/annotations/checksum~1configmap-cloud-provider-config",
			Value: b.CheckSums[common.CloudProviderConfigName],
		},
	}

	body, err := json.Marshal(patch)
	if err != nil {
		return err
	}

	_, err = b.K8sSeedClient.PatchDeployment(b.Shoot.SeedNamespace, deploymentName, body)
	return err
}

// DeploySeedLogging will install the Helm release "seed-bootstrap/charts/elastic-kibana-curator" in the Seed clusters.
func (b *Botanist) DeploySeedLogging() error {
	if !features.ControllerFeatureGate.Enabled(features.Logging) {
		return common.DeleteLoggingStack(b.K8sSeedClient, b.Shoot.SeedNamespace)
	}

	var (
		credentials = b.Secrets["logging-ingress-credentials"]
		basicAuth   = utils.CreateSHA1Secret(credentials.Data["username"], credentials.Data["password"])
		kibanaHost  = b.Seed.GetIngressFQDN("k", b.Shoot.Info.Name, b.Garden.Project.Name)
	)

	images, err := b.InjectImages(map[string]interface{}{}, b.K8sSeedClient.Version(), b.K8sSeedClient.Version(),
		common.ElasticsearchImageName,
		common.CuratorImageName,
		common.KibanaImageName,
		common.AlpineImageName,
	)
	if err != nil {
		return err
	}

	elasticKibanaCurator := map[string]interface{}{
		"ingress": map[string]interface{}{
			"basicAuthSecret": basicAuth,
			"host":            kibanaHost,
		},
		"elasticsearch": map[string]interface{}{
			"elasticsearchReplicas": b.Shoot.GetReplicas(1),
		},
		"kibanaReplicas": b.Shoot.GetReplicas(1),
		"global":         images,
	}

	return b.ApplyChartSeed(filepath.Join(common.ChartPath, "seed-bootstrap", "charts", "elastic-kibana-curator"), fmt.Sprintf("%s-logging", b.Operation.Shoot.SeedNamespace), b.Operation.Shoot.SeedNamespace, nil, elasticKibanaCurator)
}

// WakeUpControllers scales the replicas to 1 for the following deployments which are needed in case of shoot deletion:
// * cloud-controller-manager
// * kube-controller-manager
// * machine-controller-manager
func (b *Botanist) WakeUpControllers() error {
	if _, err := b.K8sSeedClient.ScaleDeployment(b.Shoot.SeedNamespace, common.KubeControllerManagerDeploymentName, 1); err != nil {
		return err
	}
	if _, err := b.K8sSeedClient.ScaleDeployment(b.Shoot.SeedNamespace, common.CloudControllerManagerDeploymentName, 1); err != nil {
		return err
	}
	if _, err := b.K8sSeedClient.ScaleDeployment(b.Shoot.SeedNamespace, common.MachineControllerManagerDeploymentName, 1); err != nil {
		return err
	}
	return nil
}

// DeployCertBroker deploys the Cert-Broker to the Shoot namespace in the Seed.
func (b *Botanist) DeployCertBroker() error {
	certManagementEnabled := features.ControllerFeatureGate.Enabled(features.CertificateManagement)
	if !certManagementEnabled {
		return b.DeleteCertBroker()
	}

	certificateManagement, ok := b.Secrets[common.GardenRoleCertificateManagement]
	if !ok {
		return fmt.Errorf("certificate management is enabled but no secret with role %s could be found", common.GardenRoleCertificateManagement)
	}
	certificateManagementConfig, err := certmanagement.RetrieveCertificateManagementConfig(certificateManagement)
	if err != nil {
		return fmt.Errorf("certificate management configuration could not be created %v", err)
	}

	var dns []interface{}
	for _, route53Provider := range certificateManagementConfig.Providers.Route53 {
		route53values := createDNSProviderValuesForDomain(&route53Provider, *b.Shoot.Info.Spec.DNS.Domain)
		dns = append(dns, route53values...)
	}
	for _, cloudDNSProvider := range certificateManagementConfig.Providers.CloudDNS {
		cloudDNSValues := createDNSProviderValuesForDomain(&cloudDNSProvider, *b.Shoot.Info.Spec.DNS.Domain)
		dns = append(dns, cloudDNSValues...)
	}

	certBrokerConfig := map[string]interface{}{
		"replicas": b.Shoot.GetReplicas(1),
		"certbroker": map[string]interface{}{
			"namespace":           b.Shoot.SeedNamespace,
			"targetClusterSecret": common.CertBrokerResourceName,
		},
		"certmanager": map[string]interface{}{
			"clusterissuer": certificateManagementConfig.ClusterIssuerName,
			"dns":           dns,
		},
		"podAnnotations": map[string]interface{}{
			"checksum/secret-cert-broker": b.CheckSums[common.CertBrokerResourceName],
		},
	}

	certBroker, err := b.InjectImages(certBrokerConfig, b.K8sSeedClient.Version(), b.K8sSeedClient.Version(), common.CertBrokerImageName)
	if err != nil {
		return nil
	}

	return b.ApplyChartSeed(filepath.Join(chartPathControlPlane, "cert-broker"), "cert-broker", b.Shoot.SeedNamespace, nil, certBroker)
}

func createDNSProviderValuesForDomain(config certmanagement.DNSProviderConfig, shootDomain string) []interface{} {
	var dnsConfig []interface{}
	for _, domain := range config.DomainNames() {
		if strings.HasSuffix(shootDomain, domain) {
			dnsConfig = append(dnsConfig, map[string]interface{}{
				"domain":   shootDomain,
				"provider": config.ProviderName(),
			})
		}
	}
	return dnsConfig
}

// DeleteCertBroker delete the Cert-Broker deployment if cert-management in disabled.
func (b *Botanist) DeleteCertBroker() error {
	if err := b.K8sSeedClient.DeleteDeployment(b.Shoot.SeedNamespace, common.CertBrokerResourceName); err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	if err := b.K8sSeedClient.DeleteSecret(b.Shoot.SeedNamespace, common.CertBrokerResourceName); err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	if err := b.K8sSeedClient.DeleteServiceAccount(b.Shoot.SeedNamespace, common.CertBrokerResourceName); err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	if err := b.K8sSeedClient.DeleteRoleBinding(b.Shoot.SeedNamespace, common.CertBrokerResourceName); err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	if err := b.K8sSeedClient.DeleteClusterRole(common.CertBrokerResourceName); err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	return nil
}
