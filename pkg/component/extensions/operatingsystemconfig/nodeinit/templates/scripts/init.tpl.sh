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
{{- /*
ctr v2 pulls manifests for all platforms of a multi-arch image by default. Mirror registries
might not copy manifests for unused architectures, which causes the default pull command to fail.
Ref: https://github.com/containerd/containerd/pull/9029#issuecomment-1706963854
*/}}
CTR_MAJOR=$(ctr version | grep Version | tail -n1 | awk '{print $2}' | cut -d '.' -f 1 | sed 's/[a-zA-Z]//g')
CTR_EXTRA_ARGS=""
if [ "$CTR_MAJOR" -gt 1 ]; then
    CTR_EXTRA_ARGS="--skip-metadata"
fi
ctr images pull $CTR_EXTRA_ARGS --hosts-dir "/etc/containerd/certs.d" "{{ .image }}"
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
exec "{{ .binaryDirectory }}/{{ .binaryName }}" bootstrap --config-dir="{{ .configDir }}"
{{- end }}
