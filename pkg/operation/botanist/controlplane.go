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
	"encoding/json"
	"fmt"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"

	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	gardencorev1alpha1helper "github.com/gardener/gardener/pkg/apis/core/v1alpha1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	controllermanagerfeatures "github.com/gardener/gardener/pkg/controllermanager/features"
	"github.com/gardener/gardener/pkg/features"
	"github.com/gardener/gardener/pkg/migration"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/utils"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/secrets"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var chartPathControlPlane = filepath.Join(common.ChartPath, "seed-controlplane", "charts")

// DeployNamespace creates a namespace in the Seed cluster which is used to deploy all the control plane
// components for the Shoot cluster. Moreover, the cloud provider configuration and all the secrets will be
// stored as ConfigMaps/Secrets.
func (b *Botanist) DeployNamespace(ctx context.Context) error {
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: b.Shoot.SeedNamespace,
		},
	}

	if err := kutil.CreateOrUpdate(ctx, b.K8sSeedClient.Client(), namespace, func() error {
		namespace.Annotations = getShootAnnotations(b.Shoot.Info.Annotations, b.Shoot.Info.Status.UID)
		namespace.Labels = map[string]string{
			common.GardenRole:                 common.GardenRoleShoot,
			common.GardenerRole:               common.GardenRoleShoot,
			common.ShootHibernated:            strconv.FormatBool(b.Shoot.IsHibernated),
			gardencorev1alpha1.BackupProvider: string(b.Seed.CloudProvider),
			gardencorev1alpha1.SeedProvider:   string(b.Seed.CloudProvider),
			gardencorev1alpha1.ShootProvider:  string(b.Shoot.CloudProvider),
		}
		return nil
	}); err != nil {
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
func (b *Botanist) DeployBackupNamespace(ctx context.Context) error {
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: common.GenerateBackupNamespaceName(b.BackupInfrastructure.Name),
		},
	}

	return kutil.CreateOrUpdate(ctx, b.K8sSeedClient.Client(), namespace, func() error {
		namespace.Labels = map[string]string{
			common.GardenRole: common.GardenRoleBackup,
		}
		return nil
	})
}

// DeleteNamespace deletes the namespace in the Seed cluster which holds the control plane components. The built-in
// garbage collection in Kubernetes will automatically delete all resources which belong to this namespace. This
// comprises volumes and load balancers as well.
func (b *Botanist) DeleteNamespace(ctx context.Context) error {
	return b.deleteNamespace(ctx, b.Shoot.SeedNamespace)
}

// DeleteBackupNamespace deletes the namespace in the Seed cluster which holds the backup infrastructure state. The built-in
// garbage collection in Kubernetes will automatically delete all resources which belong to this namespace.
func (b *Botanist) DeleteBackupNamespace(ctx context.Context) error {
	return b.deleteNamespace(ctx, common.GenerateBackupNamespaceName(b.BackupInfrastructure.Name))
}

func (b *Botanist) deleteNamespace(ctx context.Context, name string) error {
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
	err := b.K8sSeedClient.Client().Delete(ctx, namespace, kubernetes.DefaultDeleteOptionFuncs...)
	if apierrors.IsNotFound(err) || apierrors.IsConflict(err) {
		return nil
	}
	return err
}

// DeleteDeprecatedCloudMetadataServiceNetworkPolicy deletes old DEPRECATED Shoot network policy that allows access to the meta-data service only from
// the cloud-controller-manager and the kube-controller-manager
// DEPRECATED.
// TODO: Remove this after several releases.
func (b *Botanist) DeleteDeprecatedCloudMetadataServiceNetworkPolicy(ctx context.Context) error {
	return b.ChartApplierSeed.DeleteChart(ctx, filepath.Join(chartPathControlPlane, "deprecated-network-policies"), b.Shoot.SeedNamespace, "deprecated-network-policies")
}

