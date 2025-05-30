#!/usr/bin/env bash
#
# SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

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
  ["persistentvolume-attach-detach-controller"]="pkg/controller/volume/attachdetach/attach_detach_controller.go"
  ["bootstrap-signer-controller"]="pkg/controller/bootstrap/bootstrapsigner.go"
  ["cloud-node-controller"]="staging/src/k8s.io/cloud-provider/controllers/node/node_controller.go"
  ["cloud-node-lifecycle-controller"]="staging/src/k8s.io/cloud-provider/controllers/nodelifecycle/node_lifecycle_controller.go"
  ["clusterrole-aggregation-controller"]="pkg/controller/clusterroleaggregation/clusterroleaggregation_controller.go"
  ["cronjob-controller"]="pkg/controller/cronjob/cronjob_controllerv2.go"
  ["certificatesigningrequest-approving-controller"]="pkg/controller/certificates/approver/sarapprove.go"
  ["certificatesigningrequest-cleaner-controller"]="pkg/controller/certificates/cleaner/cleaner.go"
  ["certificatesigningrequest-signing-controller"]="pkg/controller/certificates/signer/signer.go"
  ["daemonset-controller"]="pkg/controller/daemon/daemon_controller.go"
  ["deployment-controller"]="pkg/controller/deployment/deployment_controller.go"
  ["disruption-controller"]="pkg/controller/disruption/disruption.go"
  ["endpoints-controller"]="pkg/controller/endpoint/endpoints_controller.go"
  ["endpointslice-controller"]="pkg/controller/endpointslice/endpointslice_controller.go"
  ["endpointslice-mirroring-controller"]="pkg/controller/endpointslicemirroring/endpointslicemirroring_controller.go"
  ["ephemeral-volume-controller"]="pkg/controller/volume/ephemeral/controller.go"
  ["garbage-collector-controller"]="pkg/controller/garbagecollector/garbagecollector.go"
  ["horizontal-pod-autoscaler-controller"]="pkg/controller/podautoscaler/horizontal.go"
  ["job-controller"]="pkg/controller/job/job_controller.go"
  ["kube-apiserver-serving-clustertrustbundle-publisher-controller"]="pkg/controller/certificates/clustertrustbundlepublisher/publisher.go"
  ["legacy-serviceaccount-token-cleaner-controller"]="pkg/controller/serviceaccount/legacy_serviceaccount_token_cleaner.go"
  ["namespace-controller"]="pkg/controller/namespace/namespace_controller.go"
  ["node-ipam-controller"]="pkg/controller/nodeipam/node_ipam_controller.go"
  ["node-lifecycle-controller"]="pkg/controller/nodelifecycle/node_lifecycle_controller.go"
  ["persistentvolume-binder-controller"]="pkg/controller/volume/persistentvolume/pv_controller_base.go"
  ["persistentvolume-expander-controller"]="pkg/controller/volume/expand/expand_controller.go"
  ["pod-garbage-collector-controller"]="pkg/controller/podgc/gc_controller.go"
  ["persistentvolume-protection-controller"]="pkg/controller/volume/pvprotection/pv_protection_controller.go"
  ["persistentvolumeclaim-protection-controller"]="pkg/controller/volume/pvcprotection/pvc_protection_controller.go"
  ["replicaset-controller"]="pkg/controller/replicaset/replica_set.go"
  ["replicationcontroller-controller"]="pkg/controller/replication/replication_controller.go"
  ["resourceclaim-controller"]="pkg/controller/resourceclaim/controller.go"
  ["resourcequota-controller"]="pkg/controller/resourcequota/resource_quota_controller.go"
  ["root-ca-certificate-publisher-controller"]="pkg/controller/certificates/rootcacertpublisher/publisher.go"
  ["node-route-controller"]="staging/src/k8s.io/cloud-provider/controllers/route/route_controller.go"
  ["selinux-warning-controller"]="pkg/controller/volume/selinuxwarning/selinux_warning_controller.go"
  ["service-lb-controller"]="staging/src/k8s.io/cloud-provider/controllers/service/controller.go"
  ["service-cidr-controller"]="pkg/controller/servicecidrs/servicecidrs_controller.go"
  ["serviceaccount-controller"]="pkg/controller/serviceaccount/serviceaccounts_controller.go"
  ["serviceaccount-token-controller"]="pkg/controller/serviceaccount/tokens_controller.go"
  ["statefulset-controller"]="pkg/controller/statefulset/stateful_set.go"
  ["storageversion-garbage-collector-controller"]="pkg/controller/storageversiongc/gc_controller.go"
  ["storage-version-migrator-controller"]="pkg/controller/storageversionmigrator/storageversionmigrator.go"
  ["taint-eviction-controller"]="pkg/controller/tainteviction/taint_eviction.go"
  ["token-cleaner-controller"]="pkg/controller/bootstrap/tokencleaner.go"
  ["ttl-controller"]="pkg/controller/ttl/ttl_controller.go"
  ["ttl-after-finished-controller"]="pkg/controller/ttlafterfinished/ttlafterfinished_controller.go"
  ["validatingadmissionpolicy-status-controller"]="pkg/controller/validatingadmissionpolicystatus/controller.go"
  ["volumeattributesclass-protection-controller"]="pkg/controller/volume/vacprotection/vac_protection_controller.go"
)

for version in "${versions[@]}"; do
  if [ "$version" \< "1.28" ]; then
    echo "Versions less than 1.28 are not supported."
    exit 1
  fi

  rm -rf "${out_dir}/kubernetes-${version}"
  rm -f "${out_dir}/k8s-controllers-${version}.txt"

  git clone --depth 1 --filter=blob:none --sparse https://github.com/kubernetes/kubernetes -b "release-${version}" "${out_dir}/kubernetes-${version}"
  pushd "${out_dir}/kubernetes-${version}" > /dev/null
  git sparse-checkout set "cmd/kube-controller-manager" "pkg/controller" "staging/src/k8s.io/cloud-provider"
  popd > /dev/null

  names=$(awk '/const \(/,/\)/' "${out_dir}/kubernetes-${version}/cmd/kube-controller-manager/names/controller_names.go" "${out_dir}/kubernetes-${version}/staging/src/k8s.io/cloud-provider/names/controller_names.go" | sed -n 's/.*"\(.*\)".*/\1/p')
  
  for name in $names; do
    if [ ! "${path_map[$name]}" ]; then
      echo
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
