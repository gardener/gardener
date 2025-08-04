// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controlplane

import (
	"context"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/controller/controlplane/genericactuator"
	extensionssecretsmanager "github.com/gardener/gardener/extensions/pkg/util/secret/manager"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/provider-local/charts"
	localimagevector "github.com/gardener/gardener/pkg/provider-local/imagevector"
	"github.com/gardener/gardener/pkg/provider-local/local"
	"github.com/gardener/gardener/pkg/utils/chart"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

const caNameControlPlane = "ca-" + local.Name + "-controlplane"

// NewValuesProvider creates a new ValuesProvider for the generic actuator.
func NewValuesProvider() genericactuator.ValuesProvider {
	return &valuesProvider{}
}

type valuesProvider struct {
	genericactuator.NoopValuesProvider
}

func getSecretConfigs(cp *extensionsv1alpha1.ControlPlane, cluster *extensionscontroller.Cluster) []extensionssecretsmanager.SecretConfigWithOptions {
	if v1beta1helper.IsShootAutonomous(cluster.Shoot) && cp.Namespace != metav1.NamespaceSystem {
		// When bootstrapping the autonomous shoot cluster (`gardenadm bootstrap`), we don't need additional secrets.
		return nil
	}

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
				Name:       local.Name + "-dummy-server",
				CommonName: local.Name + "-dummy-server",
				DNSNames:   kubernetesutils.DNSNamesForService(local.Name+"-dummy-server", cp.Namespace),
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

var (
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
