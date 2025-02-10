#!/usr/bin/env bash
#
# SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

set -e

usage() {
  echo "Usage:"
  echo "> compare-k8s-controllers.sh [ -h | <old version> <new version> ]"
  echo
  echo ">> For example: compare-k8s-controllers.sh 1.26 1.27"

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

  for dir in $kcm_dir; do
    cat "${out_dir}/kubernetes-${version}/$dir/"*.go |\
      sed -rn "s/.*[Client|Config]OrDie\(\"(.*)\"\).*/\1/p" |\
      grep -vE "informers|discovery" >> "${out_dir}/k8s-controllers-${version}.txt.tmp"
  done

  cat "${out_dir}/kubernetes-${version}/$ccm_dir/controllermanager.go" |\
    sed -rn "s/.*ClientName: \"(.*)\",.*/\1/p" >> "${out_dir}/k8s-controllers-${version}.txt.tmp"

  sort "${out_dir}/k8s-controllers-${version}.txt.tmp" | uniq > "${out_dir}/k8s-controllers-${version}.txt"
done

echo
echo "kube-controller-manager controllers added in $2 compared to $1:"
diff "${out_dir}/k8s-controllers-$1.txt" "${out_dir}/k8s-controllers-$2.txt" | grep '>' | awk '{print $2}'
echo
echo "kube-controller-manager controllers removed in $2 compared to $1:"
diff "${out_dir}/k8s-controllers-$1.txt" "${out_dir}/k8s-controllers-$2.txt" | grep '<' | awk '{print $2}'
