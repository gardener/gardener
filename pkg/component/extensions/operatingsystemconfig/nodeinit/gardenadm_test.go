// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package nodeinit_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	. "github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/nodeinit"
	"github.com/gardener/gardener/pkg/utils"
)

var _ = Describe("Init", func() {
	var image = "gardenadm:tag"
	Describe("#GardenadmConfig", func() {
		It("should return the correct units and files", func() {
			units, files, err := GardenadmConfig(image)
			Expect(err).NotTo(HaveOccurred())

			Expect(units).To(ConsistOf(extensionsv1alpha1.Unit{
				Name:    "gardenadm-download.service",
				Command: ptr.To(extensionsv1alpha1.CommandStart),
				Enable:  ptr.To(true),
				Content: ptr.To(`[Unit]
Description=Downloads the gardenadm binary from the container registry and bootstraps it.
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
ExecStart=/var/lib/gardenadm/download.sh
[Install]
WantedBy=multi-user.target`),
				FilePaths: []string{"/var/lib/gardenadm/download.sh"},
			}))

			Expect(files).To(ConsistOf(extensionsv1alpha1.File{
				Path:        "/var/lib/gardenadm/download.sh",
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

echo "> Pull gardenadm image and mount it to the temporary directory"
ctr images pull --hosts-dir "/etc/containerd/certs.d" "` + image + `"
ctr images mount "` + image + `" "$tmp_dir"

echo "> Copy gardenadm binary to host (/opt/bin) and make it executable"
mkdir -p "/opt/bin"
cp -f "$tmp_dir/gardenadm" "/opt/bin" || cp -f "$tmp_dir/ko-app/gardenadm" "/opt/bin"
chmod +x "/opt/bin/gardenadm"


`)),
					},
				},
			}))
		})
	})
})
