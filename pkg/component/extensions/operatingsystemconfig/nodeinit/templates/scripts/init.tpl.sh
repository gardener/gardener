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

echo "> Pull {{ .binaryName }} image and mount it to the temporary directory"
ctr images pull --hosts-dir "/etc/containerd/certs.d" "{{ .image }}"
ctr images mount "{{ .image }}" "$tmp_dir"

echo "> Copy {{ .binaryName }} binary to host ({{ .binaryDirectory }}) and make it executable"
mkdir -p "{{ .binaryDirectory }}"

{{- /*
Fall back to /ko-app/<binary-name> if /<binary-name> doesn't exist in image to support images built with ko.
TODO(timebertt): remove this fallback once https://github.com/ko-build/ko/pull/1403 has been released and is used to
 build images in the skaffold-based setup (add a breaking release note!).
*/}}
cp -f "$tmp_dir/{{ .binaryName }}" "{{ .binaryDirectory }}" || cp -f "$tmp_dir/ko-app/{{ .binaryName }}" "{{ .binaryDirectory }}"
chmod +x "{{ .binaryDirectory }}/{{ .binaryName }}"
{{- if eq .binaryName "gardener-node-agent" }}

echo "> Bootstrap {{ .binaryName }}"
exec "{{ .binaryDirectory }}/{{ .binaryName }}" bootstrap --config="{{ .configFile }}"
{{- end }}
