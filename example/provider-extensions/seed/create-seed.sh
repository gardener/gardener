#!/usr/bin/env bash
#
# Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#      http://www.apache.org/licenses~LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -e

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"

usage() {
  echo "Usage:"
  echo "> create-seed.sh [ -h | <path to skaffold binary> <garden-kubeconfig> <seed-kubeconfig> ]"
  echo
  echo ">> For example: create-seed.sh /usr/bin/skaffold ~/.kube/garden-kubeconfig.yaml ~/.kube/kubeconfig.yaml"

  exit 0
}

if [ "$1" == "-h" ] || [ "$#" -ne 3 ]; then
  usage
fi

skaffold=$1
garden_kubeconfig=$2
seed_kubeconfig=$3

temp_shoot_info=$(mktemp)
cleanup-shoot-info() {
  rm -f "$temp_shoot_info"
}
trap cleanup-shoot-info EXIT

echo "Getting config from shoot"
kubectl get configmaps -n kube-system shoot-info --kubeconfig $seed_kubeconfig -o yaml > $temp_shoot_info

host=$(yq '.data.domain' $temp_shoot_info)
pods_cidr=$(yq '.data.podNetwork' $temp_shoot_info)
nodes_cidr=$(yq '.data.nodeNetwork' $temp_shoot_info)
services_cidr=$(yq '.data.serviceNetwork' $temp_shoot_info)
region=$(yq '.data.region' $temp_shoot_info)
type=$(yq '.data.provider' $temp_shoot_info)
internal_dns_secret=$(yq '.global.internalDomain.domain' $SCRIPT_DIR/../../gardener-local/controlplane/extensions-config/values.yaml | sed 's/\./-/g' | sed 's/^/internal-domain-/')
dns_provider_type=$(yq '.global.internalDomain.provider' $SCRIPT_DIR/../../gardener-local/controlplane/extensions-config/values.yaml)

echo "Skaffolding seed"
GARDENER_LOCAL_KUBECONFIG=$garden_kubeconfig \
  SKAFFOLD_DEFAULT_REPO=reg.$host \
  HOST=$host \
  PODS_CIDR=$pods_cidr \
  NODES_CIDR=$nodes_cidr \
  SERVICES_CIDR=$services_cidr \
  REGION=$region \
  TYPE=$type \
  INTERNAL_DNS_SECRET=$internal_dns_secret \
  DNS_PROVIDER_TYPE=$dns_provider_type \
  SKAFFOLD_PUSH=true \
  $skaffold run -m gardenlet -p extensions --kubeconfig=$seed_kubeconfig
