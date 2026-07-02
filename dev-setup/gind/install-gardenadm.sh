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
  ctr images unmount "$tmp_dir" 2>/dev/null || true
  rm -rf "$tmp_dir"
}
trap unmount EXIT

image="$1"

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

echo "> Copy gardenadm binary to host (/gardenadm) and make it executable"
mkdir -p "/gardenadm"
cp -f "$tmp_dir/ko-app/gardenadm" "/gardenadm"
chmod +x "/gardenadm/gardenadm"
