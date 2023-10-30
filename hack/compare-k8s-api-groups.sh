#!/usr/bin/env bash
#
# Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
  echo "> compare-k8s-apigroups.sh [ -h | <old version> <new version> ]"
  echo
  echo ">> For example: compare-k8s-apigroups.sh 1.26 1.27"

  exit 0
}

if [ "$1" == "-h" ] || [ "$#" -ne 2 ]; then
  usage
fi

versions=("$1" "$2")

out_dir=$(mktemp -d)
function cleanup_output {
    rm -rf "$out_dir"
}
trap cleanup_output EXIT

base_dir="staging/src/k8s.io/client-go/informers"

for version in "${versions[@]}"; do
  rm -rf "${out_dir}/kubernetes-${version}"
  rm -f "${out_dir}/k8s-apiGVRs-${version}.txt"
  rm -f "${out_dir}/k8s-apiGVs-${version}.txt"

  git clone --depth 1 --filter=blob:none --sparse https://github.com/kubernetes/kubernetes -b "release-${version}" "${out_dir}/kubernetes-${version}"
  pushd "${out_dir}/kubernetes-${version}" > /dev/null
  git sparse-checkout set "$base_dir"
  popd > /dev/null

  groupVersions=()
  groupVersionResources=()
  g=""
  v=""

  while IFS= read -r line; do
    if [[ $line =~ Group=([a-zA-Z0-9.-]+),[[:space:]]*Version=([a-zA-Z0-9.-]+) ]]; then
      g="${BASH_REMATCH[1]}"
      v="${BASH_REMATCH[2]}"
      if [[ $g == "core" ]]; then
        groupVersions+=("$v")
      else
        groupVersions+=("$g/$v")
      fi
    elif [[ $line =~ WithResource\(\"(.*)\"\) ]]; then
      k="${BASH_REMATCH[1]}"
      if [[ $g == "core" ]]; then
        groupVersionResources+=("$v/$k")
      else
        groupVersionResources+=("$g/$v/$k")
      fi
    fi
  done < "${out_dir}/kubernetes-${version}/${base_dir}/generic.go"

  echo "${groupVersions[@]}" | tr ' ' '\n' | sort | uniq > "${out_dir}/k8s-apiGVs-${version}.txt"
  echo "${groupVersionResources[@]}" | tr ' ' '\n' | sort | uniq > "${out_dir}/k8s-apiGVRs-${version}.txt"
done

echo
echo "Kubernetes API group versions added in $2 compared to $1:"
diff "${out_dir}/k8s-apiGVs-$1.txt" "${out_dir}/k8s-apiGVs-$2.txt" | grep '>' | awk '{print $2}'
echo
echo "Kubernetes API GVRs added in $2 compared to $1:"
diff "${out_dir}/k8s-apiGVRs-$1.txt" "${out_dir}/k8s-apiGVRs-$2.txt" | grep '>' | awk '{print $2}'
echo
echo "Kubernetes API group versions removed in $2 compared to $1:"
diff "${out_dir}/k8s-apiGVs-$1.txt" "${out_dir}/k8s-apiGVs-$2.txt" | grep '<' | awk '{print $2}'
echo
echo "Kubernetes API GVRs removed in $2 compared to $1:"