// DeleteKubeAPIServer deletes the kube-apiserver deployment in the Seed cluster which holds the Shoot's control plane.
func (b *Botanist) DeleteKubeAPIServer(ctx context.Context) error {
	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      common.KubeAPIServerDeploymentName,
			Namespace: b.Shoot.SeedNamespace,
		},
	}
	return client.IgnoreNotFound(b.K8sSeedClient.Client().Delete(ctx, deploy, kubernetes.DefaultDeleteOptionFuncs...))
}

// DeployBackupInfrastructure creates a BackupInfrastructure resource into the project namespace of shoot on garden cluster.
// BackupInfrastructure controller acting on resource will actually create required cloud resources and updates the status.
func (b *Botanist) DeployBackupInfrastructure(ctx context.Context) error {
	var (
		name = common.GenerateBackupInfrastructureName(b.Shoot.SeedNamespace, b.Shoot.Info.Status.UID)

		backupInfrastructure = &gardenv1beta1.BackupInfrastructure{}
	)

	if err := b.K8sGardenClient.Client().Get(ctx, kutil.Key(b.Shoot.Info.Namespace, name), backupInfrastructure); client.IgnoreNotFound(err) != nil {
		return err
	}

	return b.ApplyChartGarden(filepath.Join(common.ChartPath, "garden-project", "charts", "backup-infrastructure"), b.Shoot.Info.Namespace, "backup-infrastructure", nil, map[string]interface{}{
		"backupInfrastructure": map[string]interface{}{
			"name":        name,
			"annotations": backupInfrastructure.Annotations,
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
	return client.IgnoreNotFound(err)
}

// DeleteKubeAddonManager deletes the kube-addon-manager deployment in the Seed cluster which holds the Shoot's control plane.
// +deprecated
// Can be removed in a future version.
func (b *Botanist) DeleteKubeAddonManager(ctx context.Context) error {
	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kube-addon-manager",
			Namespace: b.Shoot.SeedNamespace,
		},
	}
	if err := b.K8sSeedClient.Client().Delete(ctx, deploy, kubernetes.DefaultDeleteOptionFuncs...); client.IgnoreNotFound(err) != nil {
		return err
	}

	for _, name := range []string{
		"kube-addon-manager",
		"kube-addon-manager-cloud-config",
		"kube-addon-manager-core-addons",
		"kube-addon-manager-optional-addons",
		"kube-addon-manager-storageclasses",
	} {
		if err := b.K8sSeedClient.DeleteSecret(b.Shoot.SeedNamespace, name); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	}
	return nil
}

// Supported CA flags
const (
	scaleDownUtilizationThresholdFlag = "scale-down-utilization-threshold"
	scaleDownUnneededTimeFlag         = "scale-down-unneeded-time"
	scaleDownDelayAfterAddFlag        = "scale-down-delay-after-add"
	scaleDownDelayAfterFailureFlag    = "scale-down-delay-after-failure"
	scaleDownDelayAfterDeleteFlag     = "scale-down-delay-after-delete"
	scanIntervalFlag                  = "scan-interval"
)

var (
	scaleDownUnneededTimeDefault  = metav1.Duration{Duration: 30 * time.Minute}
	scaleDownDelayAfterAddDefault = metav1.Duration{Duration: 60 * time.Minute}
)

// DeployClusterAutoscaler deploys the cluster-autoscaler into the Shoot namespace in the Seed cluster. It is responsible
// for automatically scaling the worker pools of the Shoot.
func (b *Botanist) DeployClusterAutoscaler(ctx context.Context) error {
	if !b.Shoot.WantsClusterAutoscaler {
		return b.DeleteClusterAutoscaler(ctx)
	}

	var workerPools []map[string]interface{}
	for _, worker := range b.Shoot.MachineDeployments {
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

	var flags []map[string]interface{}
	if clusterAutoscalerConfig := b.Shoot.Info.Spec.Kubernetes.ClusterAutoscaler; clusterAutoscalerConfig != nil {
		flags = appendOrDefaultFlag(flags, scaleDownUtilizationThresholdFlag, clusterAutoscalerConfig.ScaleDownUtilizationThreshold, nil)
		flags = appendOrDefaultFlag(flags, scaleDownUnneededTimeFlag, clusterAutoscalerConfig.ScaleDownUnneededTime, scaleDownUnneededTimeDefault)
		flags = appendOrDefaultFlag(flags, scaleDownDelayAfterAddFlag, clusterAutoscalerConfig.ScaleDownDelayAfterAdd, scaleDownDelayAfterAddDefault)
		flags = appendOrDefaultFlag(flags, scaleDownDelayAfterFailureFlag, clusterAutoscalerConfig.ScaleDownDelayAfterFailure, nil)
		flags = appendOrDefaultFlag(flags, scaleDownDelayAfterDeleteFlag, clusterAutoscalerConfig.ScaleDownDelayAfterDelete, nil)
		flags = appendOrDefaultFlag(flags, scanIntervalFlag, clusterAutoscalerConfig.ScanInterval, nil)
	} else { // add defaults in case no ClusterAutoscaler section was found in the manifest
		flags = appendOrDefaultFlag(flags, scaleDownUnneededTimeFlag, nil, scaleDownUnneededTimeDefault)
		flags = appendOrDefaultFlag(flags, scaleDownDelayAfterAddFlag, nil, scaleDownDelayAfterAddDefault)
	}

	defaultValues := map[string]interface{}{
		"podAnnotations": map[string]interface{}{
			"checksum/secret-cluster-autoscaler": b.CheckSums[gardencorev1alpha1.DeploymentNameClusterAutoscaler],
		},
		"namespace": map[string]interface{}{
			"uid": b.SeedNamespaceObject.UID,
		},
		"replicas":    b.Shoot.GetReplicas(1),
		"workerPools": workerPools,
		"flags":       flags,
	}

	values, err := b.InjectSeedShootImages(defaultValues, common.ClusterAutoscalerImageName)
	if err != nil {
		return err
	}

	return b.ApplyChartSeed(filepath.Join(chartPathControlPlane, gardencorev1alpha1.DeploymentNameClusterAutoscaler), b.Shoot.SeedNamespace, gardencorev1alpha1.DeploymentNameClusterAutoscaler, nil, values)
}

func mkFlag(name string, value interface{}) map[string]interface{} {
	return map[string]interface{}{
		"name":  name,
		"value": marshalFlagValue(value),
	}
}

func marshalFlagValue(value interface{}) interface{} {
	switch v := value.(type) {
	case metav1.Duration:
		return v.Duration.String()
	default:
		rv := reflect.ValueOf(value)
		if rv.Type().Kind() == reflect.Ptr {
			return marshalFlagValue(rv.Elem().Interface())
		}
		return v
	}
}

func appendOrDefaultFlag(flags []map[string]interface{}, flagName string, value, defaultValue interface{}) []map[string]interface{} {
	if value == nil {
		if defaultValue == nil {
			return flags
		}
		value = defaultValue
	}
	return append(flags, mkFlag(flagName, value))
}

// DeleteClusterAutoscaler deletes the cluster-autoscaler deployment in the Seed cluster which holds the Shoot's control plane.
func (b *Botanist) DeleteClusterAutoscaler(ctx context.Context) error {
	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      gardencorev1alpha1.DeploymentNameClusterAutoscaler,
			Namespace: b.Shoot.SeedNamespace,
		},
	}
	return client.IgnoreNotFound(b.K8sSeedClient.Client().Delete(ctx, deploy, kubernetes.DefaultDeleteOptionFuncs...))
}

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
					"alertmanager": map[string]interface{}{
						"enabled": b.Shoot.WantsAlertmanager,
					},
					"elasticsearch": map[string]interface{}{
						"enabled": controllermanagerfeatures.FeatureGate.Enabled(features.Logging),
					},
				},
			},
			"shoot": map[string]interface{}{
				"apiserver": fmt.Sprintf("https://%s", b.Shoot.InternalClusterDomain),
				"provider":  b.Shoot.CloudProvider,
			},
			"vpa": map[string]interface{}{
				"enabled": controllermanagerfeatures.FeatureGate.Enabled(features.VPA),
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

	// TODO: Cleanup logic. Remove once all old grafana artifacts have been removed from all landscapes
	if err := common.DeleteOldGrafanaStack(b.K8sSeedClient, b.Shoot.SeedNamespace); err != nil {
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
			Name:      common.PrometheusStatefulSetName,
			Namespace: b.Shoot.SeedNamespace,
		},
	}
	return client.IgnoreNotFound(b.K8sSeedClient.Client().Delete(ctx, prometheusStatefulSet, kubernetes.DefaultDeleteOptionFuncs...))
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
			Value: b.CheckSums[gardencorev1alpha1.SecretNameCloudProvider],
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
func (b *Botanist) DeploySeedLogging(ctx context.Context) error {
	if !controllermanagerfeatures.FeatureGate.Enabled(features.Logging) {
		return common.DeleteLoggingStack(ctx, b.K8sSeedClient.Client(), b.Shoot.SeedNamespace)
	}

	var (
		kibanaHost                             = b.Seed.GetIngressFQDN("k", b.Shoot.Info.Name, b.Garden.Project.Name)
		kibanaCredentials                      = b.Secrets["kibana-logging-sg-credentials"]
		kibanaUserIngressCredentialsSecretName = "logging-ingress-credentials-users"
		sgKibanaUsername                       = kibanaCredentials.Data[secrets.DataKeyUserName]
		sgKibanaPassword                       = kibanaCredentials.Data[secrets.DataKeyPassword]
		sgKibanaPasswordHash                   = kibanaCredentials.Data[secrets.DataKeyPasswordBcryptHash]
		basicAuth                              = utils.EncodeBase64([]byte(fmt.Sprintf("%s:%s", sgKibanaUsername, sgKibanaPassword)))

		sgCuratorPassword     = b.Secrets["curator-sg-credentials"].Data[secrets.DataKeyPassword]
		sgCuratorPasswordHash = b.Secrets["curator-sg-credentials"].Data[secrets.DataKeyPasswordBcryptHash]

		sgUserPasswordHash  = b.Secrets[kibanaUserIngressCredentialsSecretName].Data[secrets.DataKeyPasswordBcryptHash]
		sgAdminPasswordHash = b.Secrets[common.KibanaAdminIngressCredentialsSecretName].Data[secrets.DataKeyPasswordBcryptHash]

		userIngressBasicAuth  = utils.CreateSHA1Secret(b.Secrets[kibanaUserIngressCredentialsSecretName].Data[secrets.DataKeyUserName], b.Secrets[kibanaUserIngressCredentialsSecretName].Data[secrets.DataKeyPassword])
		adminIngressBasicAuth = utils.CreateSHA1Secret(b.Secrets[common.KibanaAdminIngressCredentialsSecretName].Data[secrets.DataKeyUserName], b.Secrets[common.KibanaAdminIngressCredentialsSecretName].Data[secrets.DataKeyPassword])
		ingressBasicAuth      string

		sgFluentdPasswordHash string
	)

	userIngressBasicAuthDecoded, err := utils.DecodeBase64(userIngressBasicAuth)
	if err != nil {
		return err
	}

	adminIngressBasicAuthDecoded, err := utils.DecodeBase64(adminIngressBasicAuth)
	if err != nil {
		return err
	}

	ingressBasicAuthDecoded := fmt.Sprintf("%s\n%s", string(userIngressBasicAuthDecoded), string(adminIngressBasicAuthDecoded))
	ingressBasicAuth = utils.EncodeBase64([]byte(ingressBasicAuthDecoded))

	images, err := b.InjectSeedSeedImages(map[string]interface{}{},
		common.ElasticsearchImageName,
		common.ElasticsearchMetricsExporterImageName,
		common.ElasticsearchSearchguardImageName,
		common.CuratorImageName,
		common.KibanaImageName,
		common.SearchguardImageName,
		common.AlpineImageName,
	)
	if err != nil {
		return err
	}

	ct := b.Shoot.Info.CreationTimestamp.Time

	sgFluentdSecret := &corev1.Secret{}
	if err = b.K8sSeedClient.Client().Get(ctx, kutil.Key(common.GardenNamespace, "fluentd-es-sg-credentials"), sgFluentdSecret); err != nil {
		return err
	}

	sgFluentdPasswordHash = string(sgFluentdSecret.Data["bcryptPasswordHash"])

	elasticKibanaCurator := map[string]interface{}{
		"ingress": map[string]interface{}{
			"host":            kibanaHost,
			"basicAuthSecret": ingressBasicAuth,
		},
		"elasticsearch": map[string]interface{}{
			"replicaCount": b.Shoot.GetReplicas(1),
			"readinessProbe": map[string]interface{}{
				"httpAuth": basicAuth,
			},
			"metricsExporter": map[string]interface{}{
				"username": string(sgKibanaUsername),
				"password": string(sgKibanaPassword),
			},
		},
		"kibana": map[string]interface{}{
			"replicaCount": b.Shoot.GetReplicas(1),
			"sgUsername":   "kibanaserver",
			"sgPassword":   string(sgKibanaPassword),
		},
		"curator": map[string]interface{}{
			"hourly": map[string]interface{}{
				"schedule": fmt.Sprintf("%d * * * *", ct.Minute()),
				"suspend":  b.Shoot.IsHibernated,
			},
			"daily": map[string]interface{}{
				"schedule": fmt.Sprintf("%d 0,6,12,18 * * *", ct.Minute()%54+5),
				"suspend":  b.Shoot.IsHibernated,
			},
			"sgUsername": "curator",
			"sgPassword": string(sgCuratorPassword),
		},
		"searchguard": map[string]interface{}{
			"enabled":      true,
			"replicaCount": b.Shoot.GetReplicas(1),
			"annotations": map[string]interface{}{
				"checksum/tls-secrets-server": b.CheckSums["elasticsearch-logging-server"],
				"checksum/sg-admin-client":    b.CheckSums["sg-admin-client"],
			},
			"users": map[string]interface{}{
				"fluentd": map[string]interface{}{
					"hash": string(sgFluentdPasswordHash),
				},
				"kibanaserver": map[string]interface{}{
					"hash": string(sgKibanaPasswordHash),
				},
				"curator": map[string]interface{}{
					"hash": string(sgCuratorPasswordHash),
				},
				"user": map[string]interface{}{
					"hash": string(sgUserPasswordHash),
				},
				"admin": map[string]interface{}{
					"hash": string(sgAdminPasswordHash),
				},
			},
		},
		"global": images,
	}

	return b.ApplyChartSeed(filepath.Join(common.ChartPath, "seed-bootstrap", "charts", "elastic-kibana-curator"), b.Shoot.SeedNamespace, fmt.Sprintf("%s-logging", b.Shoot.SeedNamespace), nil, elasticKibanaCurator)
}

