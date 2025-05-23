// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controlplane

import (
	"context"
	"time"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/controller/controlplane/genericactuator"
	extensionssecretsmanager "github.com/gardener/gardener/extensions/pkg/util/secret/manager"
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
