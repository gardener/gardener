// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package nodeinit_test

import (
	"context"
	"unicode/utf8"

	"github.com/Masterminds/semver/v3"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener/extensions/pkg/controller/operatingsystemconfig"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	. "github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/nodeinit"
	nodeagentcomponent "github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components/nodeagent"
	"github.com/gardener/gardener/pkg/features"
	nodeagentconfigv1alpha1 "github.com/gardener/gardener/pkg/nodeagent/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/test"
)

var _ = Describe("Init", func() {
	Describe("#Config", func() {
		var (
			worker gardencorev1beta1.Worker
			image  = "gna-repo:gna-tag"

			config            *nodeagentconfigv1alpha1.NodeAgentConfiguration
			oscSecretName     = "osc-secret-name"
			apiServerURL      = "https://localhost"
			caBundle          = []byte("cluster-ca")
			kubernetesVersion = semver.MustParse("1.2.3")
		)

		BeforeEach(func() {
			worker = gardencorev1beta1.Worker{}
			config = nodeagentcomponent.ComponentConfig(oscSecretName, kubernetesVersion, apiServerURL, caBundle, nil)
		})

		When("kubelet data volume is not configured", func() {
			It("should return the expected units and files", func() {
				units, files, err := Config(worker, image, config)

				Expect(err).NotTo(HaveOccurred())
				Expect(units).To(ConsistOf(extensionsv1alpha1.Unit{
					Name:    nodeagentconfigv1alpha1.InitUnitName,
					Command: ptr.To(extensionsv1alpha1.CommandStart),
					Enable:  ptr.To(true),
					Content: ptr.To(`[Unit]
Description=Downloads the gardener-node-agent binary from the container registry and bootstraps it.
After=network-online.target
Wants=network-online.target
[Service]
Type=oneshot
Restart=on-failure
RestartSec=5
StartLimitBurst=0
EnvironmentFile=/etc/environment
ExecStart=/var/lib/gardener-node-agent/init.sh
[Install]
WantedBy=multi-user.target`),
					FilePaths: []string{"/var/lib/gardener-node-agent/init.sh"},
				}))
				Expect(files).To(ConsistOf(
					extensionsv1alpha1.File{
						Path:        "/var/lib/gardener-node-agent/credentials/bootstrap-token",
						Permissions: ptr.To[uint32](0640),
						Content: extensionsv1alpha1.FileContent{
							Inline: &extensionsv1alpha1.FileContentInline{
								Data: "<<BOOTSTRAP_TOKEN>>",
							},
							TransmitUnencoded: ptr.To(true),
						},
					},
					extensionsv1alpha1.File{
						Path:        "/var/lib/gardener-node-agent/config.yaml",
						Permissions: ptr.To[uint32](0600),
						Content: extensionsv1alpha1.FileContent{Inline: &extensionsv1alpha1.FileContentInline{Encoding: "b64", Data: utils.EncodeBase64([]byte(`apiServer:
  caBundle: ` + utils.EncodeBase64(caBundle) + `
  server: ` + apiServerURL + `
apiVersion: nodeagent.config.gardener.cloud/v1alpha1
bootstrap: {}
clientConnection:
  acceptContentTypes: ""
  burst: 0
  contentType: ""
  kubeconfig: ""
  qps: 0
controllers:
  operatingSystemConfig:
    kubernetesVersion: ` + kubernetesVersion.String() + `
    secretName: ` + oscSecretName + `
  token:
    syncPeriod: 12h0m0s
featureGates:
  NodeAgentAuthorizer: true
kind: NodeAgentConfiguration
logFormat: ""
logLevel: ""
server: {}
`))}},
					},
					extensionsv1alpha1.File{
						Path:        "/var/lib/gardener-node-agent/init.sh",
						Permissions: ptr.To[uint32](0755),
						Content: extensionsv1alpha1.FileContent{
							Inline: &extensionsv1alpha1.FileContentInline{
								Encoding: "b64",
								Data: utils.EncodeBase64([]byte(`#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

echo "> Prepare temporary directory for image pull and mount"
tmp_dir="$(mktemp -d)"
unmount() {
  ctr images unmount "$tmp_dir" && rm -rf "$tmp_dir"
}
trap unmount EXIT

echo "> Pull gardener-node-agent image and mount it to the temporary directory"
ctr images pull --hosts-dir "/etc/containerd/certs.d" "` + image + `"
ctr images mount "` + image + `" "$tmp_dir"

echo "> Copy gardener-node-agent binary to host (/opt/bin) and make it executable"
mkdir -p "/opt/bin"
cp -f "$tmp_dir/gardener-node-agent" "/opt/bin/gardener-node-agent"
chmod +x "/opt/bin/gardener-node-agent"

echo "> Bootstrap gardener-node-agent"
exec "/opt/bin/gardener-node-agent" bootstrap --config="/var/lib/gardener-node-agent/config.yaml"
`)),
							},
						},
					},
					extensionsv1alpha1.File{
						Path:        "/var/lib/gardener-node-agent/machine-name",
						Permissions: ptr.To[uint32](0640),
						Content: extensionsv1alpha1.FileContent{
							Inline: &extensionsv1alpha1.FileContentInline{
								Data: "<<MACHINE_NAME>>",
							},
							TransmitUnencoded: ptr.To(true),
						},
					},
				))
			})
		})

		When("kubelet data volume is configured", func() {
			BeforeEach(func() {
				worker.KubeletDataVolumeName = ptr.To("kubelet-data-vol")
				worker.DataVolumes = []gardencorev1beta1.DataVolume{{
					Name:       *worker.KubeletDataVolumeName,
					VolumeSize: "1337Ki",
				}}
			})

			It("should return an error when the data volume cannot be found", func() {
				*worker.KubeletDataVolumeName = "not-found"

				units, files, err := Config(worker, image, config)
				Expect(err).To(MatchError(ContainSubstring("failed finding data volume for kubelet in worker with name")))
				Expect(units).To(BeNil())
				Expect(files).To(BeNil())
			})

			It("should correctly configure the bootstrap configuration", func() {
				_, files, err := Config(worker, image, config)
				Expect(err).NotTo(HaveOccurred())
				Expect(files).To(ContainElement(extensionsv1alpha1.File{
					Path:        "/var/lib/gardener-node-agent/config.yaml",
					Permissions: ptr.To[uint32](0600),
					Content: extensionsv1alpha1.FileContent{Inline: &extensionsv1alpha1.FileContentInline{Encoding: "b64", Data: utils.EncodeBase64([]byte(`apiServer:
  caBundle: ` + utils.EncodeBase64(caBundle) + `
  server: ` + apiServerURL + `
apiVersion: nodeagent.config.gardener.cloud/v1alpha1
bootstrap:
  kubeletDataVolumeSize: 1369088
clientConnection:
  acceptContentTypes: ""
  burst: 0
  contentType: ""
  kubeconfig: ""
  qps: 0
controllers:
  operatingSystemConfig:
    kubernetesVersion: ` + kubernetesVersion.String() + `
    secretName: ` + oscSecretName + `
  token:
    syncPeriod: 12h0m0s
featureGates:
  NodeAgentAuthorizer: true
kind: NodeAgentConfiguration
logFormat: ""
logLevel: ""
server: {}
`))}},
				}))
			})

			It("should ensure the size of the configuration is not exceeding a certain limit", func() {
				units, files, err := Config(worker, image, config)
				Expect(err).NotTo(HaveOccurred())

				writeFilesToDiskScript, err := operatingsystemconfig.FilesToDiskScript(context.Background(), nil, "", files)
				Expect(err).NotTo(HaveOccurred())
				writeUnitsToDiskScript := operatingsystemconfig.UnitsToDiskScript(units)

				// best-effort check: ensure the node init configuration is not exceeding 4KB in size
				Expect(utf8.RuneCountInString(writeFilesToDiskScript + writeUnitsToDiskScript)).To(BeNumerically("<", 4096))
			})
		})

		When("NodeAgentAuthorizer feature gate is disabled", func() {
			BeforeEach(func() {
				DeferCleanup(test.WithFeatureGate(features.DefaultFeatureGate, features.NodeAgentAuthorizer, false))
				config = nodeagentcomponent.ComponentConfig(oscSecretName, kubernetesVersion, apiServerURL, caBundle, nil)
			})

			It("should correctly configure the bootstrap configuration", func() {
				_, files, err := Config(worker, image, config)
				Expect(err).NotTo(HaveOccurred())
				Expect(files).To(ContainElement(extensionsv1alpha1.File{
					Path:        "/var/lib/gardener-node-agent/config.yaml",
					Permissions: ptr.To[uint32](0600),
					Content: extensionsv1alpha1.FileContent{Inline: &extensionsv1alpha1.FileContentInline{Encoding: "b64", Data: utils.EncodeBase64([]byte(`apiServer:
  caBundle: ` + utils.EncodeBase64(caBundle) + `
  server: ` + apiServerURL + `
apiVersion: nodeagent.config.gardener.cloud/v1alpha1
bootstrap: {}
clientConnection:
  acceptContentTypes: ""
  burst: 0
  contentType: ""
  kubeconfig: ""
  qps: 0
controllers:
  operatingSystemConfig:
    kubernetesVersion: ` + kubernetesVersion.String() + `
    secretName: ` + oscSecretName + `
  token:
    syncConfigs:
    - path: /var/lib/gardener-node-agent/credentials/token
      secretName: gardener-node-agent
    syncPeriod: 12h0m0s
featureGates:
  NodeAgentAuthorizer: false
kind: NodeAgentConfiguration
logFormat: ""
logLevel: ""
server: {}
`))}},
				}))
			})
		})
	})
})