// DeployDependencyWatchdog deploys the dependency watchdog to the Shoot namespace in the Seed.
func (b *Botanist) DeployDependencyWatchdog(ctx context.Context) error {
	dependencyWatchdogConfig := map[string]interface{}{
		"replicas": b.Shoot.GetReplicas(1),
	}

	dependencyWatchdog, err := b.InjectSeedSeedImages(dependencyWatchdogConfig, common.DependencyWatchdogDeploymentName)
	if err != nil {
		return nil
	}
	return b.ChartApplierSeed.ApplyChart(ctx, filepath.Join(chartPathControlPlane, common.DependencyWatchdogDeploymentName), b.Shoot.SeedNamespace, common.DependencyWatchdogDeploymentName, nil, dependencyWatchdog)
}

// WakeUpControlPlane scales the replicas to 1 for the following deployments which are needed in case of shoot deletion:
// * etcd-events
// * etcd-main
// * kube-apiserver
// * kube-controller-manager
func (b *Botanist) WakeUpControlPlane(ctx context.Context) error {
	client := b.K8sSeedClient.Client()

	for _, statefulset := range []string{common.EtcdEventsStatefulSetName, common.EtcdMainStatefulSetName} {
		if err := kubernetes.ScaleStatefulSet(ctx, client, kutil.Key(b.Shoot.SeedNamespace, statefulset), 1); err != nil {
			return err
		}
	}
	if err := b.WaitUntilEtcdReady(ctx); err != nil {
		return err
	}

	if err := kubernetes.ScaleDeployment(ctx, client, kutil.Key(b.Shoot.SeedNamespace, common.KubeAPIServerDeploymentName), 1); err != nil {
		return err
	}
	if err := b.WaitUntilKubeAPIServerReady(ctx); err != nil {
		return err
	}

	controllerManagerDeployments := []string{common.KubeControllerManagerDeploymentName}
	for _, deployment := range controllerManagerDeployments {
		if err := kubernetes.ScaleDeployment(ctx, client, kutil.Key(b.Shoot.SeedNamespace, deployment), 1); err != nil {
			return err
		}
	}

	return nil
}

