#!/usr/bin/env bash
#
# SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

set -o errexit

export_artifacts_host_services() {
  mkdir -p "${ARTIFACTS:-}"

  echo "> Exporting logs of host services"
  cp /var/log/{docker,dnsmasq}.log "${ARTIFACTS:-}/" || true
}

export_artifacts_infra() {
  mkdir -p "${ARTIFACTS:-}"

  echo "> Exporting logs of local infrastructure managed via docker compose"
  mkdir -p "${ARTIFACTS:-}/infra"
  # In Docker, container logs are not stored in /var/log but in /var/lib/docker/containers.
  # However, this directory also holds a lot of other files, so we use `docker compose logs` to get only the logs of the
  # relevant containers to avoid exporting unnecessary files.
  for service in $(yq '.services | keys() | .[]' ./dev-setup/infra/docker-compose.yaml); do
    docker compose -f ./dev-setup/infra/docker-compose.yaml logs --no-log-prefix "$service" > "${ARTIFACTS:-}/infra/$service.log" || true
  done
}

export_artifacts_load_balancers() {
  mkdir -p "${ARTIFACTS:-}"

  echo "> Exporting state and logs of local load balancers"
  mkdir -p "${ARTIFACTS:-}/load-balancers"

  for container in $(docker container ls -a --format '{{.Names}}' --filter network=kind --filter label=gardener.cloud/role=loadbalancer); do
    docker container inspect "$container" > "${ARTIFACTS:-}/load-balancers/$container.json" || true
    docker container logs "$container" > "${ARTIFACTS:-}/load-balancers/$container.log" 2>&1 || true
  done
}

export_artifacts() {
  cluster_name="${1}"
  echo "> Exporting logs of kind cluster '$cluster_name'"
  kind export logs "${ARTIFACTS:-}/$cluster_name" --name "$cluster_name" || true

  export_artifacts_for_cluster "$cluster_name"
}

export_node_artifacts() {
  local node_dir="$1"
  shift
  local exec_cmd=("$@")

  mkdir -p "$node_dir"

  # general stuff
  "${exec_cmd[@]}" crictl images >"$node_dir/images.log" || true

  # relevant systemd units
  for unit in gardener-node-agent kubelet containerd containerd-configuration-local-setup ssh; do
    "${exec_cmd[@]}" journalctl --no-pager -u $unit.service >"$node_dir/$unit.log" || true
  done
  "${exec_cmd[@]}" journalctl --no-pager >"$node_dir/journal.log" || true
}

export_artifacts_for_cluster() {
  cluster_name="${1}"

  echo "> Exporting events of cluster '$cluster_name' > '$ARTIFACTS/$cluster_name'"
  export_events_for_cluster "$ARTIFACTS/$cluster_name"

  # dump logs from shoot machine pods (similar to `kind export logs`)
  while IFS= read -r namespace; do
    while IFS= read -r node; do
      echo "> Exporting logs of shoot cluster '$namespace', node '$node'"
      node_dir="${ARTIFACTS:-}/$namespace/$node"

      export_node_artifacts "$node_dir" kubectl -n "$namespace" exec "$node" --
      kubectl -n "$namespace" get pod "$node" --show-managed-fields -oyaml >"$node_dir/pod.yaml" || true

      # container logs
      kubectl cp "$namespace/$node":/var/log "$node_dir" || true
    done < <(kubectl -n "$namespace" get po -l 'app in (machine,bastion)' -oname | cut -d/ -f2)
  done < <(kubectl get ns -l gardener.cloud/role=shoot -oname | cut -d/ -f2)
}

export_artifacts_gind() {
  echo "> Exporting logs and state of gind containers"
  for container in $(yq '.services | keys() | .[]' ./dev-setup/gind/docker-compose.yaml); do
    container_name="gind-$container"
    node_dir="${ARTIFACTS:-}/gind/$container_name"
    mkdir -p "$node_dir"

    docker compose -f ./dev-setup/gind/docker-compose.yaml logs --no-log-prefix "$container" > "${ARTIFACTS:-}/gind/$container.log" || true
    docker container inspect "$container_name" > "${ARTIFACTS:-}/gind/$container_name.json" || true

    # Only export node-level artifacts for machine containers (not load balancers).
    if [[ "$container" == machine-* ]]; then
      export_node_artifacts "$node_dir" docker exec "$container_name"

      # container logs
      mkdir -p "$node_dir/var-log"
      docker cp "$container_name":/var/log/. "$node_dir/var-log" || true
    fi
  done
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
