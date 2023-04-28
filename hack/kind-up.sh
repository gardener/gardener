#!/usr/bin/env bash
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


set -o errexit
set -o nounset
set -o pipefail

CLUSTER_NAME=""
PATH_CLUSTER_VALUES=""
PATH_KUBECONFIG=""
ENVIRONMENT="skaffold"
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
    --environment)
      shift; ENVIRONMENT="$1"
      ;;
    --skip-registry)
      DEPLOY_REGISTRY=false
      ;;
    --multi-zonal)
      MULTI_ZONAL=true
      ;;
    esac

    shift
  done
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
  if ! command -v ip &>/dev/null; then
    if [[ "$OSTYPE" == "darwin"* ]]; then
      echo "'ip' command not found. Please install 'ip' command, refer https://github.com/gardener/gardener/blob/master/docs/development/local_setup.md#installing-iproute2" 1>&2
      exit 1
    fi
    echo "Skipping loopback device setup because 'ip' command is not available..."
    return
  fi
  LOOPBACK_DEVICE=$(ip address | grep LOOPBACK | sed "s/^[0-9]\+: //g" | awk '{print $1}' | sed "s/:$//g")
  LOOPBACK_IP_ADDRESSES=(127.0.0.10 127.0.0.11 127.0.0.12)
  if [[ "$IPFAMILY" == "ipv6" ]]; then
    LOOPBACK_IP_ADDRESSES+=(::10 ::11 ::12)
  fi
  echo "Checking loopback device ${LOOPBACK_DEVICE}..."
  for address in "${LOOPBACK_IP_ADDRESSES[@]}"; do
    if ip address show dev ${LOOPBACK_DEVICE} | grep -q $address; then
      echo "IP address $address already assigned to ${LOOPBACK_DEVICE}."
    else
      echo "Adding IP address $address to ${LOOPBACK_DEVICE}..."
      ${SUDO}ip address add "$address" dev "${LOOPBACK_DEVICE}"
    fi
  done
  echo "Setting up loopback device ${LOOPBACK_DEVICE} completed."
}

parse_flags "$@"

mkdir -m 0755 -p \
  "$(dirname "$0")/../dev/local-backupbuckets" \
  "$(dirname "$0")/../dev/local-registry"

if [[ "$MULTI_ZONAL" == "true" ]]; then
  setup_loopback_device
fi

setup_kind_network

if [[ "$IPFAMILY" == "ipv6" ]]; then
  ADDITIONAL_ARGS="$ADDITIONAL_ARGS --values $CHART/values-ipv6.yaml"
fi

if [[ "$IPFAMILY" == "ipv6" ]] && [[ "$MULTI_ZONAL" == "true" ]]; then
  ADDITIONAL_ARGS="$ADDITIONAL_ARGS --set gardener.seed.istio.listenAddresses={::1,::10,::11,::12}"
fi

kind create cluster \
  --name "$CLUSTER_NAME" \
  --image "kindest/node:v1.26.3" \
  --config <(helm template $CHART --values "$PATH_CLUSTER_VALUES" $ADDITIONAL_ARGS --set "environment=$ENVIRONMENT" --set "gardener.repositoryRoot"=$(dirname "$0")/..)

# adjust Kind's CRI default OCI runtime spec for new containers to include the cgroup namespace
# this is required for nesting kubelets on cgroupsv2, as the kindest-node entrypoint script assumes an existing cgroupns when the host kernel uses cgroupsv2
# See containerd CRI: https://github.com/containerd/containerd/commit/687469d3cee18bf0e12defa5c6d0c7b9139a2dbd
if [ -f "/sys/fs/cgroup/cgroup.controllers" ]; then
    echo "Host uses cgroupsv2"
    cat << 'EOF' > adjust_cri_base.sh
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

    for node_name in $(kubectl get nodes -o name | cut -d/ -f2)
    do
        echo "Adjusting containerd config for kind node $node_name"

        # copy script to the kind's docker container and execute it
        docker cp adjust_cri_base.sh "$node_name":/etc/containerd/adjust_cri_base.sh
        docker exec "$node_name" bash -c "chmod +x /etc/containerd/adjust_cri_base.sh && /etc/containerd/adjust_cri_base.sh && systemctl restart containerd"
    done
fi

# workaround https://kind.sigs.k8s.io/docs/user/known-issues/#pod-errors-due-to-too-many-open-files
kubectl get nodes -o name |\
  cut -d/ -f2 |\
  xargs -I {} docker exec {} sh -c "sysctl fs.inotify.max_user_instances=8192"

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
if [[ "$IPFAMILY" == "ipv6" ]]; then
  ip_address_field="GlobalIPv6Address"
fi

garden_cluster_ip="$(docker inspect "$garden_cluster"-control-plane | yq ".[].NetworkSettings.Networks.kind.$ip_address_field")"

# Inject garden.local.gardener.cloud into all nodes
kubectl get nodes -o name |\
  cut -d/ -f2 |\
  xargs -I {} docker exec {} sh -c "echo $garden_cluster_ip garden.local.gardener.cloud >> /etc/hosts"

# Inject garden.local.gardener.cloud into coredns config (after ready plugin, before kubernetes plugin)
kubectl -n kube-system get configmap coredns -ojson | \
  yq '.data.Corefile' | \
  sed '0,/ready.*$/s//&'"\n\
    hosts {\n\
      $garden_cluster_ip garden.local.gardener.cloud\n\
      fallthrough\n\
    }\
"'/' | \
  kubectl -n kube-system create configmap coredns --from-file Corefile=/dev/stdin --dry-run=client -oyaml | \
  kubectl -n kube-system patch configmap coredns --patch-file /dev/stdin

kubectl -n kube-system rollout restart deployment coredns

if [[ "$DEPLOY_REGISTRY" == "true" ]]; then
  kubectl apply -k "$(dirname "$0")/../example/gardener-local/registry" --server-side
  kubectl wait --for=condition=available deployment -l app=registry -n registry --timeout 5m
fi
kubectl apply -k "$(dirname "$0")/../example/gardener-local/calico/$IPFAMILY" --server-side
kubectl apply -k "$(dirname "$0")/../example/gardener-local/metrics-server"   --server-side

kubectl get nodes -l node-role.kubernetes.io/control-plane -o name |\
  cut -d/ -f2 |\
  xargs -I {} kubectl taint node {} node-role.kubernetes.io/control-plane:NoSchedule- || true

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
