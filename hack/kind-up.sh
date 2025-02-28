#!/usr/bin/env bash
# SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0


set -o errexit
set -o nounset
set -o pipefail

WITH_LPP_RESIZE_SUPPORT=${WITH_LPP_RESIZE_SUPPORT:-false}
REGISTRY_CACHE=${CI:-false}
CLUSTER_NAME=""
PATH_CLUSTER_VALUES=""
PATH_KUBECONFIG=""
DEPLOY_REGISTRY=true
MULTI_ZONAL=false
CHART=$(dirname "$0")/../example/gardener-local/kind/cluster
ADDITIONAL_ARGS=""
SUDO=""
if [[ "$(id -u)" != "0" ]]; then
  SUDO="sudo "
fi

parse_flags() {
  while test $# -gt 0; do
    case "$1" in
    --chart)
      shift; CHART="$1"
      ;;
    --cluster-name)
      shift; CLUSTER_NAME="$1"
      ;;
    --path-cluster-values)
      shift; PATH_CLUSTER_VALUES="$1"
      ;;
    --path-kubeconfig)
      shift; PATH_KUBECONFIG="$1"
      ;;
    --skip-registry)
      DEPLOY_REGISTRY=false
      ;;
    --multi-zonal)
      MULTI_ZONAL=true
      ;;
    --with-lpp-resize-support)
      shift
      WITH_LPP_RESIZE_SUPPORT="${1}"
      ;;
    esac

    shift
  done
}

check_local_dns_records() {
  local glgc_ip_address
  glgc_ip_address=""

  if [[ "$OSTYPE" == "darwin"* ]]; then
    # Suppress exit code using "|| true"
    glgc_ip_address=$(dscacheutil -q host -a name garden.local.gardener.cloud | grep "ip_address" | head -n 1| cut -d' ' -f2 || true)
  elif [[ "$OSTYPE" == "linux-gnu"* ]]; then
    # Suppress exit code using "|| true"
    glgc_ip_address="$(getent ahosts garden.local.gardener.cloud || true)"
  else
    echo "Warning: Unknown OS. Make sure garden.local.gardener.cloud resolves to 127.0.0.1"
    return 0
  fi
    
  if ! echo "$glgc_ip_address" | grep -q "127.0.0.1" ; then
      echo "Error: garden.local.gardener.cloud does not resolve to 127.0.0.1. Please add a line for it in /etc/hosts"
      echo "Command output: $glgc_ip_address"
      echo "Content of '/etc/hosts':\n$(cat /etc/hosts)"
      exit 1
  fi
}

# setup_kind_network is similar to kind's network creation logic, ref https://github.com/kubernetes-sigs/kind/blob/23d2ac0e9c41028fa252dd1340411d70d46e2fd4/pkg/cluster/internal/providers/docker/network.go#L50
# In addition to kind's logic, we ensure stable CIDRs that we can rely on in our local setup manifests and code.
setup_kind_network() {
  # check if network already exists
  local existing_network_id
  existing_network_id="$(docker network list --filter=name=^kind$ --format='{{.ID}}')"

  if [ -n "$existing_network_id" ] ; then
    # ensure the network is configured correctly
    local network network_options network_ipam expected_network_ipam
    network="$(docker network inspect $existing_network_id | yq '.[]')"
    network_options="$(echo "$network" | yq '.EnableIPv6 + "," + .Options["com.docker.network.bridge.enable_ip_masquerade"]')"
    network_ipam="$(echo "$network" | yq '.IPAM.Config' -o=json -I=0)"
    expected_network_ipam='[{"Subnet":"172.18.0.0/16","Gateway":"172.18.0.1"},{"Subnet":"fd00:10::/64","Gateway":"fd00:10::1"}]'

    if [ "$network_options" = 'true,true' ] && [ "$network_ipam" = "$expected_network_ipam" ] ; then
      # kind network is already configured correctly, nothing to do
      return 0
    else
      echo "kind network is not configured correctly for local gardener setup, recreating network with correct configuration..."
      docker network rm $existing_network_id
    fi
  fi

  # (re-)create kind network with expected settings
  docker network create kind --driver=bridge \
    --subnet 172.18.0.0/16 --gateway 172.18.0.1 \
    --ipv6 --subnet fd00:10::/64 --gateway fd00:10::1 \
    --opt com.docker.network.bridge.enable_ip_masquerade=true
}

setup_loopback_device() {
  LOOPBACK_IP_ADDRESSES=$1
  if ! command -v ip &>/dev/null; then
    if [[ "$OSTYPE" == "darwin"* ]]; then
      echo "'ip' command not found. Please install 'ip' command, refer https://github.com/gardener/gardener/blob/master/docs/development/local_setup.md#installing-iproute2" 1>&2
      exit 1
    fi
    echo "Skipping loopback device setup because 'ip' command is not available..."
    return
  fi
  LOOPBACK_DEVICE=$(ip address | grep LOOPBACK | sed "s/^[0-9]\+: //g" | awk '{print $1}' | sed "s/:$//g")
  echo "Checking loopback device ${LOOPBACK_DEVICE}..."
  for address in "${LOOPBACK_IP_ADDRESSES[@]}"; do
    if ip address show dev ${LOOPBACK_DEVICE} | grep -q $address/; then
      echo "IP address $address already assigned to ${LOOPBACK_DEVICE}."
    else
      echo "Adding IP address $address to ${LOOPBACK_DEVICE}..."
      ${SUDO}ip address add "$address" dev "${LOOPBACK_DEVICE}"
    fi
  done
  echo "Setting up loopback device ${LOOPBACK_DEVICE} completed."
}

