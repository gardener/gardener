#!/usr/bin/env bash
#
# Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -e

usage() {
  echo "Usage:"
  echo "> compute-k8s-controllers.sh [ -h | <old version> <new version> ]"
  echo
  echo ">> For example: compute-k8s-controllers.sh 1.26 1.27"

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

# Define the path map
declare -A path_map=(
  ["attachdetach"]="pkg/controller/volume/attachdetach/attach_detach_controller.go"
  ["bootstrapsigner"]="pkg/controller/bootstrap/bootstrapsigner.go"
  ["cloud-node-lifecycle"]="staging/src/k8s.io/cloud-provider/controllers/nodelifecycle/node_lifecycle_controller.go"
  ["clusterrole-aggregation"]="pkg/controller/clusterroleaggregation/clusterroleaggregation_controller.go"
  ["cronjob"]="pkg/controller/cronjob/cronjob_controllerv2.go"
  ["csrapproving"]="pkg/controller/certificates/approver/sarapprove.go"
  ["csrcleaner"]="pkg/controller/certificates/cleaner/cleaner.go"
  ["csrsigning"]="pkg/controller/certificates/signer/signer.go"
  ["daemonset"]="pkg/controller/daemon/daemon_controller.go"
  ["deployment"]="pkg/controller/deployment/deployment_controller.go"
  ["disruption"]="pkg/controller/disruption/disruption.go"
  ["endpoint"]="pkg/controller/endpoint/endpoints_controller.go"
  ["endpointslice"]="pkg/controller/endpointslice/endpointslice_controller.go"
  ["endpointslicemirroring"]="pkg/controller/endpointslicemirroring/endpointslicemirroring_controller.go"
  ["ephemeral-volume"]="pkg/controller/volume/ephemeral/controller.go"
  ["garbagecollector"]="pkg/controller/garbagecollector/garbagecollector.go"
  ["horizontalpodautoscaling"]="pkg/controller/podautoscaler/horizontal.go"
  ["job"]="pkg/controller/job/job_controller.go"
  ["legacy-service-account-token-cleaner"]="pkg/controller/serviceaccount/legacy_serviceaccount_token_cleaner.go"
  ["namespace"]="pkg/controller/namespace/namespace_controller.go"
  ["nodeipam"]="pkg/controller/nodeipam/node_ipam_controller.go"
  ["nodelifecycle"]="pkg/controller/nodelifecycle/node_lifecycle_controller.go"
  ["persistentvolume-binder"]="pkg/controller/volume/persistentvolume/pv_controller_base.go"
  ["persistentvolume-expander"]="pkg/controller/volume/expand/expand_controller.go"
  ["podgc"]="pkg/controller/podgc/gc_controller.go"
  ["pv-protection"]="pkg/controller/volume/pvprotection/pv_protection_controller.go"
  ["pvc-protection"]="pkg/controller/volume/pvcprotection/pvc_protection_controller.go"
  ["replicaset"]="pkg/controller/replicaset/replica_set.go"
  ["replicationcontroller"]="pkg/controller/replication/replication_controller.go"
  ["resource-claim-controller"]="pkg/controller/resourceclaim/controller.go"
  ["resourcequota"]="pkg/controller/resourcequota/resource_quota_controller.go"
  ["root-ca-cert-publisher"]="pkg/controller/certificates/rootcacertpublisher/publisher.go"
  ["route"]="staging/src/k8s.io/cloud-provider/controllers/route/route_controller.go"
  ["service"]="staging/src/k8s.io/cloud-provider/controllers/service/controller.go"
  ["serviceaccount"]="pkg/controller/serviceaccount/serviceaccounts_controller.go"
  ["serviceaccount-token"]="pkg/controller/serviceaccount/tokens_controller.go"
  ["statefulset"]="pkg/controller/statefulset/stateful_set.go"
  ["storage-version-gc"]="pkg/controller/storageversiongc/gc_controller.go"
  ["tokencleaner"]="pkg/controller/bootstrap/tokencleaner.go"
  ["ttl"]="pkg/controller/ttl/ttl_controller.go"
  ["ttl-after-finished"]="pkg/controller/ttlafterfinished/ttlafterfinished_controller.go"
)

for version in "${versions[@]}"; do
  rm -rf "${out_dir}/kubernetes-${version}"
  rm -f "${out_dir}/k8s-controllers-${version}.txt"

  git clone --depth 1 --filter=blob:none --sparse https://github.com/kubernetes/kubernetes -b "release-${version}" "${out_dir}/kubernetes-${version}"
  pushd "${out_dir}/kubernetes-${version}" > /dev/null
  git sparse-checkout set "cmd/kube-controller-manager" "pkg/controller" "staging/src/k8s.io/cloud-provider/controllers"
  popd > /dev/null

  if [ "$version" \< "1.26" ]; then
    names=$(grep -o 'controllers\["[^"]*' "${out_dir}/kubernetes-${version}/cmd/kube-controller-manager/app/controllermanager.go" | awk -F '"' '{print $2}')
    # This is a special controller which is not initialized normally, see https://github.com/kubernetes/kubernetes/blob/99151c39b7d4595632f7745ba7fb4dea4356f7fd/cmd/kube-controller-manager/app/controllermanager.go#L405-L411
    names+=" serviceaccount-token"
  elif [ "$version" \< "1.28" ]; then
    names=$(grep -o 'register("[^"]*' "${out_dir}/kubernetes-${version}/cmd/kube-controller-manager/app/controllermanager.go" | awk -F '"' '{print $2}')
    # This is a special controller which is not initialized normally, see https://github.com/kubernetes/kubernetes/blob/99151c39b7d4595632f7745ba7fb4dea4356f7fd/cmd/kube-controller-manager/app/controllermanager.go#L405-L411
    names+=" serviceaccount-token"
  else
    names=$(grep -E 'func KCMControllerAliases\(\) map\[string\]string \{' "${out_dir}/kubernetes-${version}/cmd/kube-controller-manager/names/controller_names.go" -A 200 | awk -F '[" :]+' '/^		\"[a-zA-Z0-9-]+\"/ {print $2}')
  fi

  for name in $names; do
    if [ ! "${path_map[$name]}" ]; then
      echo "No path mapping found for $name", The controller could have been removed or the path might have changed.
      echo "Please enhance the map in the script with the path for this controller."
      exit 1
    fi
  done

  unset api_group_controllers
  declare -A api_group_controllers

  for controller in $names; do
    file_path="${out_dir}/kubernetes-${version}/${path_map[$controller]}"
    if [ -f "$file_path" ]; then
      # Find lines containing 'k8s.io/api/' in the file, and extract content after 'k8s.io/api/' up to
      # the next double quote. This will be the API groups used for this controller.
      api_groups=$(grep -o 'k8s\.io/api/[^"]*' "$file_path" | awk -F 'k8s.io/api/' '{print $2}')
      for api_group in $api_groups
      do
          api_group=$(echo "$api_group" | tr -d '[:space:]' | sed 's/^core\/v1$/v1/' | sed 's/apiserverinternal/internal/')
          if [ -n "$api_group" ]; then
              api_group_controllers["$api_group"]+="$controller "
          fi
      done
    else
      echo "The file $file_path cannot be found. Please enhance the map in the script with the correct path for this controller."
      exit 1
    fi
  done

  for api_group in "${!api_group_controllers[@]}"; do
    echo "$api_group:$(echo "${api_group_controllers[$api_group]}" | tr ' ' '\n' | sort | tr '\n' ' ')" >> "${out_dir}/k8s-controllers-${version}.txt"
  done

  sort -o "${out_dir}/k8s-controllers-${version}.txt" "${out_dir}/k8s-controllers-${version}.txt"
done

echo
echo "kube-controller-manager controllers added in $2 compared to $1:"
IFS=$'\n' read -r -d '' -a added_lines < <(diff "${out_dir}/k8s-controllers-$1.txt" "${out_dir}/k8s-controllers-$2.txt" | grep '^>' | sed 's/^> //' && printf '\0')
for added_line in "${added_lines[@]}"; do
  api_group=$(echo "$added_line" | awk -F ': ' '{print $1}')
  controllers=$(echo "$added_line" | awk -F ': ' '{print $2}' | tr ' ' '\n')

  # Find the corresponding line in the other file
  old_line=$(grep "^$api_group: " "${out_dir}/k8s-controllers-$1.txt" | awk -F ': ' '{print $2}' | tr ' ' '\n')

  added_controllers=$(comm -23 <(echo "$controllers" | sort) <(echo "$old_line" | sort) | tr '\n' ' ')

  if [ -n "$added_controllers" ]; then
    echo "Added Controllers for API Group [$api_group]: $added_controllers"
  fi
done

echo
echo "kube-controller-manager controllers removed in $2 compared to $1:"
IFS=$'\n' read -r -d '' -a removed_lines < <(diff "${out_dir}/k8s-controllers-$1.txt" "${out_dir}/k8s-controllers-$2.txt" | grep '^<' | sed 's/^< //' && printf '\0')
for removed_line in "${removed_lines[@]}"; do
  api_group=$(echo "$removed_line" | awk -F ': ' '{print $1}')
  controllers=$(echo "$removed_line" | awk -F ': ' '{print $2}' | tr ' ' '\n')

  # Find the corresponding line in the other file
  new_line=$(grep "^$api_group: " "${out_dir}/k8s-controllers-$2.txt" | awk -F ': ' '{print $2}' | tr ' ' '\n')

  removed_controllers=$(comm -23 <(echo "$controllers" | sort) <(echo "$new_line" | sort) | tr '\n' ' ')

  if [ -n "$removed_controllers" ]; then
    echo "Removed Controllers for API Group [$api_group]: $removed_controllers"
  fi
done
