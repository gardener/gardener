#!/usr/bin/env bash
# SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

set -o errexit
set -o pipefail

COMMAND="${1:-up}"
VALID_COMMANDS=("up" "down")

case "$COMMAND" in
  up)
    # setup_containerd_registry_mirrors sets up all containerd registry mirrors.
    # Resources:
    # - https://github.com/containerd/containerd/blob/main/docs/hosts.md
    # - https://kind.sigs.k8s.io/docs/user/local-registry/
    setup_containerd_registry_mirrors() {
      NODES=("$@")

      for NODE in "${NODES[@]}"; do
        # For the local registry, we don't need a mirror config for switching the URL, but only for configuring containerd
        # to use HTTP instead of HTTPS. Probably, we could use the insecure registries config for this. However, configuring
        # mirrors is supported by gardener-node-agent via the OSC, so we use the same approach everywhere.
        setup_containerd_registry_mirror "$NODE" "registry.local.gardener.cloud:5001" "http://registry.local.gardener.cloud:5001" "http://registry.local.gardener.cloud:5001"
        setup_containerd_registry_mirror "$NODE" "gcr.io" "https://gcr.io" "http://gcr.registry-cache.local.gardener.cloud:5001"
        setup_containerd_registry_mirror "$NODE" "registry.k8s.io" "https://registry.k8s.io" "http://k8s.registry-cache.local.gardener.cloud:5001"
        setup_containerd_registry_mirror "$NODE" "quay.io" "https://quay.io" "http://quay.registry-cache.local.gardener.cloud:5001"
        setup_containerd_registry_mirror "$NODE" "europe-docker.pkg.dev" "https://europe-docker.pkg.dev" "http://europe-docker-pkg-dev.registry-cache.local.gardener.cloud:5001"

        echo "[${NODE}] Restarting containerd after setting up containerd registry mirrors."
        docker exec "${NODE}" systemctl restart containerd
      done
    }

    # setup_containerd_registry_mirror sets up a given contained registry mirror.
    setup_containerd_registry_mirror() {
      NODE=$1
      UPSTREAM_HOST=$2
      UPSTREAM_SERVER=$3
      MIRROR_HOST=$4

      echo "[${NODE}] Setting up containerd registry mirror for host ${UPSTREAM_HOST}.";
      REGISTRY_DIR="/etc/containerd/certs.d/${UPSTREAM_HOST}"
      docker exec "${NODE}" mkdir -p "${REGISTRY_DIR}"
      cat <<EOF | docker exec -i "${NODE}" cp /dev/stdin "${REGISTRY_DIR}/hosts.toml"
server = "${UPSTREAM_SERVER}"

[host."${MIRROR_HOST}"]
  capabilities = ["pull", "resolve"]
EOF
    }

    # The default StorageClass which comes with `kind' is configured to use
    # rancher.io/local-path (see [1]) provisioner, which defaults to `hostPath'
    # volume (see [2]).  However, `hostPath' does not expose any metrics via
    # kubelet, while `local' (see [3]) does. On the other hand `kind' does not
    # expose any mechanism for configuring the StorageClass it comes with (see [4]).
    #
    # This function annotates the default StorageClass with `defaultVolumeType: local',
    # so that we can later scrape the various `kubelet_volume_stats_*' metrics
    # exposed by kubelet (see [5]).
    #
    # References:
    #
    # [1]: https://github.com/rancher/local-path-provisioner
    # [2]: https://kubernetes.io/docs/concepts/storage/volumes/#hostpath
    # [3]: https://kubernetes.io/docs/concepts/storage/volumes/#local
    # [4]: https://github.com/kubernetes-sigs/kind/blob/main/pkg/cluster/internal/create/actions/installstorage/storage.go
    # [5]: https://kubernetes.io/docs/reference/instrumentation/metrics/
    setup_kind_sc_default_volume_type() {
      echo "Configuring default StorageClass for kind cluster ..."
      kubectl annotate storageclass standard defaultVolumeType=local
    }

    # The rancher.io/local-path provisioner at the moment does not support volume
    # resizing (see [1]). There is an open PR, which is scheduled for the next
    # release around May, 2024 (see [2]). Until [2] is merged we will use a custom
    # local-path provisioner with support for volume resizing.
    #
    # This function should be called after setting up the containerd registries on
    # the kind nodes.
    #
    # References:
    #
    # [1]: https://github.com/rancher/local-path-provisioner
    # [2]: https://github.com/rancher/local-path-provisioner/pull/350
    #
    # TODO(dnaeon): remove this once we have [2] merged into upstream
    setup_kind_with_lpp_resize_support() {
      if [ "${WITH_LPP_RESIZE_SUPPORT}" != "true" ]; then
        return
      fi

      echo "Configuring kind local-path provisioner with volume resize support ..."

      # First configure allowVolumeExpansion on the default StorageClass
      kubectl patch storageclass standard --patch '{"allowVolumeExpansion": true}'

      # Apply the latest manifests and use our own image
      local _image="ghcr.io/ialidzhikov/local-path-provisioner:feature-external-resizer-c0c1c13"
      local _lpp_repo="https://github.com/marjus45/local-path-provisioner"
      local _lpp_branch="feature-external-resizer"
      local _timeout="90"

      kustomize build "${_lpp_repo}/deploy/?ref=${_lpp_branch}&timeout=${_timeout}" | \
        sed -e "s|image: rancher/local-path-provisioner:master-head|image: ${_image}|g" | \
        kubectl apply -f -

      # The default manifests from rancher/local-path come with another
      # StorageClass, which we don't need, so make sure to remove it.
      kubectl delete --ignore-not-found=true storageclass local-path
    }

    docker_socket() {
      # Find the Docker socket path from the current Docker context. This also respects the DOCKER_HOST environment variable
      # if set. This should support setups with non-standard Docker socket paths, e.g., when using Lima or Colima as Docker
      # backend on macOS, without requiring users to explicitly configure the socket path.
      socket=$(docker context inspect "$(docker context show)" -f json | jq -r '.[].Endpoints.docker.Host' | sed 's|unix://||')

      # If the socket path contains .lima or .colima, we assume that Lima/Colima is used as Docker backend.
      # In this case, the socket on the host is a forward to /var/run/docker.sock in the guest VM.
      # Instead of mounting the socket on the host back into the VM, we directly use /var/run/docker.sock in the VM.
      if [[ "$socket" == *"/.lima/"* || "$socket" == *"/.colima/"* ]]; then
        socket="/var/run/docker.sock"
      fi

      echo "$socket"
    }

    "$(dirname "$0")/infra.sh" up

    kustomize build "$(dirname "$0")/kind/cluster/overlays/${KUSTOMIZE_OVERLAY}-${IPFAMILY}" | \
      yq 'del(.metadata)' | \
      sed "s|\${DOCKER_SOCKET}|$(docker_socket)|g" | \
      kind create cluster \
        --name "$CLUSTER_NAME" \
        --config /dev/stdin

    nodes=$(kubectl get nodes -o jsonpath='{.items[*].metadata.name}')

    # Configure the default StorageClass in the kind cluster
    setup_kind_sc_default_volume_type

    # adjust Kind's CRI default OCI runtime spec for new containers to include the cgroup namespace
    # this is required for nesting kubelets on cgroupsv2, as the kindest-node entrypoint script assumes an existing cgroupns when the host kernel uses cgroupsv2
    # See containerd CRI: https://github.com/containerd/containerd/commit/687469d3cee18bf0e12defa5c6d0c7b9139a2dbd
    if [ -f "/sys/fs/cgroup/cgroup.controllers" ] || [ "$(uname -s)" == "Darwin" ]; then
      echo "Host uses cgroupsv2"
      cat << 'EOF' > "$(dirname "$0")/../dev/adjust_cri_base.sh"
