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
  echo "> configure-seed.sh [ -h | <garden-kubeconfig> <seed-kubeconfig> <seed-name> ]"
  echo
  echo ">> For example: configure-seed.sh ~/.kube/garden-kubeconfig.yaml ~/.kube/kubeconfig.yaml provider-extensions"

  exit 0
}

if [ "$1" == "-h" ] || [ "$#" -ne 3 ]; then
  usage
fi

garden_kubeconfig=$1
seed_kubeconfig=$2
seed_name=$3

use_shoot_info="false"
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
    if [[  $yqResult  == "" ]] || [[  $yqResult  == "null" ]] || [[  $yqResult == "[]" ]]; then
      echo "\"$yqArg\" in file \"$file\" is empty or does not exist. Please check your config."
      exit 1
    fi
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
ensure-config-file "$REPO_ROOT_DIR"/example/provider-extensions/garden/controlplane/values.yaml
ensure-config-file "$REPO_ROOT_DIR"/example/provider-extensions/garden/project/credentials/infrastructure-secrets.yaml
ensure-config-file "$REPO_ROOT_DIR"/example/provider-extensions/garden/project/credentials/secretbindings.yaml
ensure-config-file "$REPO_ROOT_DIR"/example/provider-extensions/gardenlet/values.yaml

echo "Check if essential config options are initialized"
check-not-initial "$SCRIPT_DIR"/kubeconfig ""
check-not-initial "$REPO_ROOT_DIR"/example/provider-extensions/garden/controlplane/values.yaml ".global.internalDomain.domain"
check-not-initial "$REPO_ROOT_DIR"/example/provider-extensions/garden/controlplane/values.yaml ".global.internalDomain.provider"

registry_domain=
relay_domain=

internal_dns_secret=$(yq -e '.global.internalDomain.domain' "$REPO_ROOT_DIR"/example/provider-extensions/garden/controlplane/values.yaml | sed 's/\./-/g' | sed 's/^/internal-domain-/')
dns_provider_type=$(yq -e '.global.internalDomain.provider' "$REPO_ROOT_DIR"/example/provider-extensions/garden/controlplane/values.yaml)

if kubectl get configmaps -n kube-system shoot-info --kubeconfig "$seed_kubeconfig" -o yaml > "$temp_shoot_info"; then
  use_shoot_info="true"
  echo "Getting config from shoot"
  registry_domain=reg.$(yq -e '.data.domain' "$temp_shoot_info")
  relay_domain=relay.$(yq -e '.data.domain' "$temp_shoot_info")
  pods_cidr=$(yq -e '.data.podNetwork' "$temp_shoot_info")
  nodes_cidr=$(yq -e '.data.nodeNetwork' "$temp_shoot_info")
  services_cidr=$(yq -e '.data.serviceNetwork' "$temp_shoot_info")
  region=$(yq -e '.data.region' "$temp_shoot_info")
  type=$(yq -e '.data.provider' "$temp_shoot_info")

  yq -e -i "
    .config.seedConfig.metadata.name = \"$seed_name\" |
    .config.seedConfig.spec.networks.pods = \"$pods_cidr\" |
    .config.seedConfig.spec.networks.nodes = \"$nodes_cidr\" |
    .config.seedConfig.spec.networks.services = \"$services_cidr\" |
    .config.seedConfig.spec.dns.provider.secretRef.name = \"$internal_dns_secret\" |
    .config.seedConfig.spec.dns.provider.type = \"$dns_provider_type\" |
    .config.seedConfig.spec.provider.region = \"$region\" |
    .config.seedConfig.spec.provider.type = \"$type\"
  " "$REPO_ROOT_DIR"/example/provider-extensions/gardenlet/values.yaml
else
  echo "######################################################################################"
  echo "Please enter domain names for registry and relay domains on the seed"
  echo "######################################################################################"
  echo "Registry domain:"
  read -er registry_domain
  echo "Relay domain:"
  read -er relay_domain
  echo "######################################################################################"

  yq -e -i "
    .config.seedConfig.metadata.name = \"$seed_name\" |
    .config.seedConfig.spec.dns.provider.secretRef.name = \"$internal_dns_secret\" |
    .config.seedConfig.spec.dns.provider.type = \"$dns_provider_type\"
  " "$REPO_ROOT_DIR"/example/provider-extensions/gardenlet/values.yaml
fi

if [[ $registry_domain == "$relay_domain" ]]; then
  echo "registry and relay domains must not be equal"
  exit 1
