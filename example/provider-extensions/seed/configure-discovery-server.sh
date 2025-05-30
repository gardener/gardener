#!/usr/bin/env bash
#
# SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

set -e

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"

usage() {
  echo "Usage:"
  echo "> ${0} [ -h | <garden-kubeconfig> <seed-kubeconfig> ]"
  echo
  echo ">> For example: ${0} ~/.kube/garden-kubeconfig.yaml ~/.kube/seed-kubeconfig.yaml"

  exit 0
}

if [ "$1" == "-h" ] || [ "$#" -ne 2 ]; then
  usage
fi

garden_kubeconfig=$1
seed_kubeconfig=$2

use_shoot_info="false"
temp_shoot_info=$(mktemp)
cleanup-shoot-info() {
  rm -f "$temp_shoot_info"
}
trap cleanup-shoot-info EXIT

ensure-gardener-dns-and-cert-annotations() {
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

gardener_issuer_domain=

if kubectl get configmaps -n kube-system shoot-info --kubeconfig "$seed_kubeconfig" -o yaml > "$temp_shoot_info"; then
  use_shoot_info="true"
  echo "Getting config from shoot"
  gardener_issuer_domain=issuer.$(yq -e '.data.domain' "$temp_shoot_info")
else
  echo "######################################################################################"
  echo "Please enter domain name for gardener issuer domain on the seed"
  echo "######################################################################################"
  echo "Gardener Issuer domain:"
  read -er gardener_issuer_domain
  echo "######################################################################################"
fi

echo "Deploying load-balancer service for Gardener Discovery Server"
kubectl --server-side=true --kubeconfig "$seed_kubeconfig" apply -f "$SCRIPT_DIR"/../gardener-discovery-server/load-balancer

if [[ $use_shoot_info == "true" ]]; then
  ensure-gardener-dns-and-cert-annotations gardener-discovery-server gardener-discovery-server "$gardener_issuer_domain"
else
  echo "######################################################################################"
  echo "Please create DNS entries and generate TLS certificates for gardener workload identity issuer domain"
  echo "######################################################################################"
  echo "Gardener Issuer domain:"
  echo "DNS entry for domain: \"$gardener_issuer_domain\" -> IP from load balancer service \"kubectl get svc -n gardener-discovery-server gardener-discovery-server -o yaml\""
  echo "TLS certificate for domain \"$gardener_issuer_domain\" -> Please store the TLS certificate in secret \"name: tls namespace: gardener-discovery-server\" (https://kubernetes.io/docs/concepts/configuration/secret/#tls-secrets)"
  echo "######################################################################################"
  read -rsp "When you are ready, please press ENTER to continue"
fi

"$SCRIPT_DIR"/../gardener-discovery-server/deploy.sh "$garden_kubeconfig" "$seed_kubeconfig"
