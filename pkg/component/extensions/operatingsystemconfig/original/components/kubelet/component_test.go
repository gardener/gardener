// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package kubelet_test

import (
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components"
	. "github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components/kubelet"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/imagevector"
)

var _ = Describe("Component", func() {
	var (
		component components.Component
		ctx       components.Context

		kubeletCABundle       = []byte("certificate")
		kubeletCABundleBase64 = utils.EncodeBase64(kubeletCABundle)
		kubeletConfig         = `apiVersion: kubelet.config.k8s.io/v1beta1
authentication:
  anonymous:
    enabled: false
  webhook:
    cacheTTL: 2m0s
    enabled: true
  x509:
    clientCAFile: /var/lib/kubelet/ca.crt
authorization:
  mode: Webhook
  webhook:
    cacheAuthorizedTTL: 5m0s
    cacheUnauthorizedTTL: 30s
cgroupDriver: cgroupfs
cgroupRoot: /
cgroupsPerQOS: true
clusterDNS:
- 2001::db8:1
- 2001::db8:2
containerLogMaxSize: 100Mi
containerRuntimeEndpoint: ""
cpuCFSQuota: true
cpuManagerPolicy: none
cpuManagerReconcilePeriod: 10s
crashLoopBackOff: {}
enableControllerAttachDetach: true
enableDebuggingHandlers: true
enableServer: true
enforceNodeAllocatable:
- pods
eventBurst: 50
eventRecordQPS: 50
evictionHard:
  imagefs.available: 5%
  imagefs.inodesFree: 5%
  memory.available: 100Mi
  nodefs.available: 5%
  nodefs.inodesFree: 5%
evictionMaxPodGracePeriod: 90
evictionMinimumReclaim:
  imagefs.available: 0Mi
  imagefs.inodesFree: 0Mi
  memory.available: 0Mi
  nodefs.available: 0Mi
  nodefs.inodesFree: 0Mi
evictionPressureTransitionPeriod: 4m0s
evictionSoft:
  imagefs.available: 10%
  imagefs.inodesFree: 10%
  memory.available: 200Mi
  nodefs.available: 10%
  nodefs.inodesFree: 10%
evictionSoftGracePeriod:
  imagefs.available: 1m30s
  imagefs.inodesFree: 1m30s
  memory.available: 1m30s
  nodefs.available: 1m30s
  nodefs.inodesFree: 1m30s
failSwapOn: true
fileCheckFrequency: 20s
hairpinMode: promiscuous-bridge
httpCheckFrequency: 20s
imageGCHighThresholdPercent: 50
imageGCLowThresholdPercent: 40
imageMaximumGCAge: 0s
imageMinimumGCAge: 2m0s
kind: KubeletConfiguration
kubeAPIBurst: 50
kubeAPIQPS: 50
kubeReserved:
  cpu: 80m
  memory: 1Gi
logging:
  flushFrequency: 0
  options:
    json:
      infoBufferSize: "0"
    text:
      infoBufferSize: "0"
  verbosity: 0
maxOpenFiles: 1000000
maxPods: 110
memorySwap: {}
nodeStatusReportFrequency: 0s
nodeStatusUpdateFrequency: 0s
protectKernelDefaults: true
registerWithTaints:
- effect: NoSchedule
  key: node.gardener.cloud/critical-components-not-ready
resolvConf: /etc/resolv.conf
rotateCertificates: true
runtimeRequestTimeout: 2m0s
serializeImagePulls: true
serverTLSBootstrap: true
shutdownGracePeriod: 0s
shutdownGracePeriodCriticalPods: 0s
streamingConnectionIdleTimeout: 5m0s
syncFrequency: 1m0s
volumePluginDir: /var/lib/kubelet/volumeplugins
volumeStatsAggPeriod: 1m0s
`
	)

	BeforeEach(func() {
		component = New()
		ctx = components.Context{}
	})

	DescribeTable("#Config",
		func(kubernetesVersion string, kubeletConfig string, preferIPv6 bool) {

			ctx.CRIName = extensionsv1alpha1.CRINameContainerD
			ctx.KubernetesVersion = semver.MustParse(kubernetesVersion)
			ctx.KubeletCABundle = kubeletCABundle
			ctx.Images = map[string]*imagevector.Image{
				"hyperkube": {
					Name:       "pause-container",
					Repository: ptr.To(hyperkubeImageRepo),
					Tag:        ptr.To(hyperkubeImageTag),
				},
				"pause-container": {
					Name:       "pause-container",
					Repository: ptr.To(pauseContainerImageRepo),
					Tag:        ptr.To(pauseContainerImageTag),
				},
			}
			ctx.NodeLabels = map[string]string{
				"test": "foo",
				"blub": "bar",
			}
			ctx.PreferIPv6 = preferIPv6
			ctx.ClusterDNSAddresses = []string{"2001::db8:1", "2001::db8:2"}
			ctx.NodeMonitorGracePeriod.Duration = time.Duration(40) * time.Second

			cliFlags := CLIFlags(ctx.KubernetesVersion, ctx.NodeLabels, ctx.CRIName, ctx.KubeletCLIFlags, ctx.PreferIPv6)
			units, files, err := component.Config(ctx)

			Expect(err).NotTo(HaveOccurred())

			Expect(units).To(ConsistOf(
				kubeletUnit(cliFlags),
			))
			Expect(files).To(ConsistOf(kubeletFiles(ctx, kubeletConfig, kubeletCABundleBase64)))
		},

		Entry(
			"kubernetes 1.27",
			"1.27.1",
			kubeletConfig,
			false,
		),
		Entry(
			"kubernetes 1.27 and preferIPv6",
			"1.27.1",
			kubeletConfig,
			true,
		),
	)
})

