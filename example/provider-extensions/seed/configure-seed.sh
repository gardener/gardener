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
REPO_ROOT_DIR="$(realpath "$SCRIPT_DIR"/../../..)"

usage() {
  echo "Usage:"
  echo "> configure-seed.sh [ -h | <garden-kubeconfig> <seed-kubeconfig> ]"
  echo
  echo ">> For example: configure-seed.sh ~/.kube/garden-kubeconfig.yaml ~/.kube/kubeconfig.yaml provider-extensions"

  exit 0
}

if [ "$1" == "-h" ] || [ "$#" -ne 2 ]; then
  usage
fi

garden_kubeconfig=$1
seed_kubeconfig=$2

temp_shoot_info=$(mktemp)
cleanup-shoot-info() {
  rm -f "$temp_shoot_info"
}
trap cleanup-shoot-info EXIT

ensure-config-file() {
  local file=$1
  if [[ ! -f "$file" ]]; then
    echo "Creating \"$file\" from template."
    cp "$file".tmpl "$file"
  fi
}

check-not-initial() {
  local file=$1
  local yqArg=$2

  if [[ $yqArg == "" ]]; then
    if [[ ! -f "$file" ]]; then
      echo "File \"$file\" does not exist. Please check your config."
      exit 1
    fi
  else
    local yqResult
    yqResult=$(yq "${yqArg}" "$file")
    if [[  $yqResult  == "" ]] || [[  $yqResult  == "null" ]]; then
      echo "\"$yqArg\" in file \"$file\" is empty or does not exist. Please check your config."
      exit 1
    fi
  fi

}

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

ensure-gardener-dns-annotations() {
  local namespace=$1
  local name=$2
  local domain=$3

  kubectl annotate --overwrite --kubeconfig "$seed_kubeconfig" svc -n "$namespace" "$name" \
    dns.gardener.cloud/class=garden \
    dns.gardener.cloud/ttl="60" \
    dns.gardener.cloud/dnsnames="$domain" \
    cert.gardener.cloud/commonname="$domain" \
    cert.gardener.cloud/dnsnames="$domain" \
    cert.gardener.cloud/purpose=managed \
    cert.gardener.cloud/secretname=tls

}

echo "Ensuring config files"
ensure-config-file "$SCRIPT_DIR"/seed-config.yaml
ensure-config-file "$REPO_ROOT_DIR"/example/provider-extensions/garden/controlplane/values.yaml
ensure-config-file "$REPO_ROOT_DIR"/example/provider-extensions/garden/project/credentials/infrastructure-secrets.yaml
ensure-config-file "$REPO_ROOT_DIR"/example/provider-extensions/garden/project/credentials/secret-bindings.yaml
touch -a "$REPO_ROOT_DIR"/example/provider-extensions/gardenlet/values.yaml

echo "Check if essential config options are initialized"
check-not-initial "$SCRIPT_DIR"/kubeconfig ""
check-not-initial "$SCRIPT_DIR"/seed-config.yaml ".ingressDomain"
check-not-initial "$REPO_ROOT_DIR"/example/provider-extensions/garden/controlplane/values.yaml ".global.internalDomain.domain"
check-not-initial "$SCRIPT_DIR"/seed-config.yaml ".useGardenerShootInfo"
check-not-initial "$SCRIPT_DIR"/seed-config.yaml ".useGardenerShootDNS"


registry_domain=$(yq '.registryDomain' "$SCRIPT_DIR"/seed-config.yaml)
relay_domain=$(yq '.relayDomain' "$SCRIPT_DIR"/seed-config.yaml)
type=$(yq '.provider' "$SCRIPT_DIR"/seed-config.yaml)

if [[ $(yq '.useGardenerShootInfo' "$SCRIPT_DIR"/seed-config.yaml) == "true" ]]; then
  echo "Getting config from shoot"
  kubectl get configmaps -n kube-system shoot-info --kubeconfig "$seed_kubeconfig" -o yaml > "$temp_shoot_info"

  registry_domain=$(default-if-initial "$registry_domain" "$temp_shoot_info" ".data.domain" "reg.")
  relay_domain=$(default-if-initial "$relay_domain" "$temp_shoot_info" ".data.domain" "relay.")
  type=$(default-if-initial "$type" "$temp_shoot_info" ".data.provider")
fi

if [[ $registry_domain == "$relay_domain" ]]; then
  echo "registry and relay domains must not be equal"
  exit 1
fi

echo "Deploying load-balancer services"
kubectl --server-side=true --kubeconfig "$seed_kubeconfig" apply -k "$SCRIPT_DIR"/../registry-seed/load-balancer/base

if [[ $type == "aws" ]]; then
  kubectl --server-side=true --kubeconfig "$seed_kubeconfig" apply -k "$SCRIPT_DIR"/../quic-relay/load-balancer/aws
else
  kubectl --server-side=true --kubeconfig "$seed_kubeconfig" apply -k "$SCRIPT_DIR"/../quic-relay/load-balancer/base
fi

if [[ $(yq '.useGardenerShootDNS' "$SCRIPT_DIR"/seed-config.yaml) == "true" ]]; then
  ensure-gardener-dns-annotations registry registry "$registry_domain"
  ensure-gardener-dns-annotations relay gardener-api-quic-server "$relay_domain"
else
  echo "######################################################################################"
  echo "Please create DNS entries and generate TLS certificates for registry and relay domains"
  echo "######################################################################################"
  echo "Registry domain:"
  echo "DNS entry for domain: \"$registry_domain\" -> IP from load balancer service \"kubectl get svc -n registry registry -o yaml\""
  echo "TLS certificate for domain \"$registry_domain\" -> Please store the TLS certificate in secret \"name: tls namespace: registry\" (https://kubernetes.io/docs/concepts/configuration/secret/#tls-secrets)"
  echo " "
  echo "Relay domain:"
  echo "DNS entry for domain: \"$relay_domain\" -> IP from load balancer service \"kubectl get svc -n relay gardener-api-quic-server -o yaml\""
  echo "######################################################################################"
  read -rsp "When you are ready, please press ENTER to continue"
fi

echo "Deploying kyverno, quic-relay and container registry"
kubectl --server-side=true --kubeconfig "$seed_kubeconfig" apply -k "$SCRIPT_DIR"/../kyverno
until kubectl --kubeconfig "$seed_kubeconfig" get clusterpolicies.kyverno.io ; do date; sleep 1; echo ""; done
kubectl --server-side=true --force-conflicts=true --kubeconfig "$seed_kubeconfig" apply -k "$SCRIPT_DIR"/../kyverno-policies
"$SCRIPT_DIR"/../quic-relay/deploy-quic-relay.sh "$garden_kubeconfig" "$seed_kubeconfig" "$relay_domain"
"$SCRIPT_DIR"/../registry-seed/deploy-registry.sh "$seed_kubeconfig" "$registry_domain"
