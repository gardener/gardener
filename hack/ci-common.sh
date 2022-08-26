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

dump_logs() {
  cluster_name=${1}
  echo "> Collecting logs of kind cluster '${cluster_name}'"
  kind export logs "${ARTIFACTS:-}" --name "${cluster_name}" || true

  # dump logs from shoot machine pods (similar to `kind export logs`)
  while IFS= read -r namespace; do
    while IFS= read -r node; do
      echo "> Collecting logs of shoot cluster '$namespace', node '$node'"
      node_dir="${ARTIFACTS:-}/$namespace/$node"
      mkdir -p "$node_dir"

      # general stuff
      kubectl -n "$namespace" exec "$node" -- crictl images > "$node_dir/images.log" || true
      kubectl -n "$namespace" get pod "$node" --show-managed-fields -oyaml > "$node_dir/pod.yaml" || true

      # systemd units
      for unit in cloud-config-downloader kubelet containerd ; do
        kubectl -n "$namespace" exec "$node" -- journalctl --no-pager -u $unit.service > "$node_dir/$unit.log" || true
      done
      kubectl -n "$namespace" exec "$node" -- journalctl --no-pager > "$node_dir/journal.log" || true

      # container logs
      kubectl cp "$namespace/$node":/var/log "$node_dir" || true
    done < <(kubectl -n "$namespace" get po -l app=machine -oname | cut -d/ -f2)
  done < <(kubectl get ns -l gardener.cloud/role=shoot -oname | cut -d/ -f2)
}

clamp_mss_to_pmtu() {
  # https://github.com/kubernetes/test-infra/issues/23741
  if [[ "$OSTYPE" != "darwin"* ]]; then
    iptables -t mangle -A POSTROUTING -p tcp --tcp-flags SYN,RST SYN -j TCPMSS --clamp-mss-to-pmtu
  fi
}
