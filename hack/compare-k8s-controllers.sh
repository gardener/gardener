#!/usr/bin/env bash
#
# Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
  echo "> compare-k8s-controllers.sh [ -h | <old version> <new version> ]"
  echo
  echo ">> For example: compare-k8s-controllers.sh 1.22 1.23"

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

kcm_dir="cmd/kube-controller-manager/app"
ccm_dir="staging/src/k8s.io/cloud-provider/app"

for version in "${versions[@]}"; do
  rm -rf "${out_dir}/kubernetes-${version}"
  rm -f "${out_dir}/k8s-controllers-${version}.txt"

  git clone --depth 1 --filter=blob:none --sparse https://github.com/kubernetes/kubernetes -b "release-${version}" "${out_dir}/kubernetes-${version}"
  pushd "${out_dir}/kubernetes-${version}" > /dev/null
  git sparse-checkout set "$kcm_dir" "$ccm_dir"
  popd > /dev/null

  for dir in $kcm_dir $ccm_dir; do
    cat "${out_dir}/kubernetes-${version}/$dir/"*.go |\
      sed -rn "s/.*[Client|Config]OrDie\(\"(.*)\"\).*/\1/p" |\
      grep -vE "informers|discovery" |\
      sort |\
      uniq >> "${out_dir}/k8s-controllers-${version}.txt"
  done

  # Starting with release-v1.23 the names for the CCM controllers are maintained differently.
  cat "${out_dir}/kubernetes-${version}/$ccm_dir/controllermanager.go" |\
    sed -rn "s/.*ClientName: \"(.*)\",.*/\1/p" |\
    sort |\
    uniq >> "${out_dir}/k8s-controllers-${version}.txt"
done

echo
echo "kube-controller-manager controllers added in $2 compared to $1:"
diff "${out_dir}/k8s-controllers-$1.txt" "${out_dir}/k8s-controllers-$2.txt" | grep '>' | awk '{print $2}'
echo
echo "kube-controller-manager controllers removed in $2 compared to $1:"
diff "${out_dir}/k8s-controllers-$1.txt" "${out_dir}/k8s-controllers-$2.txt" | grep '<' | awk '{print $2}'
