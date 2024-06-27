#!/usr/bin/env bash

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
ctr images pull --hosts-dir "/etc/containerd/certs.d" "{{ .image }}"
ctr images mount "{{ .image }}" "$tmp_dir"

echo "> Copy gardener-node-agent binary to host ({{ .binaryDirectory }}) and make it executable"
mkdir -p "{{ .binaryDirectory }}"
cp -f "$tmp_dir/ko-app/gardener-node-agent" "{{ .binaryDirectory }}"
chmod +x "{{ .binaryDirectory }}/gardener-node-agent"

echo "> Bootstrap gardener-node-agent"
exec "{{ .binaryDirectory }}/gardener-node-agent" bootstrap --config="{{ .configFile }}"
