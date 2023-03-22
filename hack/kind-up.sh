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
  for address in ${LOOPBACK_IP_ADDRESSES[@]}; do
    if ip address show dev ${LOOPBACK_DEVICE} | grep -q $address; then
      echo "IP address $address already assigned to ${LOOPBACK_DEVICE}."
    else
      echo "Adding IP address $address to ${LOOPBACK_DEVICE}..."
      sudo ip address add $address dev ${LOOPBACK_DEVICE}
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

if [[ "$IPFAMILY" == "ipv6" ]]; then
  ADDITIONAL_ARGS="$ADDITIONAL_ARGS --values $CHART/values-ipv6.yaml"
fi

if [[ "$IPFAMILY" == "ipv6" ]] && [[ "$MULTI_ZONAL" == "true" ]]; then
  ADDITIONAL_ARGS="$ADDITIONAL_ARGS --set gardener.seed.istio.listenAddresses={::1,::10,::11,::12}"
fi

kind create cluster \
  --name "$CLUSTER_NAME" \
  --config <(helm template $CHART --values "$PATH_CLUSTER_VALUES" $ADDITIONAL_ARGS --set "environment=$ENVIRONMENT" --set "gardener.repositoryRoot"=$(dirname "$0")/..)

# workaround https://kind.sigs.k8s.io/docs/user/known-issues/#pod-errors-due-to-too-many-open-files
kubectl get nodes -o name |\
  cut -d/ -f2 |\
  xargs -I {} docker exec {} sh -c "sysctl fs.inotify.max_user_instances=8192"

if [[ "$KUBECONFIG" != "$PATH_KUBECONFIG" ]]; then
  cp "$KUBECONFIG" "$PATH_KUBECONFIG"
fi

if [[ "$DEPLOY_REGISTRY" == "true" ]]; then
  kubectl apply -k "$(dirname "$0")/../example/gardener-local/registry" --server-side
  kubectl wait --for=condition=available deployment -l app=registry -n registry --timeout 5m
fi
kubectl apply -k "$(dirname "$0")/../example/gardener-local/calico/$IPFAMILY" --server-side
kubectl apply -k "$(dirname "$0")/../example/gardener-local/metrics-server"   --server-side

kubectl get nodes -l node-role.kubernetes.io/control-plane -o name |\
  cut -d/ -f2 |\
  xargs -I {} kubectl taint node {} node-role.kubernetes.io/master:NoSchedule- node-role.kubernetes.io/control-plane:NoSchedule- || true

# Allow multiple shoot worker nodes with calico as shoot CNI: As we run overlay in overlay ip-in-ip needs to be allowed in the workload.
# Unfortunately, the felix configuration is created on the fly by calico. Hence, we need to poll until kubectl wait for new resources
# (https://github.com/kubernetes/kubernetes/issues/83242) is fixed. (2 minutes should be enough for the felix configuration to be created.)
echo "Waiting for FelixConfiguration to be created..."
felix_config_found=0
max_retries=120
for ((i = 0; i < max_retries; i++)); do
  if kubectl get felixconfiguration default > /dev/null 2>&1; then
    if kubectl patch felixconfiguration default --type merge --patch '{"spec":{"allowIPIPPacketsFromWorkloads":true}}' > /dev/null 2>&1; then
      echo "FelixConfiguration 'default' successfully updated."
      felix_config_found=1
      break
    fi
  fi
  sleep 1s
done
if [ $felix_config_found -eq 0 ]; then
  echo "Error: FelixConfiguration 'default' not found or patch failed after $max_retries attempts."
  exit 1
fi
