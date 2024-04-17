#!/usr/bin/env bash
#
# SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

set -e

usage() {
  echo "Usage:"
  echo "> compare-k8s-feature-gates.sh [ -h | <old version> <new version> ]"
  echo
  echo ">> For example: compare-k8s-feature-gates.sh 1.26 1.27"

  exit 0
}

if [ "$1" == "-h" ] || [ "$#" -ne 2 ]; then
  usage
fi

versions=("$1" "$2")
files=(
  "pkg/features/kube_features.go"
  "staging/src/k8s.io/apiserver/pkg/features/kube_features.go"
  "staging/src/k8s.io/apiextensions-apiserver/pkg/features/kube_features.go"
  "staging/src/k8s.io/controller-manager/pkg/features/kube_features.go"
)

out_dir=$(mktemp -d)
function cleanup_output {
    rm -rf "$out_dir"
}
trap cleanup_output EXIT

for version in "${versions[@]}"; do
  rm -f "${out_dir}/featuregates-${version}.txt" "${out_dir}/locked-featuregates-${version}.txt"
  touch "${out_dir}/featuregates-${version}.txt" "${out_dir}/locked-featuregates-${version}.txt"

  for file in "${files[@]}"; do
    { wget -q -O - "https://raw.githubusercontent.com/kubernetes/kubernetes/release-${version}/${file}" || echo; } > "${out_dir}/kube_features.go"
    grep -E '{Default: .*, PreRelease: .*},' "${out_dir}/kube_features.go" | awk '{print $1}' | { grep -Eo '[A-Z]\w+' || true; } > "${out_dir}/constants.txt"
    while read -r constant; do
      grep -E "${constant} featuregate.Feature = \".*\"" "${out_dir}/kube_features.go" | awk '{print $4}' | { grep -Eo '[A-Z]\w+' || true; } >> "${out_dir}/featuregates-${version}.txt"
    done < "${out_dir}/constants.txt"

    grep -E '{Default: .*, PreRelease: .*, LockToDefault: .*},' "${out_dir}/kube_features.go" | sed -En 's/([A-Z]\w+)(: +\{)(Default: (true|false)).*$/\1 \4/p' > "${out_dir}/locked_features.txt"
    while read -r feature; do
      name=$(echo "$feature" | awk '{print $1}' )
      default="$(echo "$feature" | awk '{print $2}')"
      grep -E "${name} featuregate.Feature = \".*\"" "${out_dir}/kube_features.go" | sed -En 's/^.*\"([A-Z]\w+)\".*$/\1 \tDefault: '"$default"'/p' >> "${out_dir}/locked-featuregates-${version}.txt"
    done < "${out_dir}/locked_features.txt"
    rm -f "${out_dir}/kube_features.go" "${out_dir}/constants.txt" "${out_dir}/locked_features.txt"
  done

  sort -u -o "${out_dir}/featuregates-${version}.txt" "${out_dir}/featuregates-${version}.txt"
  sort -u -o "${out_dir}/locked-featuregates-${version}.txt" "${out_dir}/locked-featuregates-${version}.txt"
done

echo "Feature gates added in $2 compared to $1:"
diff "${out_dir}/featuregates-$1.txt" "${out_dir}/featuregates-$2.txt" | grep '>' | awk '{print $2}'
echo
echo "Feature gates removed in $2 compared to $1:"
diff "${out_dir}/featuregates-$1.txt" "${out_dir}/featuregates-$2.txt" | grep '<' | awk '{print $2}'
echo
echo "Feature gates locked to default in $2 compared to $1:"
diff "${out_dir}/locked-featuregates-$1.txt" "${out_dir}/locked-featuregates-$2.txt" | grep '>' | cut -c 2- | column -t
echo
