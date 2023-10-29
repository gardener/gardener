#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

echo "> Prepare temporary directory for image pull and mount"
tmp_dir="$(mktemp -d)"
trap 'ctr images unmount "$tmp_dir" && rm -rf "$tmp_dir"' EXIT

echo "> Pull gardener-node-agent image and mount it to the temporary directory"
ctr images pull  "{{ .image }}"
ctr images mount "{{ .image }}" "$tmp_dir"

echo "> Copy gardener-node-agent binary to host ({{ .binaryDirectory }}) and make it executable"
cp -f "$tmp_dir/gardener-node-agent" "{{ .binaryDirectory }}"
chmod +x "{{ .binaryDirectory }}/gardener-node-agent"

echo "> Bootstrap gardener-node-agent"
"{{ .binaryDirectory }}/gardener-node-agent" bootstrap --config="{{ .configFile }}"
