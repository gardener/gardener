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
