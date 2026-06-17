// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package opentelemetrycollector

import (
	"k8s.io/apimachinery/pkg/runtime"
	clientcmdlatest "k8s.io/client-go/tools/clientcmd/api/latest"
	clientcmdv1 "k8s.io/client-go/tools/clientcmd/api/v1"

	"github.com/gardener/gardener/imagevector"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

const (
	// UnitName is the name of the opentelemetry-collector service.
	UnitName = "opentelemetry-collector.service"

	// UnitNameHealthCheck is the name of the opentelemetry-collector healthcheck oneshot.
	UnitNameHealthCheck = "opentelemetry-collector-healthcheck.service"

	// UnitNameHealthCheckTimer is the name of the opentelemetry-collector timer.
	UnitNameHealthCheckTimer = "opentelemetry-collector-healthcheck.timer"

	// UnitNameAuthToken is the opentelemetry-collector checker that ensures the auth-token file exists before startup.
	UnitNameAuthToken = "opentelemetry-collector-auth-token.path"

	// PathDirectory is the path for the opentelemetry-collector's directory.
	PathDirectory = "/var/lib/opentelemetry-collector"
	// PathAuthToken is the path for the file containing opentelemetry-collector's authentication that gets
	// validated by the kube-rbac-proxy.
	PathAuthToken = PathDirectory + "/auth-token"
	// PathConfig is the path for the opentelemetry-collector's configuration file.
	PathConfig = "/var/lib/opentelemetry-collector/config/config"
	// PathCACert is the path for the otelCollector-tls certificate authority.
	PathCACert = PathDirectory + "/ca.crt"
	// MetricsPort is the port on which the OpenTelemetry collector exposes
	// its internal metrics.
	MetricsPort = 18888

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

	if !ctx.OpenTelemetryCollectorLogShipperEnabled {
		return units, files, nil
	}

	collectorConfigFile, err := getOpentelemetryCollectorConfigurationFile(ctx)
	if err != nil {
		return nil, nil, err
	}

	var (
		authInfo   = clientcmdv1.AuthInfo{TokenFile: PathAuthToken}
		cluster    = clientcmdv1.Cluster{Server: ctx.APIServerURL, CertificateAuthority: PathCACert}
		kubeconfig = kubernetesutils.NewKubeconfig("shoot", cluster, authInfo)
	)

	raw, err := runtime.Encode(clientcmdlatest.Codec, kubeconfig)
	if err != nil {
		return nil, nil, err
	}

	// File dependencies for the opentelemetry-collector unit are expressed in two
	// complementary ways, and it is worth being explicit about why:
	//   - The `FilePaths` field on extensionsv1alpha1.Unit lists files that are
	//     managed as part of the OperatingSystemConfig (config, CA cert, binary,
	//     kubeconfig). The gardener-node-agent restarts the unit when any of
	//     these managed files change.
	//   - A systemd.path(5) unit (see getOpenTelemetryCollectorPathAuthTokenUnit)
	//     waits for the auth-token file to appear on disk. The auth-token is not
	//     part of the OSC `Files` list because it is provisioned at runtime by
	//     another component, so `FilePaths` cannot express this dependency.
	// A future refactor that unifies how resource dependencies are declared
	// would make the overall picture easier to follow.
	//
	// As a consequence, the opentelemetry-collector service can be (re)started
	// through four different mechanisms, which can be confusing while debugging:
	//   - the path unit, when the auth-token file first appears,
	//   - the healthcheck timer unit, which restarts the service on a failed probe,
	//   - the gardener-node-agent, when any file listed in `FilePaths` changes,
	//   - `Restart=always` in the service unit itself, on process exit.
	// When investigating an unexpected restart, check all four sources.
	units = append(
		units,
		getOpenTelemetryCollectorUnit(),
		getOpenTelemetryCollectorPathAuthTokenUnit(),
		getOpenTelemetryCollectorHealthCheckUnit(),
		getOpenTelemetryCollectorTimerUnit(),
	)
	files = append(files, collectorConfigFile, getOpenTelemetryCollectorCAFile(ctx), extensionsv1alpha1.File{
		Path:        openTelemetryCollectorBinaryPath,
		Permissions: new(uint32(0700)),
		Content: extensionsv1alpha1.FileContent{
			ImageRef: &extensionsv1alpha1.FileContentImageRef{
				Image:           ctx.Images[imagevector.ContainerImageNameOpentelemetryCollector].String(),
				FilePathInImage: "/bin/otelcol",
			},
		},
	}, extensionsv1alpha1.File{
		Path:        openTelemetryCollectorKubeconfigPath,
		Permissions: new(uint32(0600)),
		Content: extensionsv1alpha1.FileContent{
			Inline: &extensionsv1alpha1.FileContentInline{
				// Plain text
				Encoding: "",
				Data:     string(raw),
			},
		},
	})

	return units, files, nil
}