// HibernateControlPlane hibernates the entire control plane if the shoot shall be hibernated.
func (b *Botanist) HibernateControlPlane(ctx context.Context) error {
	client := b.K8sSeedClient.Client()

	// If a shoot is hibernated we only want to scale down the entire control plane if no nodes exist anymore. The node-lifecycle-controller
	// inside KCM is responsible for deleting Node objects of terminated/non-existing VMs, so let's wait for that before scaling down.
	if b.K8sShootClient != nil {
		ctxWithTimeOut, cancel := context.WithTimeout(ctx, 10*time.Minute)
		defer cancel()

		if err := b.WaitUntilNodesDeleted(ctxWithTimeOut); err != nil {
			return err
		}
	}

	deployments := []string{
		common.GardenerResourceManagerDeploymentName,
		common.KubeControllerManagerDeploymentName,
		common.KubeAPIServerDeploymentName,
	}
	for _, deployment := range deployments {
		if err := kubernetes.ScaleDeployment(ctx, client, kutil.Key(b.Shoot.SeedNamespace, deployment), 0); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	}

	for _, statefulset := range []string{common.EtcdEventsStatefulSetName, common.EtcdMainStatefulSetName} {
		if err := kubernetes.ScaleStatefulSet(ctx, client, kutil.Key(b.Shoot.SeedNamespace, statefulset), 0); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	}

	return nil
}

