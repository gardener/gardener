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

set -o errexit

VERSION=$(cat VERSION)

export_logs() {
  cluster_name="${1}"
  echo "> Exporting logs of kind cluster '$cluster_name'"
  kind export logs "${ARTIFACTS:-}" --name "$cluster_name" || true

  # dump logs from shoot machine pods (similar to `kind export logs`)
  while IFS= read -r namespace; do
    while IFS= read -r node; do
      echo "> Exporting logs of shoot cluster '$namespace', node '$node'"
      node_dir="${ARTIFACTS:-}/$namespace/$node"
      mkdir -p "$node_dir"

      # general stuff
      kubectl -n "$namespace" exec "$node" -- crictl images >"$node_dir/images.log" || true
      kubectl -n "$namespace" get pod "$node" --show-managed-fields -oyaml >"$node_dir/pod.yaml" || true

      # systemd units
      for unit in cloud-config-downloader kubelet containerd; do
        kubectl -n "$namespace" exec "$node" -- journalctl --no-pager -u $unit.service >"$node_dir/$unit.log" || true
      done
      kubectl -n "$namespace" exec "$node" -- journalctl --no-pager >"$node_dir/journal.log" || true

      # container logs
      kubectl cp "$namespace/$node":/var/log "$node_dir" || true
    done < <(kubectl -n "$namespace" get po -l app=machine -oname | cut -d/ -f2)
  done < <(kubectl get ns -l gardener.cloud/role=shoot -oname | cut -d/ -f2)
}

export_events_for_kind() {
  echo "> Exporting events of kind cluster '$1'"
  export_events_for_cluster "${1}-control-plane"
}

export_events_for_shoots() {
  while IFS= read -r shoot; do
    shoot_namespace="$(echo "$shoot" | awk '{print $1}')"
    shoot_name="$(echo "$shoot" | awk '{print $2}')"
    shoot_id="$(echo "$shoot" | awk '{print $3}')"

    echo "> Exporting events of shoot cluster '$shoot_id'"

    shoot_kubeconfig="$(mktemp)"
    kubectl create \
      -f <(echo '{"apiVersion": "authentication.gardener.cloud/v1alpha1","kind": "AdminKubeconfigRequest","spec": {"expirationSeconds": 1000}}') \
      --raw "/apis/core.gardener.cloud/v1beta1/namespaces/$shoot_namespace/shoots/$shoot_name/adminkubeconfig" |
      yq e ".status.kubeconfig" - |
      base64 -d \
        >"$shoot_kubeconfig"

    KUBECONFIG="$shoot_kubeconfig" export_events_for_cluster "$shoot_id"
    rm -f "$shoot_kubeconfig"
  done < <(kubectl get shoot -A -o=custom-columns=namespace:metadata.namespace,name:metadata.name,id:status.technicalID --no-headers)
}

export_events_for_cluster() {
  local dir="$ARTIFACTS/$1/events"
  mkdir -p "$dir"

  while IFS= read -r namespace; do
    kubectl -n "$namespace" get event --sort-by=lastTimestamp >"$dir/$namespace.log" 2>&1 || true
  done < <(kubectl get ns -oname | cut -d/ -f2)
}

clamp_mss_to_pmtu() {
  # https://github.com/kubernetes/test-infra/issues/23741
  if [[ "$OSTYPE" != "darwin"* ]]; then
    iptables -t mangle -A POSTROUTING -p tcp --tcp-flags SYN,RST SYN -j TCPMSS --clamp-mss-to-pmtu
  fi
}

