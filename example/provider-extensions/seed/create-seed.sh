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
  echo "> create-seed.sh [ -h | <garden-kubeconfig> <seed-kubeconfig> <seed-name> ]"
  echo
  echo ">> For example: create-seed.sh ~/.kube/garden-kubeconfig.yaml ~/.kube/kubeconfig.yaml provider-extensions"

  exit 0
}

if [ "$1" == "-h" ] || [ "$#" -ne 3 ]; then
  usage
fi

garden_kubeconfig=$1
seed_kubeconfig=$2
seed_name=$3

temp_shoot_info=$(mktemp)
cleanup-shoot-info() {
  rm -f "$temp_shoot_info"
}
trap cleanup-shoot-info EXIT

default-if-initial() {
  local var=$1
  local file=$2
  local yqArg=$3
  local prefix=$4

  if [[  $var  == "" ]] || [[  $var  == "null" ]]; then
    echo "${prefix}$(yq "${yqArg}" "$file")"
  else
    echo "$var"
  fi
}

ingress_domain=$(yq -e '.ingressDomain' "$SCRIPT_DIR"/seed-config.yaml)
zones=$(yq -e '.zones | (.[] |= sub("(.*)", "\"${1}\"")) | join(",")' "$SCRIPT_DIR"/seed-config.yaml)
registry_domain=$(yq '.registryDomain' "$SCRIPT_DIR"/seed-config.yaml)
pods_cidr=$(yq '.podNetwork' "$SCRIPT_DIR"/seed-config.yaml)
nodes_cidr=$(yq '.nodeNetwork' "$SCRIPT_DIR"/seed-config.yaml)
services_cidr=$(yq '.serviceNetwork' "$SCRIPT_DIR"/seed-config.yaml)
region=$(yq '.region' "$SCRIPT_DIR"/seed-config.yaml)
type=$(yq '.provider' "$SCRIPT_DIR"/seed-config.yaml)
internal_dns_secret=$(yq -e '.global.internalDomain.domain' "$SCRIPT_DIR"/../../provider-extensions/garden/controlplane/values.yaml | sed 's/\./-/g' | sed 's/^/internal-domain-/')
dns_provider_type=$(yq -e '.global.internalDomain.provider' "$SCRIPT_DIR"/../../provider-extensions/garden/controlplane/values.yaml)

if [[ $(yq '.useGardenerShootInfo' "$SCRIPT_DIR"/seed-config.yaml) == "true" ]]; then
  echo "Getting config from shoot"
  kubectl get configmaps -n kube-system shoot-info --kubeconfig "$seed_kubeconfig" -o yaml > "$temp_shoot_info"

  registry_domain=$(default-if-initial "$registry_domain" "$temp_shoot_info" ".data.domain" "reg.")
  pods_cidr=$(default-if-initial "$pods_cidr" "$temp_shoot_info" ".data.podNetwork")
  nodes_cidr=$(default-if-initial "$nodes_cidr" "$temp_shoot_info" ".data.nodeNetwork")
  services_cidr=$(default-if-initial "$services_cidr" "$temp_shoot_info" ".data.serviceNetwork")
  region=$(default-if-initial "$region" "$temp_shoot_info" ".data.region")
  type=$(default-if-initial "$type" "$temp_shoot_info" ".data.provider")
fi

yq -e -i "
  .config.seedConfig.metadata.name = \"$seed_name\" |
  .config.seedConfig.spec.ingress.domain = \"$ingress_domain\" |
  .config.seedConfig.spec.networks.pods = \"$pods_cidr\" |
  .config.seedConfig.spec.networks.nodes = \"$nodes_cidr\" |
  .config.seedConfig.spec.networks.services = \"$services_cidr\" |
  .config.seedConfig.spec.dns.provider.secretRef.name = \"$internal_dns_secret\" |
  .config.seedConfig.spec.dns.provider.type = \"$dns_provider_type\" |
  .config.seedConfig.spec.provider.region = \"$region\" |
  .config.seedConfig.spec.provider.type = \"$type\" |
  .config.seedConfig.spec.provider.zones = [$zones]
" "$SCRIPT_DIR"/../../provider-extensions/gardenlet/values.yaml

echo "Skaffolding seed"
GARDENER_LOCAL_KUBECONFIG=$garden_kubeconfig \
  SKAFFOLD_DEFAULT_REPO=$registry_domain \
  REGISTRY_DOMAIN=$registry_domain \
  SKAFFOLD_PUSH=true \
  skaffold run -m gardenlet -p extensions --kubeconfig="$seed_kubeconfig"
