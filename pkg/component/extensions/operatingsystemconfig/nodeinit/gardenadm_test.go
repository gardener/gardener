// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package nodeinit_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	. "github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/nodeinit"
	"github.com/gardener/gardener/pkg/utils"
)

var _ = Describe("Init", func() {
	const sshPublicKey = "ssh-rsa foobar"

	Describe("#GardenadmConfig", func() {
		It("should return the correct units and files", func() {
			units, files, err := GardenadmConfig(sshPublicKey)
			Expect(err).NotTo(HaveOccurred())

			Expect(units).To(ConsistOf(
				And(
					HaveField("Name", "gardener-user.service"),
					HaveField("Enable", HaveValue(BeTrue())),
				),
				And(
					HaveField("Name", "gardener-user.path"),
					HaveField("Enable", HaveValue(BeTrue())),
				),
			))

			Expect(files).To(ConsistOf(
				HaveField("Path", "/var/lib/gardener-user/run.sh"),
				And(
					HaveField("Path", "/var/lib/gardener-user-authorized-keys"),
					HaveField("Content.Inline.Data", utils.EncodeBase64([]byte(sshPublicKey))),
				),
				extensionsv1alpha1.File{
					Path:        "/var/lib/gardenadm/download.sh",
					Permissions: new(uint32(0755)),
					Content: extensionsv1alpha1.FileContent{
						Inline: &extensionsv1alpha1.FileContentInline{
							Encoding: "b64",
							Data: utils.EncodeBase64([]byte(`#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

if [ -z "$1" ]; then
  echo "Usage: $0 <image-ref>"
  exit 1
fi
image="$1"

echo "> Prepare temporary directory for image pull and mount"
tmp_dir="$(mktemp -d)"
unmount() {
  ctr images unmount "$tmp_dir" 2>/dev/null || true
  rm -rf "$tmp_dir"
}
trap unmount EXIT

echo "> Pull gardenadm image and mount it to the temporary directory"
CTR_MAJOR=$(ctr version | grep Version | tail -n1 | awk '{print $2}' | cut -d '.' -f 1 | sed 's/[a-zA-Z]//g')
CTR_EXTRA_ARGS=""
if [ "$CTR_MAJOR" -gt 1 ]; then
    CTR_EXTRA_ARGS="--skip-metadata"
fi
ctr images pull $CTR_EXTRA_ARGS --hosts-dir "/etc/containerd/certs.d" "$image"
if [ "$CTR_MAJOR" -gt 1 ]; then
    echo "> containerd v2.x detected: using export+extract instead of mount (ctr images mount fails on FIPS kernels)"
    ctr images export "$tmp_dir/image.tar" "$image"
    tar -xf "$tmp_dir/image.tar" -C "$tmp_dir"
    for blob in "$tmp_dir"/blobs/sha256/*; do
        if file "$blob" 2>/dev/null | grep -q "gzip\|tar"; then
            if tar -tf "$blob" 2>/dev/null | grep -q "gardenadm"; then
                tar -xf "$blob" -C "$tmp_dir" 2>/dev/null
                break
            fi
        fi
    done
else
    ctr images mount "$image" "$tmp_dir"
fi

echo "> Copy gardenadm binary to host (/opt/bin) and make it executable"
mkdir -p "/opt/bin"
cp -f "$tmp_dir/gardenadm" "/opt/bin" || cp -f "$tmp_dir/ko-app/gardenadm" "/opt/bin"
chmod +x "/opt/bin/gardenadm"
`)),
						},
					},
				},
				And(
					HaveField("Path", "/var/lib/gardener-node-agent/machine-name"),
					HaveField("Content.Inline.Data", "<<MACHINE_NAME>>"),
					HaveField("Content.TransmitUnencoded", HaveValue(BeTrue())),
				),
			))
		})
	})
})
