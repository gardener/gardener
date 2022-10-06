#!/usr/bin/env bash
#
# Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

CA_NAME=quic-tunnel-ca
SERVER_NAME=quic-tunnel-server
CLIENT_NAME=quic-tunnel-client

usage() {
  echo "Usage:"
  echo "> deploy-quic-relay.sh [ -h | <garden-kubeconfig> <seed-kubeconfig> <relay domain> ]"
  echo
  echo ">> For example: deploy-quic-relay.sh ~/.kube/garden-kubeconfig.yaml ~/.kube/kubeconfig.yaml quic.gardener.cloud"

  exit 0
}

if [ "$1" == "-h" ] || [ "$#" -ne 3 ]; then
  usage
fi

garden_kubeconfig=$1
seed_kubeconfig=$2
relay_domain=$3

echo "Ensure namespace"
kubectl --kubeconfig "$seed_kubeconfig" --server-side=true apply  -f "$SCRIPT_DIR"/load-balancer/base/namespace.yaml
kubectl --kubeconfig "$garden_kubeconfig" --server-side=true apply  -f "$SCRIPT_DIR"/load-balancer/base/namespace.yaml

echo "Creating certs"
mkdir -p "$SCRIPT_DIR/certs"
"$SCRIPT_DIR"/../../../hack/local-development/remote-garden/generate-certs $CA_NAME $SERVER_NAME $CLIENT_NAME "$SCRIPT_DIR/certs"

echo "Creating cert secret on seed"
kubectl create secret generic -n relay quic-tls --from-file=ca.crt="$SCRIPT_DIR"/certs/$CA_NAME.crt --from-file=tls.crt="$SCRIPT_DIR"/certs/$SERVER_NAME.crt --from-file=tls.key="$SCRIPT_DIR"/certs/$SERVER_NAME.key --dry-run=client -o yaml | \
  kubectl --kubeconfig "$seed_kubeconfig" --server-side=true apply  -f -
kubectl rollout restart deployment -n relay -l app=gardener-api-quic-server --kubeconfig "$seed_kubeconfig"

echo "Creating cert secret on garden"
kubectl create secret generic -n relay quic-tls --from-file=ca.crt="$SCRIPT_DIR"/certs/$CA_NAME.crt --from-file=tls.crt="$SCRIPT_DIR"/certs/$CLIENT_NAME.crt --from-file=tls.key="$SCRIPT_DIR"/certs/$CLIENT_NAME.key --dry-run=client -o yaml | \
  kubectl --kubeconfig "$garden_kubeconfig" --server-side=true apply  -f -
kubectl rollout restart deployment -n relay -l app=gardener-api-quic-client --kubeconfig "$garden_kubeconfig"

echo "Deploying quic server to seed at $relay_domain"
sed "s/\$(RELAY_DOMAIN)/$relay_domain/g" "$SCRIPT_DIR"/quic-server/quic-relay-server.yaml | kubectl --kubeconfig "$seed_kubeconfig" --server-side=true apply -f -

echo "Deploying quic client to garden"
sed "s/\$(RELAY_DOMAIN)/$relay_domain/g" "$SCRIPT_DIR"/quic-client/quic-relay-client.yaml | kubectl --kubeconfig "$garden_kubeconfig" --server-side=true apply -f -
