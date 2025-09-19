// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package opentelemetrycollector

import (
	"k8s.io/apimachinery/pkg/runtime"
	clientcmdlatest "k8s.io/client-go/tools/clientcmd/api/latest"
	clientcmdv1 "k8s.io/client-go/tools/clientcmd/api/v1"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener/imagevector"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

const (
	// UnitName is the name of the opentelemetry-collector service.
	UnitName = v1beta1constants.OperatingSystemConfigUnitNameOpenTelemetryCollector

	// PathDirectory is the path for the opentelemetry-collector's directory.
	PathDirectory = "/var/lib/opentelemetry-collector"
	// PathAuthToken is the path for the file containing opentelemetry-collector's authentication that gets
	// validated by the kube-rbac-proxy.
	PathAuthToken = PathDirectory + "/auth-token"
	// PathConfig is the path for the opentelemetry-collector's configuration file.
	PathConfig = v1beta1constants.OperatingSystemConfigFilePathOpenTelemetryCollector
	// PathCACert is the path for the otelCollector-tls certificate authority.
	PathCACert = PathDirectory + "/ca.crt"

	openTelemetryCollectorBinaryPath     = v1beta1constants.OperatingSystemConfigFilePathBinaries + "/opentelemetry-collector"
	openTelemetryCollectorKubeconfigPath = PathDirectory + "/kubeconfig"
)

type component struct{}

// New returns a new opentelemetry-collector component.
func New() *component {
	return &component{}
}

// Name returns the name of the component.
func (component) Name() string {
	return "opentelemetry-collector"
}

// Config returns the units and files for the opentelemetry-collector component.
func (component) Config(ctx components.Context) ([]extensionsv1alpha1.Unit, []extensionsv1alpha1.File, error) {
	var (
		units []extensionsv1alpha1.Unit
		files []extensionsv1alpha1.File
	)

	if ctx.OpenTelemetryCollectorLogShipperEnabled {
		collectorConfigFile, err := getOpentelemetryCollectorConfigurationFile(ctx)
		if err != nil {
			return nil, nil, err
		}

		authInfo := clientcmdv1.AuthInfo{TokenFile: PathAuthToken}
		cluster := clientcmdv1.Cluster{Server: ctx.APIServerURL, CertificateAuthority: PathCACert}
		kubeconfig := kubernetesutils.NewKubeconfig("shoot", cluster, authInfo)

		raw, err := runtime.Encode(clientcmdlatest.Codec, kubeconfig)
		if err != nil {
			return nil, nil, err
		}

		units = append(units, getOpenTelemetryCollectorUnit())
		files = append(files, collectorConfigFile, getOpenTelemetryCollectorCAFile(ctx), extensionsv1alpha1.File{
			Path:        openTelemetryCollectorBinaryPath,
			Permissions: ptr.To[uint32](0700),
			Content: extensionsv1alpha1.FileContent{
				ImageRef: &extensionsv1alpha1.FileContentImageRef{
					Image:           ctx.Images[imagevector.ContainerImageNameOpentelemetryCollector].String(),
					FilePathInImage: "/otelcol-contrib",
				},
			},
		}, extensionsv1alpha1.File{
			Path:        openTelemetryCollectorKubeconfigPath,
			Permissions: ptr.To[uint32](0600),
			Content: extensionsv1alpha1.FileContent{
				Inline: &extensionsv1alpha1.FileContentInline{
					// Plain text
					Encoding: "",
					Data:     string(raw),
				},
			},
		})
	}

	return units, files, nil
}