#!/bin/bash
if [ -f /etc/containerd/cri-base.json ]; then
  key=$(cat /etc/containerd/cri-base.json | jq '.linux.namespaces | map(select(.type == "cgroup"))[0]')
  if [ "$key" = "null" ]; then
      echo "Adjusting kind node /etc/containerd/cri-base.json to create containers with a cgroup namespace";
      cat /etc/containerd/cri-base.json | jq '.linux.namespaces += [{
          "type": "cgroup"
      }]' > /etc/containerd/cri-base.tmp.json && cp /etc/containerd/cri-base.tmp.json /etc/containerd/cri-base.json
    else
      echo "cgroup namespace already configured for kind node";
  fi
else
    echo "cannot configure cgroup namespace for kind containers: /etc/containerd/cri-base.json not found in kind container"
fi
EOF

      for node_name in $nodes; do
        echo "Adjusting containerd config for kind node $node_name"

        # copy script to the kind's docker container and execute it
        docker cp "$(dirname "$0")/../dev/adjust_cri_base.sh" "$node_name":/etc/containerd/adjust_cri_base.sh
        docker exec "$node_name" bash -c "chmod +x /etc/containerd/adjust_cri_base.sh && /etc/containerd/adjust_cri_base.sh && systemctl restart containerd"
      done
    fi

    for node in $nodes; do
      # workaround https://kind.sigs.k8s.io/docs/user/known-issues/#pod-errors-due-to-too-many-open-files
      docker exec "$node" sh -c "sysctl fs.inotify.max_user_instances=8192"
    done

    # copy kind cluster kubeconfig to additional directories where needed
    for path_kubeconfig in "${KUBECONFIG_SEED_CLUSTER:-}" "${KUBECONFIG_SEED2_CLUSTER:-}" "${KUBECONFIG_SEED_SECRET_PATH:-}"; do
      [[ -z "$path_kubeconfig" ]] && continue

      if [[ "$(realpath "$KUBECONFIG")" != "$(realpath "$path_kubeconfig")" ]]; then
        cp "$KUBECONFIG" "$path_kubeconfig"
      fi
      # Replace 127.0.0.1 in the kind cluster kubeconfig with  the container name of the Docker container for the second
      # kind cluster. This will be referenced in the `Gardenlet` resource (which is meant to register the second kind
      # cluster as Seed). `gardener-operator` will read this kubeconfig when attempting to deploy `gardenlet` into it.
      # Since `gardener-operator` runs in a pod, 127.0.0.1 would not work to communicate with the second kind cluster - but
      # the Docker container name is resolvable.
      if [[ "$path_kubeconfig" == *"seed-local2"* ]]; then
        sed "s/127\.0\.0\.1:[0-9]\+/gardener-local2-control-plane:6443/g" "$path_kubeconfig" > "${path_kubeconfig}-gardener-operator"
      fi

      # Prepare a kubeconfig that can be used by provider-local as the provider credentials to talk to the kind cluster
      # from within the kind cluster and also from within a self-hosted shoot.
      # See docs/extensions/provider-local.md#credentials.
      if [[ "$CLUSTER_NAME" == "gardener-local" ]] ; then
        sed "s/127\.0\.0\.1:[0-9]\+/$CLUSTER_NAME-control-plane:6443/g" "$path_kubeconfig" > "$(dirname "$0")/gardenconfig/components/credentials/secret-project-garden-with-kind-kubeconfig/kubeconfig"
      fi
    done

    kubectl apply -k "$(dirname "$0")/kind/calico/overlays/$IPFAMILY" --server-side
    kubectl apply -k "$(dirname "$0")/kind/metrics-server"            --server-side
    kubectl apply -k "$(dirname "$0")/kind/node-status-capacity"      --server-side

    setup_containerd_registry_mirrors $nodes
    setup_kind_with_lpp_resize_support

    for node in $(kubectl get nodes -l node-role.kubernetes.io/control-plane -o jsonpath='{.items[*].metadata.name}'); do
      kubectl taint node "$node" node-role.kubernetes.io/control-plane:NoSchedule- || true
    done

    # Allow multiple shoot worker nodes with calico as shoot CNI: As we run overlay in overlay ip-in-ip needs to be allowed in the workload.
    # Unfortunately, the felix configuration is created on the fly by calico. Hence, we need to poll until kubectl wait for new resources
    # (https://github.com/kubernetes/kubernetes/issues/83242) is fixed. (10 minutes should be enough for the felix configuration to be created.)
    echo "Waiting for FelixConfiguration to be created..."
    felix_config_found=0
    max_retries=600
    for ((i = 0; i < max_retries; i++)); do
      if kubectl get felixconfiguration default > /dev/null 2>&1; then
        if kubectl patch felixconfiguration default --type merge --patch '{"spec":{"allowIPIPPacketsFromWorkloads":true}}' > /dev/null 2>&1; then
          echo "FelixConfiguration 'default' successfully updated."
          felix_config_found=1
          break
        fi
      fi
      sleep 1
    done
    if [ $felix_config_found -eq 0 ]; then
      echo "Error: FelixConfiguration 'default' not found or patch failed after $max_retries attempts."
      exit 1
    fi

    # Auto approve Kubelet Serving Certificate Signing Requests: https://kubernetes.io/docs/tasks/administer-cluster/kubeadm/kubeadm-certs/#kubelet-serving-certs
    # such that the Kubelet in the KinD cluster uses a certificate signed by the cluster's CA: 'kubernetes'
    # instead of a self-signed certificate generated by the Kubelet itself.
    # The CSR is created with some delay, so for each node, wait for the CSR to be created.
    # There can be multiple CSRs for a node, so approve all of them.
    echo "Approving Kubelet Serving Certificate Signing Requests..."
    for node in $nodes; do
      max_retries=600
      for ((i = 0; i < max_retries; i++)); do
        csr_names=$(kubectl get csr -o json | jq -r --arg node "$node" '
          .items[] | select(.status=={} and
                            .spec.signerName == "kubernetes.io/kubelet-serving" and
                            .spec.username == "system:node:"+$node) | .metadata.name')
        if [ -n "$csr_names" ]; then
          for csr_name in $csr_names; do
            kubectl certificate approve "$csr_name"
          done
          break
        fi
        sleep 1
      done
    done
    echo "Kubelet Serving Certificate Signing Requests approved."

    # Automatically start cloud-provider-local on kind cluster
    make cloud-provider-local-up
    ;;

  down)
    kind delete cluster --name "$CLUSTER_NAME"

    # remove kind cluster kubeconfig to additional directories where needed
    for path_kubeconfig in "$KUBECONFIG" "${KUBECONFIG_SEED_CLUSTER:-}" "${KUBECONFIG_SEED2_CLUSTER:-}" "${KUBECONFIG_SEED_SECRET_PATH:-}"; do
      [[ -z "$path_kubeconfig" ]] && continue
      rm -f "$path_kubeconfig" "${path_kubeconfig}-gardener-operator"
    done

    if [[ "$CLUSTER_NAME" == "gardener-local2" ]]; then
      echo "Removing load balancer containers of cluster $CLUSTER_NAME"
      for container in $(docker container ls -aq --filter network=kind --filter label=gardener.cloud/role=loadbalancer --filter label=kubernetes.io/cluster="$CLUSTER_NAME"); do
        docker container rm -f "$container"
      done
      exit 0
    fi

    # Only stop the infra containers if deleting the "main" kind cluster.
    # When deleting the secondary cluster, we might still need infra containers (DNS/registry/etc.) for the other
    # cluster (see early exit above)
    "$(dirname "$0")/infra.sh" down
    ;;

  *)
    echo "Error: Invalid command '${COMMAND}'. Valid options are: ${VALID_COMMANDS[*]}." >&2
    exit 1
    ;;
esac