# copy_kubeconfig_from_kubeconfig_env_var copies the kubeconfig to apporiate location based on kind setup
copy_kubeconfig_from_kubeconfig_env_var(){
  case "$SHOOT_FAILURE_TOLERANCE_TYPE" in
    node )
      cp $KUBECONFIG example/provider-local/seed-kind-ha-single-zone/base/kubeconfig
      cp $KUBECONFIG example/gardener-local/kind/ha-single-zone/kubeconfig
      ;;
    zone )
      cp $KUBECONFIG example/provider-local/seed-kind-ha-multi-zone/base/kubeconfig
      cp $KUBECONFIG example/gardener-local/kind/ha-multi-zone/kubeconfig
      ;;
    *)
      cp $KUBECONFIG example/provider-local/seed-kind/base/kubeconfig
      cp $KUBECONFIG example/gardener-local/kind/local/kubeconfig
      ;;
  esac
}

gardener_up(){
  case "$SHOOT_FAILURE_TOLERANCE_TYPE" in
    node )
      make gardener-ha-single-zone-up
      ;;
    zone )
      make gardener-ha-multi-zone-up
      ;;
    *)
      make gardener-up
      ;;
  esac
}

set_gardener_version_env_variables(){
if [[ -z $GARDENER_PREVIOUS_RELEASE ]]; then
  export GARDENER_PREVIOUS_RELEASE=$(curl -s https://api.github.com/repos/gardener/gardener/releases/latest | grep tag_name | cut -d '"' -f 4)
fi

if [[ -z $GARDENER_NEXT_RELEASE ]]; then
  export GARDENER_NEXT_RELEASE=$VERSION
fi
}

download_and_install_gardener_previous_release(){
  # download gardener previous release to perform gardener upgrade tests
  $(dirname "${0}")/download_gardener_source_code.sh --gardener-version $GARDENER_PREVIOUS_RELEASE --download-path $DOWNLOAD_PATH/gardener-upgrade/$GARDENER_PREVIOUS_RELEASE
  cd $DOWNLOAD_PATH/gardener-upgrade/$GARDENER_PREVIOUS_RELEASE/gardener
  copy_kubeconfig_from_kubeconfig_env_var
  echo "Installing gardener version '$GARDENER_PREVIOUS_RELEASE'"
  gardener_up
  cd -
}

# download_and_upgrade_gardener_next_release downloads and upgrades to GARDENER_NEXT_RELEASE release if GARDENER_NEXT_RELEASE is not same as version mentioned in VERSION file.
# if GARDENER_NEXT_RELEASE is same as version mentioned in VERSION file then it is considered as local release and install gardener from local repo. 
download_and_upgrade_gardener_next_release(){
  echo "Upgrading gardener version '$GARDENER_PREVIOUS_RELEASE' to '$GARDENER_NEXT_RELEASE'"
  if [[ -n $GARDENER_NEXT_RELEASE && $GARDENER_NEXT_RELEASE != $VERSION ]]; then
    # download gardener previous release to perform gardener upgrade tests
    $(dirname "${0}")/download_gardener_source_code.sh --gardener-version $GARDENER_NEXT_RELEASE --download-path $DOWNLOAD_PATH/gardener-upgrade/$GARDENER_NEXT_RELEASE
    cd $DOWNLOAD_PATH/gardener-upgrade/$GARDENER_NEXT_RELEASE/gardener
    copy_kubeconfig_from_kubeconfig_env_var
    gardener_up
    cd -
    wait_until_seed_gets_upgraded
    return
  fi
  
  gardener_up
  wait_until_seed_gets_upgraded
}

wait_until_seed_gets_upgraded(){
  seed_name=""
  case "$SHOOT_FAILURE_TOLERANCE_TYPE" in
    node )
      seed_name="local-ha-single-zone"
      ;;
    zone )
      seed_name="local-ha-multi-zone"
      ;;
    *)
      seed_name="local"
      ;;
  esac
  echo "Wait until seed gets upgraded from version '$GARDENER_PREVIOUS_RELEASE' to '$GARDENER_NEXT_RELEASE'"
  kubectl wait seed $seed_name --timeout=5m \
    --for=jsonpath='{.status.gardener.version}'=$GARDENER_NEXT_RELEASE \
    --for=condition=gardenletready --for=condition=extensionsready \
    --for=condition=bootstrapped 
}