#!/usr/bin/env bash

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
  echo "Checking loopback device ${LOOPBACK_DEVICE}..."
  for address in 127.0.0.10 127.0.0.11 127.0.0.12; do
    if ip address show dev ${LOOPBACK_DEVICE} | grep -q $address; then
      echo "IP address $address already assigned to ${LOOPBACK_DEVICE}."
    else
      echo "Adding IP address $address to ${LOOPBACK_DEVICE}..."
      ip address add $address dev ${LOOPBACK_DEVICE}
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

kind create cluster \
  --name "$CLUSTER_NAME" \
  --config <(helm template $CHART --values "$PATH_CLUSTER_VALUES" --set "environment=$ENVIRONMENT" --set "gardener.repositoryRoot"=$(dirname "$0")/..)

# workaround https://kind.sigs.k8s.io/docs/user/known-issues/#pod-errors-due-to-too-many-open-files
kubectl get nodes -o name |\
  cut -d/ -f2 |\
  xargs -I {} docker exec {} sh -c "sysctl fs.inotify.max_user_instances=8192"

if [[ "$KUBECONFIG" != "$PATH_KUBECONFIG" ]]; then
  cp "$KUBECONFIG" "$PATH_KUBECONFIG"
fi

if [[ "$DEPLOY_REGISTRY" == "true" ]]; then
  kubectl apply -k "$(dirname "$0")/../example/gardener-local/registry"       --server-side
  kubectl wait --for=condition=available deployment -l app=registry -n registry --timeout 5m
fi
kubectl apply   -k "$(dirname "$0")/../example/gardener-local/calico"         --server-side
kubectl apply   -k "$(dirname "$0")/../example/gardener-local/metrics-server" --server-side

kubectl get nodes -l node-role.kubernetes.io/control-plane -o name |\
  cut -d/ -f2 |\
  xargs -I {} kubectl taint node {} node-role.kubernetes.io/master:NoSchedule- node-role.kubernetes.io/control-plane:NoSchedule- || true
