#!/usr/bin/env bash
#
# SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

set -o errexit

export_artifacts() {
  cluster_name="${1}"
  echo "> Exporting logs of kind cluster '$cluster_name'"
  kind export logs "${ARTIFACTS:-}/$cluster_name" --name "$cluster_name" || true

  echo "> Exporting events of kind cluster '$cluster_name' > '$ARTIFACTS/$cluster_name'"
  export_events_for_cluster "$ARTIFACTS/$cluster_name"

  export_resource_yamls_for seeds shoots etcds leases
  export_events_for_shoots

  # dump logs from shoot machine pods (similar to `kind export logs`)
  while IFS= read -r namespace; do
    while IFS= read -r node; do
      echo "> Exporting logs of shoot cluster '$namespace', node '$node'"
      node_dir="${ARTIFACTS:-}/$namespace/$node"
      mkdir -p "$node_dir"

      # general stuff
      kubectl -n "$namespace" exec "$node" -- crictl images >"$node_dir/images.log" || true
      kubectl -n "$namespace" get pod "$node" --show-managed-fields -oyaml >"$node_dir/pod.yaml" || true

      # relevant systemd units
      for unit in gardener-node-agent kubelet containerd containerd-configuration-local-setup; do
        kubectl -n "$namespace" exec "$node" -- journalctl --no-pager -u $unit.service >"$node_dir/$unit.log" || true
      done
      kubectl -n "$namespace" exec "$node" -- journalctl --no-pager >"$node_dir/journal.log" || true

      # container logs
      kubectl cp "$namespace/$node":/var/log "$node_dir" || true
    done < <(kubectl -n "$namespace" get po -l app=machine -oname | cut -d/ -f2)
  done < <(kubectl get ns -l gardener.cloud/role=shoot -oname | cut -d/ -f2)

  echo "> Exporting /etc/hosts"
  cp /etc/hosts $ARTIFACTS/$cluster_name/hosts
}

export_resource_yamls_for() {
  mkdir -p $ARTIFACTS
  # Loop over the resource types
  for resource_type in "$@"; do
    echo "> Exporting Resource '$resource_type' yaml > $ARTIFACTS/$resource_type.yaml"
    echo -e "---\n# cluster name: '${cluster_name:-}'" >> "$ARTIFACTS/$resource_type.yaml"
    kubectl get "$resource_type" -A -o yaml >> "$ARTIFACTS/$resource_type.yaml" || true
  done
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

    KUBECONFIG="$shoot_kubeconfig" export_events_for_cluster "$ARTIFACTS/$shoot_id"
    rm -f "$shoot_kubeconfig"
  done < <(kubectl get shoot -A -o=custom-columns=namespace:metadata.namespace,name:metadata.name,id:status.technicalID --no-headers)
}

export_events_for_cluster() {
  local dir="$1/events"
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

# If running in prow, we need to ensure that garden.local.gardener.cloud resolves to localhost
ensure_glgc_resolves_to_localhost() {
  if [ -n "${CI:-}" -a -n "${ARTIFACTS:-}" ]; then
    echo "> Adding garden.local.gardener.cloud to /etc/hosts..."
    printf "\n127.0.0.1 garden.local.gardener.cloud\n" >> /etc/hosts
    printf "\n::1 garden.local.gardener.cloud\n" >> /etc/hosts
    echo "> Content of '/etc/hosts' after adding garden.local.gardener.cloud:\n$(cat /etc/hosts)"
  fi
}