// ControlPlaneDefaultTimeout is the default timeout and defines how long Gardener should wait
// for a successful reconciliation of a control plane resource.
const ControlPlaneDefaultTimeout = 10 * time.Minute

// DeployControlPlane creates the `ControlPlane` extension resource in the shoot namespace in the seed
// cluster. Gardener waits until an external controller did reconcile the cluster successfully.
func (b *Botanist) DeployControlPlane(ctx context.Context) error {
	var (
		cp = &extensionsv1alpha1.ControlPlane{
			ObjectMeta: metav1.ObjectMeta{
				Name:      b.Shoot.Info.Name,
				Namespace: b.Shoot.SeedNamespace,
			},
		}
	)

	// In the future the providerConfig will be blindly copied from the core.gardener.cloud/v1alpha1.Shoot
	// resource. However, until we have completely moved to this resource, we have to compute the needed
	// configuration ourselves from garden.sapcloud.io/v1beta1.Shoot.
	providerConfig, err := migration.ShootToControlPlaneConfig(b.Shoot.Info)
	if err != nil {
		return err
	}

	return kutil.CreateOrUpdate(ctx, b.K8sSeedClient.Client(), cp, func() error {
		cp.Spec = extensionsv1alpha1.ControlPlaneSpec{
			DefaultSpec: extensionsv1alpha1.DefaultSpec{
				Type: string(b.Shoot.CloudProvider),
			},
			Region: b.Shoot.Info.Spec.Cloud.Region,
			SecretRef: corev1.SecretReference{
				Name:      gardencorev1alpha1.SecretNameCloudProvider,
				Namespace: cp.Namespace,
			},
			ProviderConfig: &runtime.RawExtension{
				Object: providerConfig,
			},
			InfrastructureProviderStatus: &runtime.RawExtension{
				Raw: b.Shoot.InfrastructureStatus,
			},
		}
		return nil
	})
}

