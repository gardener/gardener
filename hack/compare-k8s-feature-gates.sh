#!/bin/bash
#
# Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -e

usage() {
  echo "Usage:"
  echo "> compare-k8s-feature-gates.sh [ -h | <old version> <new version> ]"

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

out_dir=dev/temp
mkdir -p "${out_dir}"

for version in "${versions[@]}"; do
  rm -f "${out_dir}/featuregates-${version}.txt"
  touch "${out_dir}/featuregates-${version}.txt"

  for file in "${files[@]}"; do
    { wget -q -O - "https://raw.githubusercontent.com/kubernetes/kubernetes/release-${version}/${file}" || echo; } > "${out_dir}/kube_features.go"
    grep -E '{Default: .*, PreRelease: .*},' "${out_dir}/kube_features.go" | awk '{print $1}' | { grep -Eo '[A-Z]\w+' || true; } > "${out_dir}/constants.txt"
    while read constant; do
      grep -E "${constant} featuregate.Feature = \".*\"" "${out_dir}/kube_features.go" | awk '{print $4}' | { grep -Eo '[A-Z]\w+' || true; } >> "${out_dir}/featuregates-${version}.txt"
    done < "${out_dir}/constants.txt"
    rm -f "${out_dir}/kube_features.go" "${out_dir}/constants.txt"
  done

  sort -u -o "${out_dir}/featuregates-${version}.txt" "${out_dir}/featuregates-${version}.txt"
done

echo "Feature gates added in $2 compared to $1:"
diff "${out_dir}/featuregates-$1.txt" "${out_dir}/featuregates-$2.txt" | grep '>' | awk '{print $2}'
echo
echo "Feature gates removed in $2 compared to $1:"
diff "${out_dir}/featuregates-$1.txt" "${out_dir}/featuregates-$2.txt" | grep '<' | awk '{print $2}'