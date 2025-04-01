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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener/imagevector"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	extensionsv1alpha1helper "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1/helper"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components/containerd"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components/rootcertificates"
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
			Permissions: ptr.To[uint32](0644),
			Content: extensionsv1alpha1.FileContent{
				Inline: &extensionsv1alpha1.FileContentInline{
					Encoding: "b64",
					Data:     utils.EncodeBase64(ctx.KubeletCABundle),
				},
			},
		},
		{
			Path:        PathKubeletConfig,
			Permissions: ptr.To[uint32](0600),
			Content: extensionsv1alpha1.FileContent{
				Inline: fileContentKubeletConfig,
			},
		},
		{
			Path:        v1beta1constants.OperatingSystemConfigFilePathBinaries + "/kubelet",
			Permissions: ptr.To[uint32](0755),
			Content: extensionsv1alpha1.FileContent{
				ImageRef: &extensionsv1alpha1.FileContentImageRef{
					Image:           ctx.Images[imagevector.ContainerImageNameHyperkube].String(),
					FilePathInImage: "/kubelet",
				},
			},
		},
	}

	cliFlags := CLIFlags(ctx.KubernetesVersion, ctx.NodeLabels, ctx.CRIName, ctx.KubeletCLIFlags, ctx.PreferIPv6)

	http2ReadIdleTimeSeconds, http2PingTimeSeconds := calcKubeletHTTP2Timeouts(ctx.NodeMonitorGracePeriod)

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
Environment="HTTP2_READ_IDLE_TIMEOUT_SECONDS=` + strconv.Itoa(http2ReadIdleTimeSeconds) + `" "HTTP2_PING_TIMEOUT_SECONDS=` + strconv.Itoa(http2PingTimeSeconds) + `"
EnvironmentFile=/etc/environment
EnvironmentFile=-/var/lib/kubelet/extra_args
ExecStart=` + v1beta1constants.OperatingSystemConfigFilePathBinaries + `/kubelet \
    ` + utils.Indent(strings.Join(cliFlags, " \\\n"), 4) + ` $KUBELET_EXTRA_ARGS`),
		FilePaths: append(extensionsv1alpha1helper.FilePathsFrom(kubeletFiles), rootcertificates.PathLocalSSLRootCerts),
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

// The default for HTTP2_READ_IDLE_TIMEOUT_SECONDS is 30 and for HTTP2_PING_TIMEOUT_SECONDS 15.
// This results in issues if the tcp connection to kube-apiserver is silently dropped,
// as node-monitor-grace-period is only 40s.
// HTTP2_READ_IDLE_TIMEOUT_SECONDS + HTTP2_PING_TIMEOUT_SECONDS should be less than node-monitor-grace-period.
func calcKubeletHTTP2Timeouts(nodeMonitorGracePeriod metav1.Duration) (int, int) {
	http2ReadIdleTimeSeconds := int(30)
	http2PingTimeSeconds := int(15)

	if nodeMonitorGracePeriod.Seconds() < 46 {
		http2ReadIdleTimeSeconds = positiveOrZero(int((nodeMonitorGracePeriod.Seconds() - 2) * 2 / 3))
		http2PingTimeSeconds = positiveOrZero(int((nodeMonitorGracePeriod.Seconds() - 2) * 1 / 3))
	}
	return http2ReadIdleTimeSeconds, http2PingTimeSeconds
}

func positiveOrZero(v int) int {
	if v > 0 {
		return v
	}
	return 0
}
