// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package nodeinit_test

import (
	"context"
	"fmt"
	"unicode/utf8"

	"github.com/Masterminds/semver/v3"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/component-base/version"

	"github.com/gardener/gardener/extensions/pkg/controller/operatingsystemconfig"
	nodeagentconfigv1alpha1 "github.com/gardener/gardener/pkg/apis/config/nodeagent/v1alpha1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	. "github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/nodeinit"
	nodeagentcomponent "github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components/nodeagent"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components/rootcertificates"
	"github.com/gardener/gardener/pkg/utils"
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
			config = nodeagentcomponent.ComponentConfig(oscSecretName, kubernetesVersion, apiServerURL, nil)
		})

		When("kubelet data volume is not configured", func() {
			It("should return the expected units and files", func() {
				units, files, err := Config(worker, image, config, caBundle, false)

				Expect(err).NotTo(HaveOccurred())
				Expect(units).To(ConsistOf(extensionsv1alpha1.Unit{
					Name:    nodeagentconfigv1alpha1.InitUnitName,
					Command: new(extensionsv1alpha1.CommandStart),
					Enable:  new(true),
					Content: new(`[Unit]
Description=Downloads the gardener-node-agent binary from the container registry and bootstraps it.
Requires=containerd.service
After=containerd.service
After=network-online.target
Wants=network-online.target
[Service]
Type=oneshot
Restart=on-failure
RestartSec=5
StartLimitBurst=0
EnvironmentFile=/etc/environment
ExecStart=/var/lib/gardener-node-agent/init.sh
StandardOutput=journal+console
StandardError=journal+console
[Install]
WantedBy=multi-user.target`),
					FilePaths: []string{"/var/lib/gardener-node-agent/init.sh"},
				}))

				Expect(files).To(ConsistOf(
					extensionsv1alpha1.File{
						Path:        "/var/lib/gardener-node-agent/credentials/bootstrap-token",
						Permissions: new(uint32(0640)),
						Content: extensionsv1alpha1.FileContent{
							Inline: &extensionsv1alpha1.FileContentInline{
								Data: "<<BOOTSTRAP_TOKEN>>",
							},
							TransmitUnencoded: new(true),
						},
					},
					extensionsv1alpha1.File{
						Path:        fmt.Sprintf("/var/lib/gardener-node-agent/config-%s.yaml", version.Get().GitVersion),
						Permissions: new(uint32(0600)),
						Content: extensionsv1alpha1.FileContent{Inline: &extensionsv1alpha1.FileContentInline{Encoding: "b64", Data: utils.EncodeBase64([]byte(`apiServer:
  caFile: ` + nodeagentconfigv1alpha1.ClusterCAFilePath + `
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
  systemdUnitCheck: {}
  token:
    syncPeriod: 12h0m0s
kind: NodeAgentConfiguration
logFormat: ""
logLevel: ""
server: {}
`))}},
					},
					extensionsv1alpha1.File{
						Path:        "/var/lib/gardener-node-agent/init.sh",
						Permissions: new(uint32(0755)),
						Content: extensionsv1alpha1.FileContent{
							Inline: &extensionsv1alpha1.FileContentInline{
								Encoding: "b64",
								Data: utils.EncodeBase64([]byte(`#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

image="` + image + `"

echo "> Prepare temporary directory for image pull and mount"
tmp_dir="$(mktemp -d)"
unmount() {
  ctr images unmount "$tmp_dir" && rm -rf "$tmp_dir"
}
trap unmount EXIT

echo "> Pull gardener-node-agent image and mount it to the temporary directory"
CTR_MAJOR=$(ctr version | grep Version | tail -n1 | awk '{print $2}' | cut -d '.' -f 1 | sed 's/[a-zA-Z]//g')
CTR_EXTRA_ARGS=""
if [ "$CTR_MAJOR" -gt 1 ]; then
    CTR_EXTRA_ARGS="--skip-metadata"
fi
ctr images pull $CTR_EXTRA_ARGS --hosts-dir "/etc/containerd/certs.d" "$image"
ctr images mount "$image" "$tmp_dir"

echo "> Copy gardener-node-agent binary to host (/opt/bin) and make it executable"
mkdir -p "/opt/bin"
cp -f "$tmp_dir/gardener-node-agent" "/opt/bin" || cp -f "$tmp_dir/ko-app/gardener-node-agent" "/opt/bin"
chmod +x "/opt/bin/gardener-node-agent"

echo "> Bootstrap gardener-node-agent"
exec "/opt/bin/gardener-node-agent" bootstrap --config-dir="/var/lib/gardener-node-agent"
`)),
							},
						},
					},
					extensionsv1alpha1.File{
						Path:        "/var/lib/gardener-node-agent/machine-name",
						Permissions: new(uint32(0640)),
						Content: extensionsv1alpha1.FileContent{
							Inline: &extensionsv1alpha1.FileContentInline{
								Data: "<<MACHINE_NAME>>",
							},
							TransmitUnencoded: new(true),
						},
					},
					extensionsv1alpha1.File{
						Path:        "/var/lib/gardener-node-agent/cluster-ca.crt",
						Permissions: new(uint32(0640)),
						Content: extensionsv1alpha1.FileContent{
							Inline: &extensionsv1alpha1.FileContentInline{
								Encoding: "b64",
								Data:     utils.EncodeBase64(caBundle),
							},
						},
					},
				))
			})
		})

		Context("registry CA", func() {
			When("registryCA is enabled", func() {
				It("should use Wants=containerd.service in the unit", func() {
					units, _, err := Config(worker, image, config, caBundle, true)

					Expect(err).NotTo(HaveOccurred())
					Expect(units).To(ConsistOf(extensionsv1alpha1.Unit{
						Name:    nodeagentconfigv1alpha1.InitUnitName,
						Command: new(extensionsv1alpha1.CommandStart),
						Enable:  new(true),
						Content: new(`[Unit]
Description=Downloads the gardener-node-agent binary from the container registry and bootstraps it.
Wants=containerd.service
After=containerd.service
After=network-online.target
Wants=network-online.target
[Service]
Type=oneshot
Restart=on-failure
RestartSec=5
StartLimitBurst=0
EnvironmentFile=/etc/environment
ExecStart=/var/lib/gardener-node-agent/init.sh
StandardOutput=journal+console
StandardError=journal+console
[Install]
WantedBy=multi-user.target`),
						FilePaths: []string{"/var/lib/gardener-node-agent/init.sh"},
					}))
				})

				It("should embed the CA fetch block in the init script and include the update-ca-certificates script file", func() {
					_, files, err := Config(worker, image, config, caBundle, true)

					Expect(err).NotTo(HaveOccurred())
					Expect(files).To(ContainElement(extensionsv1alpha1.File{
						Path:        "/var/lib/gardener-node-agent/init.sh",
						Permissions: new(uint32(0755)),
						Content: extensionsv1alpha1.FileContent{
							Inline: &extensionsv1alpha1.FileContentInline{
								Encoding: "b64",
								Data: utils.EncodeBase64([]byte(`#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

image="` + image + `"

echo "> Prepare temporary directory for image pull and mount"
tmp_dir="$(mktemp -d)"
unmount() {
  ctr images unmount "$tmp_dir" && rm -rf "$tmp_dir"
}
trap unmount EXIT

CA_B64=$(curl -sf $([ -f "/var/lib/gardener-node-agent/cluster-ca.crt" ] && echo "--cacert /var/lib/gardener-node-agent/cluster-ca.crt") \
  --header "Authorization: Bearer $(cat "/var/lib/gardener-node-agent/credentials/bootstrap-token" 2>/dev/null)" \
  "` + apiServerURL + `/api/v1/namespaces/kube-system/configmaps/registry-ca-bundle" \
  2>/dev/null | sed -n 's/.*"ca\.b64"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' || true)
if [ -n "$CA_B64" ]; then
  mkdir -p "/var/lib/ca-certificates-local" "/etc/pki/trust/anchors"
  printf '%s' "$CA_B64" | base64 -d | tee "/var/lib/ca-certificates-local/registry-ca.crt" > "/etc/pki/trust/anchors/registry-ca.pem"
  /var/lib/ssl/update-local-ca-certificates.sh
  systemctl restart containerd
  echo "> Registry CA configured"
fi

echo "> Pull gardener-node-agent image and mount it to the temporary directory"
CTR_MAJOR=$(ctr version | grep Version | tail -n1 | awk '{print $2}' | cut -d '.' -f 1 | sed 's/[a-zA-Z]//g')
CTR_EXTRA_ARGS=""
if [ "$CTR_MAJOR" -gt 1 ]; then
    CTR_EXTRA_ARGS="--skip-metadata"
fi
ctr images pull $CTR_EXTRA_ARGS --hosts-dir "/etc/containerd/certs.d" "$image"
ctr images mount "$image" "$tmp_dir"

echo "> Copy gardener-node-agent binary to host (/opt/bin) and make it executable"
mkdir -p "/opt/bin"
cp -f "$tmp_dir/gardener-node-agent" "/opt/bin" || cp -f "$tmp_dir/ko-app/gardener-node-agent" "/opt/bin"
chmod +x "/opt/bin/gardener-node-agent"

echo "> Bootstrap gardener-node-agent"
exec "/opt/bin/gardener-node-agent" bootstrap --config-dir="/var/lib/gardener-node-agent"
`)),
							},
						},
					}))

					updateCACertsFile, err := rootcertificates.UpdateLocalCACertificatesScriptFile()
					Expect(err).NotTo(HaveOccurred())
					Expect(files).To(ContainElement(updateCACertsFile))
				})
			})

			When("registryCA is disabled", func() {
				It("should use Requires=containerd.service in the unit", func() {
					units, _, err := Config(worker, image, config, caBundle, false)

					Expect(err).NotTo(HaveOccurred())
					Expect(units).To(ConsistOf(extensionsv1alpha1.Unit{
						Name:    nodeagentconfigv1alpha1.InitUnitName,
						Command: new(extensionsv1alpha1.CommandStart),
						Enable:  new(true),
						Content: new(`[Unit]
Description=Downloads the gardener-node-agent binary from the container registry and bootstraps it.
Requires=containerd.service
After=containerd.service
After=network-online.target
Wants=network-online.target
[Service]
Type=oneshot
Restart=on-failure
RestartSec=5
StartLimitBurst=0
EnvironmentFile=/etc/environment
ExecStart=/var/lib/gardener-node-agent/init.sh
StandardOutput=journal+console
StandardError=journal+console
[Install]
WantedBy=multi-user.target`),
						FilePaths: []string{"/var/lib/gardener-node-agent/init.sh"},
					}))
				})

				It("should not embed the CA fetch block in the init script", func() {
					_, files, err := Config(worker, image, config, caBundle, false)

					Expect(err).NotTo(HaveOccurred())
					Expect(files).To(ContainElement(extensionsv1alpha1.File{
						Path:        "/var/lib/gardener-node-agent/init.sh",
						Permissions: new(uint32(0755)),
						Content: extensionsv1alpha1.FileContent{
							Inline: &extensionsv1alpha1.FileContentInline{
								Encoding: "b64",
								Data: utils.EncodeBase64([]byte(`#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

image="` + image + `"

echo "> Prepare temporary directory for image pull and mount"
tmp_dir="$(mktemp -d)"
unmount() {
  ctr images unmount "$tmp_dir" && rm -rf "$tmp_dir"
}
trap unmount EXIT

echo "> Pull gardener-node-agent image and mount it to the temporary directory"
CTR_MAJOR=$(ctr version | grep Version | tail -n1 | awk '{print $2}' | cut -d '.' -f 1 | sed 's/[a-zA-Z]//g')
CTR_EXTRA_ARGS=""
if [ "$CTR_MAJOR" -gt 1 ]; then
    CTR_EXTRA_ARGS="--skip-metadata"
fi
ctr images pull $CTR_EXTRA_ARGS --hosts-dir "/etc/containerd/certs.d" "$image"
ctr images mount "$image" "$tmp_dir"

echo "> Copy gardener-node-agent binary to host (/opt/bin) and make it executable"
mkdir -p "/opt/bin"
cp -f "$tmp_dir/gardener-node-agent" "/opt/bin" || cp -f "$tmp_dir/ko-app/gardener-node-agent" "/opt/bin"
chmod +x "/opt/bin/gardener-node-agent"

echo "> Bootstrap gardener-node-agent"
exec "/opt/bin/gardener-node-agent" bootstrap --config-dir="/var/lib/gardener-node-agent"
`)),
							},
						},
					}))
				})
			})
		})

		When("kubelet data volume is configured", func() {
			BeforeEach(func() {
				worker.KubeletDataVolumeName = new("kubelet-data-vol")
				worker.DataVolumes = []gardencorev1beta1.DataVolume{{
					Name:       *worker.KubeletDataVolumeName,
					VolumeSize: "1337Ki",
				}}
			})

			It("should return an error when the data volume cannot be found", func() {
				*worker.KubeletDataVolumeName = "not-found"

				units, files, err := Config(worker, image, config, caBundle, false)
				Expect(err).To(MatchError(ContainSubstring("failed finding data volume for kubelet in worker with name")))
				Expect(units).To(BeNil())
				Expect(files).To(BeNil())
			})

			It("should correctly configure the bootstrap configuration", func() {
				_, files, err := Config(worker, image, config, caBundle, false)
				Expect(err).NotTo(HaveOccurred())
				Expect(files).To(ContainElement(extensionsv1alpha1.File{
					Path:        fmt.Sprintf("/var/lib/gardener-node-agent/config-%s.yaml", version.Get().GitVersion),
					Permissions: new(uint32(0600)),
					Content: extensionsv1alpha1.FileContent{Inline: &extensionsv1alpha1.FileContentInline{Encoding: "b64", Data: utils.EncodeBase64([]byte(`apiServer:
  caFile: ` + nodeagentconfigv1alpha1.ClusterCAFilePath + `
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
  systemdUnitCheck: {}
  token:
    syncPeriod: 12h0m0s
kind: NodeAgentConfiguration
logFormat: ""
logLevel: ""
server: {}
`))}},
				}))
			})

			It("should ensure the size of the configuration is not exceeding a certain limit", func() {
				units, files, err := Config(worker, image, config, caBundle, false)
				Expect(err).NotTo(HaveOccurred())

				writeFilesToDiskScript, err := operatingsystemconfig.FilesToDiskScript(context.Background(), nil, "", files)
				Expect(err).NotTo(HaveOccurred())
				writeUnitsToDiskScript := operatingsystemconfig.UnitsToDiskScript(units)

				// best-effort check: ensure the node init configuration is not exceeding 4KB in size
				Expect(utf8.RuneCountInString(writeFilesToDiskScript + writeUnitsToDiskScript)).To(BeNumerically("<", 4096))
			})
		})
	})
})