fi
echo "$registry_domain" > "$SCRIPT_DIR"/registrydomain

echo "Check if gardenlet values.yaml is complete"
check-not-initial "$REPO_ROOT_DIR"/example/provider-extensions/gardenlet/values.yaml ".config.seedConfig.metadata.name"
check-not-initial "$REPO_ROOT_DIR"/example/provider-extensions/gardenlet/values.yaml ".config.seedConfig.spec.ingress.domain"
check-not-initial "$REPO_ROOT_DIR"/example/provider-extensions/gardenlet/values.yaml ".config.seedConfig.spec.networks.pods"
check-not-initial "$REPO_ROOT_DIR"/example/provider-extensions/gardenlet/values.yaml ".config.seedConfig.spec.networks.nodes"
check-not-initial "$REPO_ROOT_DIR"/example/provider-extensions/gardenlet/values.yaml ".config.seedConfig.spec.networks.services"
check-not-initial "$REPO_ROOT_DIR"/example/provider-extensions/gardenlet/values.yaml ".config.seedConfig.spec.dns.provider.secretRef.name"
check-not-initial "$REPO_ROOT_DIR"/example/provider-extensions/gardenlet/values.yaml ".config.seedConfig.spec.dns.provider.type"
check-not-initial "$REPO_ROOT_DIR"/example/provider-extensions/gardenlet/values.yaml ".config.seedConfig.spec.provider.region"
check-not-initial "$REPO_ROOT_DIR"/example/provider-extensions/gardenlet/values.yaml ".config.seedConfig.spec.provider.type"
check-not-initial "$REPO_ROOT_DIR"/example/provider-extensions/gardenlet/values.yaml ".config.seedConfig.spec.provider.zones"

echo "Deploying load-balancer services"
kubectl --server-side=true --kubeconfig "$seed_kubeconfig" apply -k "$SCRIPT_DIR"/../registry-seed/load-balancer/base
kubectl --server-side=true --kubeconfig "$seed_kubeconfig" apply -k "$SCRIPT_DIR"/../ssh-reverse-tunnel/load-balancer

if [[ $use_shoot_info == "true" ]]; then
  ensure-gardener-dns-annotations registry registry "$registry_domain"
  ensure-gardener-dns-annotations relay gardener-apiserver-tunnel "$relay_domain"
else
  echo "######################################################################################"
  echo "Please create DNS entries and generate TLS certificates for registry and relay domains"
  echo "######################################################################################"
  echo "Registry domain:"
  echo "DNS entry for domain: \"$registry_domain\" -> IP from load balancer service \"kubectl get svc -n registry registry -o yaml\""
  echo "TLS certificate for domain \"$registry_domain\" -> Please store the TLS certificate in secret \"name: tls namespace: registry\" (https://kubernetes.io/docs/concepts/configuration/secret/#tls-secrets)"
  echo " "
  echo "Relay domain:"
  echo "DNS entry for domain: \"$relay_domain\" -> IP from load balancer service \"kubectl get svc -n relay gardener-apiserver-tunnel -o yaml\""
  echo "######################################################################################"
  read -rsp "When you are ready, please press ENTER to continue"
fi

echo "Create host and client keys for SSH reverse tunnel"
"$SCRIPT_DIR"/../ssh-reverse-tunnel/create-host-keys.sh "$relay_domain" 6222
"$SCRIPT_DIR"/../ssh-reverse-tunnel/create-client-keys.sh "$relay_domain" provider-extensions

echo "Deploying kyverno, SSH reverse tunnel and container registry"
kubectl --server-side=true --kubeconfig "$seed_kubeconfig" apply -k "$SCRIPT_DIR"/../kyverno
until kubectl --kubeconfig "$seed_kubeconfig" get clusterpolicies.kyverno.io ; do date; sleep 1; echo ""; done
kubectl --server-side=true --force-conflicts=true --kubeconfig "$seed_kubeconfig" apply -k "$SCRIPT_DIR"/../kyverno-policies
kubectl --server-side=true --kubeconfig "$seed_kubeconfig" apply -k "$SCRIPT_DIR"/../ssh-reverse-tunnel/sshd
kubectl --server-side=true --kubeconfig "$garden_kubeconfig" apply -k "$SCRIPT_DIR"/../ssh-reverse-tunnel/ssh
"$SCRIPT_DIR"/../registry-seed/deploy-registry.sh "$seed_kubeconfig" "$registry_domain"
