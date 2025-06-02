#!/usr/bin/env bash
#
# SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

set -e

usage() {
  echo "Usage:"
  echo "> compare-k8s-admission-plugins.sh [ -h | <old version> <new version> ]"
  echo
  echo ">> For example: compare-k8s-admission-plugins.sh 1.26 1.27"

  exit 0
}

if [ "$1" == "-h" ] || [ "$#" -ne 2 ]; then
  usage
fi

versions=("$1" "$2")

options_plugins="pkg/kubeapiserver/options/plugins.go"
server_plugins="staging/src/k8s.io/apiserver/pkg/server/plugins.go"

out_dir=$(mktemp -d)
function cleanup_output {
    rm -rf "$out_dir"
}
trap cleanup_output EXIT

for version in "${versions[@]}"; do
  rm -f "${out_dir}/admissionplugins-${version}.txt" "${out_dir}/admissionplugins-${version}.txt"
  touch "${out_dir}/admissionplugins-${version}.txt" "${out_dir}/admissionplugins-${version}.txt"

  wget -nv -O "${out_dir}/options_plugins.go" "https://raw.githubusercontent.com/kubernetes/kubernetes/release-${version}/${options_plugins}" > /dev/null 2> >(sed '/plugins.go/d' >&2)
  wget -nv -O "${out_dir}/server_plugins.go" "https://raw.githubusercontent.com/kubernetes/kubernetes/release-${version}/${server_plugins}" > /dev/null 2> >(sed '/plugins.go/d' >&2)
  awk '/var AllOrderedPlugins = \[\]string\{/,/\}/' "${out_dir}/options_plugins.go" > "${out_dir}/ordered_admission_plugins.txt"
  grep  '\.Register' "${out_dir}/options_plugins.go" | awk '{print $1}' | { grep -Eo '^[a-z]\w+' || true; } > "${out_dir}/plugin_packages.txt"
  grep  '\.Register' "${out_dir}/server_plugins.go" | awk '{print $1}' | { grep -Eo '^[a-z]\w+' || true; } >> "${out_dir}/plugin_packages.txt"
  while read -r plugin_package; do
    grep -E "\s+${plugin_package}\..*,.*" "${out_dir}/ordered_admission_plugins.txt" | { grep -Eo '//\s*[a-z|A-Z]\w+' | tr -d '//' | tr -d ' ' || true; }  >> "${out_dir}/admissionplugins-${version}.txt"
  done < "${out_dir}/plugin_packages.txt"

  sort -u -o "${out_dir}/admissionplugins-${version}.txt" "${out_dir}/admissionplugins-${version}.txt"
done

echo "Admission plugins added in $2 compared to $1:"
diff "${out_dir}/admissionplugins-$1.txt" "${out_dir}/admissionplugins-$2.txt" | grep '>' | awk '{print $2}'
echo
echo "Admission plugins removed in $2 compared to $1:"
diff "${out_dir}/admissionplugins-$1.txt" "${out_dir}/admissionplugins-$2.txt" | grep '<' | awk '{print $2}'
echo
