// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controlplane

import (
	"context"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/controller/controlplane/genericactuator"
	extensionssecretsmanager "github.com/gardener/gardener/extensions/pkg/util/secret/manager"
	v1beta1helper "github.com/gardener/gardener/pkg/api/core/v1beta1/helper"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/provider-local/apis/local/helper"
	"github.com/gardener/gardener/pkg/provider-local/charts"
	localimagevector "github.com/gardener/gardener/pkg/provider-local/imagevector"
	"github.com/gardener/gardener/pkg/provider-local/local"
	"github.com/gardener/gardener/pkg/utils/chart"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

const (
	caNameControlPlane               = "ca-" + local.Name + "-controlplane"
	cloudControllerManagerServerName = "cloud-controller-manager-server"
)

// NewValuesProvider creates a new ValuesProvider for the generic actuator.
func NewValuesProvider() genericactuator.ValuesProvider {
	return &valuesProvider{}
}

type valuesProvider struct {
	genericactuator.NoopValuesProvider
}

func getSecretConfigs(namespace string) []extensionssecretsmanager.SecretConfigWithOptions {
	return []extensionssecretsmanager.SecretConfigWithOptions{
		{
			Config: &secretsutils.CertificateSecretConfig{
				Name:       caNameControlPlane,
				CommonName: caNameControlPlane,
				CertType:   secretsutils.CACert,
			},
			Options: []secretsmanager.GenerateOption{secretsmanager.Persist()},
		},
		{
			Config: &secretsutils.CertificateSecretConfig{
				Name:                        cloudControllerManagerServerName,
				CommonName:                  local.CloudControllerManagerName,
				DNSNames:                    kubernetesutils.DNSNamesForService(local.CloudControllerManagerName, namespace),
				CertType:                    secretsutils.ServerCert,
				SkipPublishingCACertificate: true,
			},
			Options: []secretsmanager.GenerateOption{secretsmanager.SignedByCA(caNameControlPlane)},
		},
		{
			Config: &secretsutils.CertificateSecretConfig{
				Name:       local.Name + "-dummy-server",
				CommonName: local.Name + "-dummy-server",
				DNSNames:   kubernetesutils.DNSNamesForService(local.Name+"-dummy-server", namespace),
				CertType:   secretsutils.ServerCert,
			},
			Options: []secretsmanager.GenerateOption{secretsmanager.SignedByCA(caNameControlPlane)},
		},
		{
			Config: &secretsutils.CertificateSecretConfig{
				Name:                        local.Name + "-dummy-client",
				CommonName:                  "extensions.gardener.cloud:" + local.Name + ":dummy-client",
				Organization:                []string{"extensions.gardener.cloud:" + local.Name + ":dummy"},
				CertType:                    secretsutils.ClientCert,
				SkipPublishingCACertificate: true,
			},
			Options: []secretsmanager.GenerateOption{secretsmanager.SignedByCA(caNameControlPlane)},
		},
		{
			Config: &secretsutils.BasicAuthSecretConfig{
				Name:           local.Name + "-dummy-auth",
				Format:         secretsutils.BasicAuthFormatNormal,
				PasswordLength: 32,
			},
			Options: []secretsmanager.GenerateOption{secretsmanager.Validity(time.Hour)},
		},
	}
}

func shootAccessSecrets(namespace string) []*gardenerutils.AccessSecret {
	return []*gardenerutils.AccessSecret{
		gardenerutils.NewShootAccessSecret(local.CloudControllerManagerName, namespace),
	}
}

var (
	configChart = &chart.Chart{
		Name:       "cloud-provider-config",
		EmbeddedFS: charts.ChartConfig,
		Path:       charts.ChartPathConfig,
		Objects: []*chart.Object{
			{Type: &corev1.ConfigMap{}, Name: local.CloudProviderConfigName},
		},
	}

	controlPlaneChart = &chart.Chart{
		Name:       "seed-controlplane",
		EmbeddedFS: charts.ChartControlPlane,
		Path:       charts.ChartPathControlPlane,
		SubCharts: []*chart.Chart{
			{
				Name: local.CloudControllerManagerName,
				Images: []string{
					localimagevector.ImageNameCloudControllerManagerLocal,
				},
				Objects: []*chart.Object{
					{Type: &appsv1.Deployment{}, Name: local.CloudControllerManagerName},
					{Type: &corev1.ServiceAccount{}, Name: local.CloudControllerManagerName},
					{Type: &rbacv1.Role{}, Name: local.CloudControllerManagerName},
					{Type: &rbacv1.RoleBinding{}, Name: local.CloudControllerManagerName},
				},
			},
		},
	}

	controlPlaneShootChart = &chart.Chart{
		Name:       "shoot-system-components",
		EmbeddedFS: charts.ChartShootSystemComponents,
		Path:       charts.ChartPathShootSystemComponents,
		SubCharts: []*chart.Chart{
			{
				Name: "local-path-provisioner",
				Images: []string{
					localimagevector.ImageNameLocalPathProvisioner,
					localimagevector.ImageNameLocalPathHelper,
				},
			},
		},
	}

	storageClassChart = &chart.Chart{
		Name:       "shoot-storageclasses",
		EmbeddedFS: charts.ChartShootStorageClasses,
		Path:       charts.ChartPathShootStorageClasses,
	}
)

// GetConfigChartValues returns the values for the config chart applied by the generic actuator.
func (vp *valuesProvider) GetConfigChartValues(
	_ context.Context,
	_ *extensionsv1alpha1.ControlPlane,
	cluster *extensionscontroller.Cluster,
) (map[string]interface{}, error) {
	config, err := helper.CloudProfileConfigFromCluster(cluster)
	if err != nil {
		return nil, err
	}

	// Use cloud-controller-manager's in-cluster credentials by default. Use the kind kubeconfig from the cloudprovider
	// secret for self-hosted shoots with managed infrastructure.
	runtimeKubeconfig := ""
	if v1beta1helper.IsShootSelfHosted(cluster.Shoot.Spec.Provider.Workers) && v1beta1helper.HasManagedInfrastructure(cluster.Shoot) {
		runtimeKubeconfig = "/var/run/secrets/gardener.cloud/cloudprovider/kubeconfig"
	}

	return map[string]any{
		"runtimeCluster": map[string]any{
			"namespace":  cluster.Shoot.Status.TechnicalID,
			"kubeconfig": runtimeKubeconfig,
		},
		"loadBalancer": map[string]any{
			"image": config.LoadBalancer.Image,
		},
	}, nil
}

// GetControlPlaneChartValues returns the values for the control plane chart applied by the generic actuator.
func (vp *valuesProvider) GetControlPlaneChartValues(
	_ context.Context,
	_ *extensionsv1alpha1.ControlPlane,
	cluster *extensionscontroller.Cluster,
	secretsReader secretsmanager.Reader,
	checksums map[string]string,
	scaledDown bool,
) (map[string]any, error) {
	serverSecret, found := secretsReader.Get(cloudControllerManagerServerName)
	if !found {
		return nil, fmt.Errorf("secret %q not found", cloudControllerManagerServerName)
	}

	return map[string]any{
		"global": map[string]interface{}{
			"genericTokenKubeconfigSecretName": extensionscontroller.GenericTokenKubeconfigSecretNameFromCluster(cluster),
		},
		local.CloudControllerManagerName: map[string]any{
			"replicas":    extensionscontroller.GetControlPlaneReplicas(cluster, scaledDown, 1),
			"clusterName": cluster.Shoot.Status.TechnicalID,
			"podAnnotations": map[string]interface{}{
				"checksum/configmap-" + local.CloudProviderConfigName: checksums[local.CloudProviderConfigName],
			},
			"podLabels": map[string]interface{}{
				v1beta1constants.LabelPodMaintenanceRestart: "true",
			},
			"priorityClassName": v1beta1constants.PriorityClassNameShootControlPlane300,
			"server": map[string]interface{}{
				"tlsSecret":       serverSecret.Name,
				"tlsCipherSuites": kubernetesutils.TLSCipherSuites,
			},
		},
	}, nil
}

// GetControlPlaneShootChartValues returns the values for the control plane shoot chart applied by the generic actuator.
func (vp *valuesProvider) GetControlPlaneShootChartValues(
	_ context.Context,
	_ *extensionsv1alpha1.ControlPlane,
	_ *extensionscontroller.Cluster,
	_ secretsmanager.Reader,
	_ map[string]string,
) (map[string]any, error) {
	return map[string]any{}, nil
}
