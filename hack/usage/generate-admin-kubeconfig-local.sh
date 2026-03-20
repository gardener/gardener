#!/usr/bin/env bash
#
# SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

set -euo pipefail

MODE="${1:-}"

usage() {
  echo "Usage: $0 {virtual-garden|self-hosted-shoot}"
  exit 1
}

if [[ "$MODE" != "virtual-garden" && "$MODE" != "self-hosted-shoot" ]]; then
  usage
fi

USER_NAME="admin-user"

TMP_DIR="$(mktemp -d)"
KEY_FILE="${TMP_DIR}/${USER_NAME}.key"
CSR_FILE="${TMP_DIR}/${USER_NAME}.csr"
CRT_FILE="${TMP_DIR}/${USER_NAME}.crt"
CLUSTER_CA_CERT_FILE="${TMP_DIR}/cluster-ca.crt"
CLIENT_CA_CERT_FILE="${TMP_DIR}/client-ca.crt"
CLIENT_CA_KEY_FILE="${TMP_DIR}/client-ca.key"
KUBECONFIG_FILE="${TMP_DIR}/kubeconfig"

if [[ "$MODE" == "virtual-garden" ]]; then
  CLUSTER_NAME="virtual-garden"

  # TODO(rfranzke): Remove this once we store the kind/runtime cluster kubeconfig at a fixed location.
  RUNTIME_CLUSTER_KUBECONFIG="${RUNTIME_CLUSTER_KUBECONFIG:-"$(dirname "$0")/../../dev-setup/kubeconfigs/runtime/kubeconfig"}"
  GARDEN_NAME="${GARDEN_NAME:-local}"

  cp "$RUNTIME_CLUSTER_KUBECONFIG" "${KUBECONFIG_FILE}"
  kubectl --kubeconfig "${KUBECONFIG_FILE}" -n garden get secret -l name=ca        -o jsonpath='{..data.ca\.crt}' | base64 -d > "${CLUSTER_CA_CERT_FILE}"
  kubectl --kubeconfig "${KUBECONFIG_FILE}" -n garden get secret -l name=ca-client -o jsonpath='{..data.ca\.crt}' | base64 -d > "${CLIENT_CA_CERT_FILE}"
  kubectl --kubeconfig "${KUBECONFIG_FILE}" -n garden get secret -l name=ca-client -o jsonpath='{..data.ca\.key}' | base64 -d > "${CLIENT_CA_KEY_FILE}"
  DNS_DOMAIN=$(kubectl --kubeconfig "${KUBECONFIG_FILE}" get gardens "${GARDEN_NAME}" -o yaml | yq '.spec.virtualCluster.dns.domains[0].name')
  rm "${KUBECONFIG_FILE}"

elif [[ "$MODE" == "self-hosted-shoot" ]]; then
  SHOOT_NAMESPACE="${SHOOT_NAMESPACE:-garden}"
  SHOOT_NAME="${SHOOT_NAME:-root}"
  CLUSTER_NAME="self-hosted-shoot--${SHOOT_NAMESPACE}--${SHOOT_NAME}"

  MACHINE_POD="$(kubectl get pod -A -l app=machine -o jsonpath='{.items[0].metadata.namespace}/{.items[0].metadata.name}')"
  MACHINE_NAMESPACE="${MACHINE_POD%%/*}"
  MACHINE_POD="${MACHINE_POD##*/}"

  remote_kubectl() {
    kubectl -n "${MACHINE_NAMESPACE}" exec "${MACHINE_POD}" -- kubectl --kubeconfig /etc/kubernetes/admin.conf "$@"
  }

  remote_kubectl -n kube-system get secret -l name=ca        -o jsonpath='{..data.ca\.crt}' | base64 -d > "${CLUSTER_CA_CERT_FILE}"
  remote_kubectl -n kube-system get secret -l name=ca-client -o jsonpath='{..data.ca\.crt}' | base64 -d > "${CLIENT_CA_CERT_FILE}"
  remote_kubectl -n kube-system get secret -l name=ca-client -o jsonpath='{..data.ca\.key}' | base64 -d > "${CLIENT_CA_KEY_FILE}"
  DNS_DOMAIN=$(remote_kubectl config view --minify -o jsonpath='{.clusters[0].cluster.server}' | sed 's|https://api\.||')
fi

openssl genrsa -out "$KEY_FILE" 2048 >/dev/null
openssl req -new -key "$KEY_FILE" -out "$CSR_FILE" -subj "/CN=${USER_NAME}/O=system:masters" >/dev/null
openssl x509 -req -in "$CSR_FILE" -CA "$CLIENT_CA_CERT_FILE" -CAkey "${CLIENT_CA_KEY_FILE}" -CAcreateserial -out "$CRT_FILE" -days 365 -extensions v3_req -extfile <(echo -e "[v3_req]\nkeyUsage=critical,digitalSignature,keyEncipherment\nextendedKeyUsage=clientAuth") 2>/dev/null

kubectl config --kubeconfig="${KUBECONFIG_FILE}" set-cluster "${CLUSTER_NAME}" \
  --server="https://api.$DNS_DOMAIN" \
  --certificate-authority=<(cat "${CLUSTER_CA_CERT_FILE}") \
  --embed-certs=true >/dev/null
kubectl config --kubeconfig="${KUBECONFIG_FILE}" set-credentials "${USER_NAME}" \
  --client-key="$KEY_FILE" \
  --client-certificate="$CRT_FILE" \
  --embed-certs=true >/dev/null
kubectl config --kubeconfig="${KUBECONFIG_FILE}" set-context "${CLUSTER_NAME}" \
  --cluster="${CLUSTER_NAME}" \
  --user="${USER_NAME}" >/dev/null
kubectl config --kubeconfig="${KUBECONFIG_FILE}" use-context "${CLUSTER_NAME}" >/dev/null

cat "${KUBECONFIG_FILE}"
