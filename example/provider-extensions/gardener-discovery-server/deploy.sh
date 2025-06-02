#!/usr/bin/env bash
#
# SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

set -e

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"
REPO_ROOT_DIR="$(realpath "$SCRIPT_DIR"/../../..)"

usage() {
  echo "Usage:"
  echo "> ${0} [ -h | <garden_kubeconfig> <seed_kubeconfig> ]"
  echo
  echo ">> For example: ${0} ~/.kube/garden-kubeconfig.yaml ~/.kube/seed-kubeconfig.yaml"

  exit 0
}

if [ "$1" == "-h" ] || [ "$#" -ne 2 ]; then
  usage
fi

garden_kubeconfig=$1
seed_kubeconfig=$2

kubectl --server-side=true --kubeconfig "$garden_kubeconfig" apply -f "$SCRIPT_DIR"/rbac

token=$(kubectl --kubeconfig "$garden_kubeconfig" -n garden create token gardener-discovery-server --duration 48h)
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

workload_identity_key="$(kubectl --kubeconfig "$garden_kubeconfig" -n garden get secret gardener-apiserver-workload-identity-signing-key -o yaml | yq -r '.data."key.pem"' | base64 -d)"
gardener_issuer="$(cat "$REPO_ROOT_DIR"/example/provider-extensions/garden/controlplane/workload-identity-issuer.yaml | yq -r '.global.apiserver.workloadIdentity.token.issuer')"
oidc_config="$(oidcmeta config "$gardener_issuer" <<< "$workload_identity_key")"
jwks="$(oidcmeta jwks <<< "$workload_identity_key")"
kubectl --kubeconfig "$seed_kubeconfig" apply -f - << EOF
apiVersion: v1
kind: ConfigMap
metadata:
  name: gardener-discovery-server-workload-identity
  namespace: gardener-discovery-server
data:
  openid-configuration.json: |
    $oidc_config
  jwks.json: |
    $jwks
EOF

kubectl --server-side=true --kubeconfig "$seed_kubeconfig" apply -f "$SCRIPT_DIR"/server