# setup_containerd_registry_mirrors sets up all containerd registry mirrors.
# Resources:
# - https://github.com/containerd/containerd/blob/main/docs/hosts.md
# - https://kind.sigs.k8s.io/docs/user/local-registry/
setup_containerd_registry_mirrors() {
  NODES=("$@")
  REGISTRY_HOSTNAME="garden.local.gardener.cloud"

  for NODE in "${NODES[@]}"; do
    setup_containerd_registry_mirror $NODE "localhost:5001" "http://localhost:5001" "http://${REGISTRY_HOSTNAME}:5001"
    setup_containerd_registry_mirror $NODE "gcr.io" "https://gcr.io" "http://${REGISTRY_HOSTNAME}:5003"
    setup_containerd_registry_mirror $NODE "registry.k8s.io" "https://registry.k8s.io" "http://${REGISTRY_HOSTNAME}:5006"
    setup_containerd_registry_mirror $NODE "quay.io" "https://quay.io" "http://${REGISTRY_HOSTNAME}:5007"
    setup_containerd_registry_mirror $NODE "europe-docker.pkg.dev" "https://europe-docker.pkg.dev" "http://${REGISTRY_HOSTNAME}:5008"
    setup_containerd_registry_mirror $NODE "garden.local.gardener.cloud:5001" "http://garden.local.gardener.cloud:5001" "http://${REGISTRY_HOSTNAME}:5001"
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

check_registry_cache_availability() {
  local registry_cache_ip
  local registry_cache_dns
  if [[ "$REGISTRY_CACHE" != "true" ]]; then
    return
  fi
  echo "Registry-cache enabled. Checking if registry-cache instances are deployed in prow cluster."
  for registry_cache_dns in $(kubectl create -k "$(dirname "$0")/../example/gardener-local/registry-prow" --dry-run=client -o yaml | grep kube-system.svc.cluster.local | awk '{ print $2 }' | sed -e "s/^http:\/\///" -e "s/:5000$//"); do
    registry_cache_ip=$(getent hosts "$registry_cache_dns" | awk '{ print $1 }' || true)
    if [[ "$registry_cache_ip" == "" ]]; then
      echo "Unable to resolve IP of $registry_cache_dns in prow cluster. Disabling registry-cache."
      REGISTRY_CACHE=false
    fi
  done
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

check_shell_dependencies() {
  errors=()

  if ! sed --version >/dev/null 2>&1; then
    errors+=("Current sed version does not support --version flag. Please ensure GNU sed is installed.")
  fi

  if tar --version 2>&1 | grep -q "bsdtar"; then
    errors+=("BSD tar detected. Please ensure GNU tar is installed.")
  fi

  if grep --version 2>&1 | grep -q "BSD grep"; then
    errors+=("BSD grep detected. Please ensure GNU grep is installed.")
  fi

  if [[ "$OSTYPE" == "darwin"* ]]; then
    if ! date --version >/dev/null 2>&1; then
      errors+=("Current date version does not support --version flag. Please ensure coreutils are installed.")
    fi

    if gzip --version 2>&1 | grep -q "Apple"; then
      errors+=("Apple built-in gzip utility detected. Please ensure GNU gzip is installed.")
    fi
  fi

  if [ "${#errors[@]}" -gt 0 ]; then
    printf 'Error: Required shell dependencies not met. Please refer to https://github.com/gardener/gardener/blob/master/docs/development/local_setup.md#macos-only-install-gnu-core-utilities:\n'
    printf '    - %s\n' "${errors[@]}"
    exit 1
  fi
}

check_shell_dependencies
check_local_dns_records

parse_flags "$@"

mkdir -m 0755 -p \
  "$(dirname "$0")/../dev/local-backupbuckets" \
  "$(dirname "$0")/../dev/local-registry"

LOOPBACK_IP_ADDRESSES=(172.18.255.1)
if [[ "$IPFAMILY" == "ipv6" ]] || [[ "$IPFAMILY" == "dual" ]]; then
  LOOPBACK_IP_ADDRESSES+=(::1)
fi

if [[ "$MULTI_ZONAL" == "true" ]]; then
  LOOPBACK_IP_ADDRESSES+=(172.18.255.10 172.18.255.11 172.18.255.12)
  if [[ "$IPFAMILY" == "ipv6" ]] || [[ "$IPFAMILY" == "dual" ]]; then
    LOOPBACK_IP_ADDRESSES+=(::10 ::11 ::12)
  fi
fi
if [[ "$CLUSTER_NAME" == "gardener-operator-local" ]]; then
  LOOPBACK_IP_ADDRESSES+=(172.18.255.3)
  if [[ "$IPFAMILY" == "ipv6" ]] || [[ "$IPFAMILY" == "dual" ]]; then
    LOOPBACK_IP_ADDRESSES+=(::3)
  fi
elif [[ "$CLUSTER_NAME" == "gardener-local2" || "$CLUSTER_NAME" == "gardener-local2-ha-single-zone" ]]; then
  LOOPBACK_IP_ADDRESSES+=(172.18.255.2)
  if [[ "$IPFAMILY" == "ipv6" ]] || [[ "$IPFAMILY" == "dual" ]]; then
    LOOPBACK_IP_ADDRESSES+=(::2)
  fi
fi
setup_loopback_device "${LOOPBACK_IP_ADDRESSES[@]}"

setup_kind_network

if [[ "$IPFAMILY" == "ipv6" ]]; then
  ADDITIONAL_ARGS="$ADDITIONAL_ARGS --values $CHART/values-ipv6.yaml"
fi
if [[ "$IPFAMILY" == "dual" ]]; then
  ADDITIONAL_ARGS="$ADDITIONAL_ARGS --values $CHART/values-dual.yaml"
fi

if [[ "$IPFAMILY" == "ipv6" ]] && [[ "$MULTI_ZONAL" == "true" ]]; then
  ADDITIONAL_ARGS="$ADDITIONAL_ARGS --set gardener.seed.istio.listenAddresses={::1,::10,::11,::12}"
fi

kind create cluster \
  --name "$CLUSTER_NAME" \
  --config <(helm template $CHART --values "$PATH_CLUSTER_VALUES" $ADDITIONAL_ARGS --set "gardener.repositoryRoot"=$(dirname "$0")/..)

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

if [[ "$KUBECONFIG" != "$PATH_KUBECONFIG" ]]; then
  cp "$KUBECONFIG" "$PATH_KUBECONFIG"
fi

# Prepare garden.local.gardener.cloud hostname that can be used everywhere to talk to the garden cluster.
# Historically, we used the docker container name for this, but this differs between clusters with different names
# and doesn't work in IPv6 kind clusters: https://github.com/kubernetes-sigs/kind/issues/3114
# Hence, we "manually" inject a host configuration into the cluster that always resolves to the kind container's IP,
# that serves our garden cluster API.
# This works in
# - the first and the second kind cluster
# - in IPv4 and IPv6 kind clusters
# - in ManagedSeeds

garden_cluster="$CLUSTER_NAME"
if [[ "$CLUSTER_NAME" == "gardener-local2" ]] ; then
  # garden-local2 is used as a second seed cluster, the first kind cluster runs the gardener control plane
  garden_cluster="gardener-local"
fi

if [[ "$CLUSTER_NAME" == "gardener-local2-ha-single-zone" ]]; then
  garden_cluster="gardener-local-ha-single-zone"
fi

ip_address_field="IPAddress"
if [[ "$IPFAMILY" == "ipv6" ]] || [[ "$IPFAMILY" == "dual" ]] ; then
  ip_address_field="GlobalIPv6Address"
fi

garden_cluster_ip="$(docker inspect "$garden_cluster"-control-plane | yq ".[].NetworkSettings.Networks.kind.$ip_address_field")"

# Inject garden.local.gardener.cloud into all nodes
for node in $nodes; do
  docker exec "$node" sh -c "echo $garden_cluster_ip garden.local.gardener.cloud >> /etc/hosts"
done

# Inject garden.local.gardener.cloud into coredns config (after ready plugin, before kubernetes plugin)
kubectl -n kube-system get configmap coredns -ojson | \
  yq '.data.Corefile' | \
  sed '0,/ready.*$/s//&'"\n\
    hosts {\n\
      $garden_cluster_ip garden.local.gardener.cloud\n\
      $garden_cluster_ip gardener.virtual-garden.local.gardener.cloud\n\
      $garden_cluster_ip api.virtual-garden.local.gardener.cloud\n\
      $garden_cluster_ip dashboard.ingress.runtime-garden.local.gardener.cloud\n\
      fallthrough\n\
    }\
"'/' | \
  kubectl -n kube-system create configmap coredns --from-file Corefile=/dev/stdin --dry-run=client -oyaml | \
  kubectl -n kube-system patch configmap coredns --patch-file /dev/stdin

kubectl -n kube-system rollout restart deployment coredns

if [[ "$DEPLOY_REGISTRY" == "true" ]]; then
  check_registry_cache_availability
  if [[ "$REGISTRY_CACHE" == "true" ]]; then
    echo "Deploying local container registries in registry-cache configuration"
    kubectl apply -k "$(dirname "$0")/../example/gardener-local/registry-prow" --server-side
  else
    echo "Deploying local container registries in default configuration"
    kubectl apply -k "$(dirname "$0")/../example/gardener-local/registry" --server-side
  fi
  kubectl wait --for=condition=available deployment -l app=registry -n registry --timeout 5m
fi
kubectl apply -k "$(dirname "$0")/../example/gardener-local/calico/$IPFAMILY" --server-side
kubectl apply -k "$(dirname "$0")/../example/gardener-local/metrics-server"   --server-side

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
