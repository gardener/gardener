// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package kubelet

import (
	_ "embed"
	"strconv"
	"strings"

	"github.com/Masterminds/semver/v3"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener/imagevector"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	extensionsv1alpha1helper "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1/helper"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components/containerd"
	oscutils "github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/utils"
	"github.com/gardener/gardener/pkg/utils"
)

const (
	// UnitName is the name of the kubelet service.
	UnitName = v1beta1constants.OperatingSystemConfigUnitNameKubeletService

	// PathKubeconfigBootstrap is the path for the kubelet's bootstrap kubeconfig.
	PathKubeconfigBootstrap = PathKubeletDirectory + "/kubeconfig-bootstrap"
	// PathKubeconfigReal is the path for the kubelet's real kubeconfig (with client certificate after the TLS
	// bootstrapping process finished).
	PathKubeconfigReal = PathKubeletDirectory + "/kubeconfig-real"
	// PathKubeletCACert is the path for the kubelet's certificate authority.
	PathKubeletCACert = PathKubeletDirectory + "/ca.crt"
	// PathKubeletConfig is the path for the kubelet's config file.
	PathKubeletConfig = v1beta1constants.OperatingSystemConfigFilePathKubeletConfig
	// PathKubeletDirectory is the path for the kubelet's directory.
	PathKubeletDirectory = "/var/lib/kubelet"

	// http2ReadIdleTimeout is the timeout after which kubelet will send a ping frame
	// frame if no frame is received on the connection.
	http2ReadIdleTimeSeconds = 20
	// http2PingTimeSeconds is the timeout after which the connection will be closed
	// if a response to Ping is not received.
	http2PingTimeSeconds = 10

	pathVolumePluginDirectory = "/var/lib/kubelet/volumeplugins"
)

type component struct{}

// New returns a new kubelet component.
func New() *component {
	return &component{}
}

func (component) Name() string {
	return "kubelet"
}

func (component) Config(ctx components.Context) ([]extensionsv1alpha1.Unit, []extensionsv1alpha1.File, error) {
	fileContentKubeletConfig, err := getFileContentKubeletConfig(ctx.KubernetesVersion, ctx.ClusterDNSAddresses, ctx.ClusterDomain, ctx.Taints, ctx.KubeletConfigParameters)
	if err != nil {
		return nil, nil, err
	}

	kubeletFiles := []extensionsv1alpha1.File{
		{
			Path:        PathKubeletCACert,
			Permissions: ptr.To[int32](0644),
			Content: extensionsv1alpha1.FileContent{
				Inline: &extensionsv1alpha1.FileContentInline{
					Encoding: "b64",
					Data:     utils.EncodeBase64(ctx.KubeletCABundle),
				},
			},
		},
		{
			Path:        PathKubeletConfig,
			Permissions: ptr.To[int32](0644),
			Content: extensionsv1alpha1.FileContent{
				Inline: fileContentKubeletConfig,
			},
		},
		{
			Path:        v1beta1constants.OperatingSystemConfigFilePathBinaries + "/kubelet",
			Permissions: ptr.To[int32](0755),
			Content: extensionsv1alpha1.FileContent{
				ImageRef: &extensionsv1alpha1.FileContentImageRef{
					Image:           ctx.Images[imagevector.ImageNameHyperkube].String(),
					FilePathInImage: "/kubelet",
				},
			},
		},
	}

	cliFlags := CLIFlags(ctx.KubernetesVersion, ctx.NodeLabels, ctx.CRIName, ctx.KubeletCLIFlags, ctx.PreferIPv6)

	kubeletUnit := extensionsv1alpha1.Unit{
		Name:    UnitName,
		Command: ptr.To(extensionsv1alpha1.CommandStart),
		Enable:  ptr.To(true),
		Content: ptr.To(`[Unit]
Description=kubelet daemon
Documentation=https://kubernetes.io/docs/admin/kubelet
After=` + containerd.UnitName + `
[Install]
WantedBy=multi-user.target
[Service]
Restart=always
RestartSec=5
` +
			// The default for HTTP2_READ_IDLE_TIMEOUT_SECONDS is 30 and for HTTP2_PING_TIMEOUT_SECONDS 15.
			// This results in issues if the tcp connection to kube-apiserver is silently dropped,
			// as node-monitor-grace-period is only 40s.
			// HTTP2_READ_IDLE_TIMEOUT_SECONDS + HTTP2_PING_TIMEOUT_SECONDS should be less than node-monitor-grace-period.
			`Environment="HTTP2_READ_IDLE_TIMEOUT_SECONDS=` + strconv.Itoa(http2ReadIdleTimeSeconds) + `" "HTTP2_PING_TIMEOUT_SECONDS=` + strconv.Itoa(http2PingTimeSeconds) + `"
EnvironmentFile=/etc/environment
EnvironmentFile=-/var/lib/kubelet/extra_args
ExecStart=` + v1beta1constants.OperatingSystemConfigFilePathBinaries + `/kubelet \
    ` + utils.Indent(strings.Join(cliFlags, " \\\n"), 4) + ` $KUBELET_EXTRA_ARGS`),
		FilePaths: extensionsv1alpha1helper.FilePathsFrom(kubeletFiles),
	}

	return []extensionsv1alpha1.Unit{kubeletUnit}, kubeletFiles, nil
}

func getFileContentKubeletConfig(kubernetesVersion *semver.Version, clusterDNSAddresses []string, clusterDomain string, taints []corev1.Taint, params components.ConfigurableKubeletConfigParameters) (*extensionsv1alpha1.FileContentInline, error) {
	var (
		kubeletConfig = Config(kubernetesVersion, clusterDNSAddresses, clusterDomain, taints, params)
		configFCI     = &extensionsv1alpha1.FileContentInline{Encoding: "b64"}
		kcCodec       = NewConfigCodec(oscutils.NewFileContentInlineCodec())
	)

	return kcCodec.Encode(kubeletConfig, configFCI.Encoding)
}
