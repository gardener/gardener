#!/usr/bin/env bash
#
# Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
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
  echo "> deploy-registry.sh [ -h | <kubeconfig> <registry> ]"
  echo
  echo ">> For example: deploy-registry.sh ~/.kube/kubeconfig.yaml registry.gardener.cloud"

  exit 0
}

if [ "$1" == "-h" ] || [ "$#" -ne 2 ]; then
  usage
fi

if ! [ -x "$(command -v "htpasswd")" ]; then
  echo "ERROR: htpasswd is not present. Exiting..."
  exit 1
fi

kubeconfig=$1
registry=$2

echo "Generating new password for container registry $registry"
mkdir -p "$SCRIPT_DIR"/htpasswd
password=$(openssl rand -base64 20)
htpasswd -Bbn gardener "$password" > "$SCRIPT_DIR"/htpasswd/auth

echo "Creating basic auth secret for registry"
kubectl --kubeconfig "$kubeconfig" --server-side=true apply -f "$SCRIPT_DIR"/load-balancer/base/namespace.yaml
kubectl create secret generic -n registry registry-htpasswd --from-file="$SCRIPT_DIR"/htpasswd/auth --dry-run=client -o yaml | \
  kubectl --kubeconfig "$kubeconfig" --server-side=true apply  -f -
kubectl rollout restart statefulsets -n registry -l app=registry --kubeconfig "$kubeconfig"

echo "Creating pull secret in garden namespace"
kubectl apply -f "$SCRIPT_DIR"/../../00-namespace-garden.yaml --kubeconfig "$kubeconfig" --server-side=true
kubectl create secret docker-registry -n garden gardener-images --docker-server="$registry" --docker-username=gardener --docker-password="$password" --docker-email=gardener@localhost --dry-run=client -o yaml | \
  kubectl --kubeconfig "$kubeconfig" --server-side=true apply  -f -

echo "Deploying container registry $registry"
kubectl --kubeconfig "$kubeconfig" --server-side=true apply -f "$SCRIPT_DIR"/registry/registry.yaml

echo "Waiting max 5m until registry endpoint is available"
start_time=$(date +%s)
until [ "$(curl --write-out '%{http_code}' --silent --output /dev/null https://"$registry"/v2/)" -eq "401" ]; do
  elapsed_time=$(($(date +%s) - ${start_time}))
  if [ $elapsed_time -gt 300 ]; then
    echo "Timeout"
    exit 1
  fi
  sleep 1
done

echo "Run docker login for registry $registry"
docker login "$registry" -u gardener -p "$password"