// DestroyControlPlane deletes the `ControlPlane` extension resource in the shoot namespace in the seed cluster,
// and it waits for a maximum of 10m until it is deleted.
func (b *Botanist) DestroyControlPlane(ctx context.Context) error {
	return client.IgnoreNotFound(b.K8sSeedClient.Client().Delete(ctx, &extensionsv1alpha1.ControlPlane{ObjectMeta: metav1.ObjectMeta{Namespace: b.Shoot.SeedNamespace, Name: b.Shoot.Info.Name}}))
}

// WaitUntilControlPlaneReady waits until the control plane resource has been reconciled successfully.
func (b *Botanist) WaitUntilControlPlaneReady(ctx context.Context) error {
	var (
		timedContext, cancel = context.WithTimeout(ctx, ControlPlaneDefaultTimeout)
		lastError            *gardencorev1alpha1.LastError
		cpStatus             []byte
	)

	defer cancel()

	if err := wait.PollUntil(5*time.Second, func() (bool, error) {
		cp := &extensionsv1alpha1.ControlPlane{}
		if err := b.K8sSeedClient.Client().Get(ctx, client.ObjectKey{Name: b.Shoot.Info.Name, Namespace: b.Shoot.SeedNamespace}, cp); err != nil {
			return false, err
		}

		if lastErr := cp.Status.LastError; lastErr != nil {
			b.Logger.Errorf("Control plane did not get ready yet, lastError is: %s", lastErr.Description)
			lastError = lastErr
		}

		if lastOperation := cp.Status.LastOperation; lastOperation != nil &&
			lastOperation.State == gardencorev1alpha1.LastOperationStateSucceeded &&
			cp.Status.ObservedGeneration == cp.Generation {

			if providerStatus := cp.Status.ProviderStatus; providerStatus != nil {
				cpStatus = providerStatus.Raw
			}
			return true, nil
		}

		b.Logger.Infof("Waiting for control plane to be ready...")
		return false, nil
	}, timedContext.Done()); err != nil {
		message := fmt.Sprintf("Failed to create control plane")
		if lastError != nil {
			return gardencorev1alpha1helper.DetermineError(fmt.Sprintf("%s: %s", message, lastError.Description))
		}
		return gardencorev1alpha1helper.DetermineError(fmt.Sprintf("%s: %s", message, err.Error()))
	}

	b.Shoot.ControlPlaneStatus = cpStatus
	return nil
}

