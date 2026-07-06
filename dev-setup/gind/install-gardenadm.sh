#!/usr/bin/env bash
# SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

set -o errexit
set -o nounset
set -o pipefail

echo "> Prepare temporary directory for image pull and mount"
tmp_dir="$(mktemp -d)"
unmount() {
  ctr images unmount "$tmp_dir" && rm -rf "$tmp_dir"
}
trap unmount EXIT

image="$1"

echo "> Pull gardenadm image and mount it to the temporary directory"
ctr images pull --hosts-dir "/etc/containerd/certs.d" "$image"
ctr images mount "$image" "$tmp_dir"

echo "> Copy gardenadm binary to host (/gardenadm) and make it executable"
mkdir -p "/gardenadm"
cp -f "$tmp_dir/ko-app/gardenadm" "/gardenadm"
chmod +x "/gardenadm/gardenadm"
