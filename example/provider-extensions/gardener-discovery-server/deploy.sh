#!/usr/bin/env bash
#
# SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

set -e

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"

usage() {
  echo "Usage:"
  echo "> deploy.sh [ -h | <garden_kubeconfig> <seed_kubeconfig> ]"
  echo
  echo ">> For example: deploy.sh ~/.kube/garden-kubeconfig.yaml ~/.kube/garden-kubeconfig.yaml"

  exit 0
}

if [ "$1" == "-h" ] || [ "$#" -ne 2 ]; then
  usage
fi

garden_kubeconfig=$1
seed_kubeconfig=$2

kubectl --server-side=true --kubeconfig "$garden_kubeconfig" apply -k "$SCRIPT_DIR"/rbac

token=$(kubectl --kubeconfig "$garden_kubeconfig" -n garden create token gardener-discovery-server --duration 12h)
kubectl --kubeconfig "$seed_kubeconfig" apply -f - << EOF
apiVersion: v1
kind: Secret
metadata:
  name: gardener-discovery-server-kubeconfig
  namespace: gardener-discovery-server
type: Opaque
stringData:
  kubeconfig: |
    apiVersion: v1
    kind: Config
    current-context: gardener-discovery-server
    clusters:
    - cluster:
        insecure-skip-tls-verify: true
        server: https://gardener-apiserver.relay.svc.cluster.local
      name: gardener-discovery-server
    contexts:
    - context:
        cluster: gardener-discovery-server
        user: gardener-discovery-server
      name: gardener-discovery-server
    users:
    - name: gardener-discovery-server
      user:
        token: $token
EOF

kubectl --server-side=true --kubeconfig "$seed_kubeconfig" apply -k "$SCRIPT_DIR"/server