const (
	pauseContainerImageRepo = "foo.io"
	pauseContainerImageTag  = "v1.2.3"
	hyperkubeImageRepo      = "hyperkube.io"
	hyperkubeImageTag       = "v4.5.6"
)

func kubeletUnit(cliFlags []string) extensionsv1alpha1.Unit {
	var kubeletStartPre string

	unit := extensionsv1alpha1.Unit{
		Name:    "kubelet.service",
		Command: ptr.To(extensionsv1alpha1.CommandStart),
		Enable:  ptr.To(true),
		Content: ptr.To(`[Unit]
Description=kubelet daemon
Documentation=https://kubernetes.io/docs/admin/kubelet
After=containerd.service
[Install]
WantedBy=multi-user.target
[Service]
Restart=always
RestartSec=5
Environment="HTTP2_READ_IDLE_TIMEOUT_SECONDS=25" "HTTP2_PING_TIMEOUT_SECONDS=12"
EnvironmentFile=/etc/environment
EnvironmentFile=-/var/lib/kubelet/extra_args` + kubeletStartPre + `
ExecStart=/opt/bin/kubelet \
    ` + utils.Indent(strings.Join(cliFlags, " \\\n"), 4) + ` $KUBELET_EXTRA_ARGS`),
		FilePaths: []string{"/var/lib/kubelet/ca.crt", "/var/lib/kubelet/config/kubelet", "/opt/bin/kubelet", "/var/lib/ca-certificates-local/ROOTcerts.crt"},
	}

	return unit
}

func kubeletFiles(ctx components.Context, kubeletConfig, kubeletCABundleBase64 string) []extensionsv1alpha1.File {
	files := []extensionsv1alpha1.File{
		{
			Path:        "/var/lib/kubelet/ca.crt",
			Permissions: ptr.To[uint32](0644),
			Content: extensionsv1alpha1.FileContent{
				Inline: &extensionsv1alpha1.FileContentInline{
					Encoding: "b64",
					Data:     kubeletCABundleBase64,
				},
			},
		},
		{
			Path:        "/var/lib/kubelet/config/kubelet",
			Permissions: ptr.To[uint32](0600),
			Content: extensionsv1alpha1.FileContent{
				Inline: &extensionsv1alpha1.FileContentInline{
					Encoding: "b64",
					Data:     utils.EncodeBase64([]byte(kubeletConfig)),
				},
			},
		},
	}

	files = append(files, extensionsv1alpha1.File{
		Path:        "/opt/bin/kubelet",
		Permissions: ptr.To[uint32](0755),
		Content: extensionsv1alpha1.FileContent{
			ImageRef: &extensionsv1alpha1.FileContentImageRef{
				Image:           ctx.Images["hyperkube"].String(),
				FilePathInImage: "/kubelet",
			},
		},
	})

	return files
}