// WaitUntilControlPlaneDeleted waits until the control plane resource has been deleted.
func (b *Botanist) WaitUntilControlPlaneDeleted(ctx context.Context) error {
	var (
		timedContext, cancel = context.WithTimeout(ctx, ControlPlaneDefaultTimeout)
		lastError            *gardencorev1alpha1.LastError
	)

	defer cancel()

	if err := wait.PollUntil(5*time.Second, func() (bool, error) {
		cp := &extensionsv1alpha1.ControlPlane{}
		if err := b.K8sSeedClient.Client().Get(ctx, client.ObjectKey{Name: b.Shoot.Info.Name, Namespace: b.Shoot.SeedNamespace}, cp); err != nil {
			if apierrors.IsNotFound(err) {
				return true, nil
			}
			return false, err
		}

		if lastErr := cp.Status.LastError; lastErr != nil {
			b.Logger.Errorf("Control plane did not get deleted yet, lastError is: %s", lastErr.Description)
			lastError = lastErr
		}

		b.Logger.Infof("Waiting for control plane to be deleted...")
		return false, nil
	}, timedContext.Done()); err != nil {
		message := fmt.Sprintf("Failed to delete control plane")
		if lastError != nil {
			return gardencorev1alpha1helper.DetermineError(fmt.Sprintf("%s: %s", message, lastError.Description))
		}
		return gardencorev1alpha1helper.DetermineError(fmt.Sprintf("%s: %s", message, err.Error()))
	}

	return nil
}
