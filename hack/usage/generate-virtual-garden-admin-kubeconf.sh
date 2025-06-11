#!/usr/bin/env bash
#
# SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

set -euo pipefail

USER_NAME="admin-user"
CLUSTER_NAME="virtual-garden"

TMP_DIR="$(mktemp -d)"
KEY_FILE="${TMP_DIR}/${USER_NAME}.key"
CSR_FILE="${TMP_DIR}/${USER_NAME}.csr"
CRT_FILE="${TMP_DIR}/${USER_NAME}.crt"
CLUSTER_CA_CERT_FILE="${TMP_DIR}/cluster-ca.crt"
CLIENT_CA_CERT_FILE="${TMP_DIR}/client-ca.crt"
CLIENT_CA_KEY_FILE="${TMP_DIR}/client-ca.key"
KUBECONFIG_FILE="${TMP_DIR}/kubeconfig"

cp "$(dirname "$0")/../../example/gardener-local/kind/operator/kubeconfig" "${KUBECONFIG_FILE}"
kubectl --kubeconfig "${KUBECONFIG_FILE}" -n garden get secret -l name=ca        -o jsonpath='{..data.ca\.crt}' | base64 -d > "${CLUSTER_CA_CERT_FILE}"
kubectl --kubeconfig "${KUBECONFIG_FILE}" -n garden get secret -l name=ca-client -o jsonpath='{..data.ca\.crt}' | base64 -d > "${CLIENT_CA_CERT_FILE}"
kubectl --kubeconfig "${KUBECONFIG_FILE}" -n garden get secret -l name=ca-client -o jsonpath='{..data.ca\.key}' | base64 -d > "${CLIENT_CA_KEY_FILE}"
rm "${KUBECONFIG_FILE}"

openssl genrsa -out "$KEY_FILE" 2048 >/dev/null
openssl req -new -key "$KEY_FILE" -out "$CSR_FILE" -subj "/CN=${USER_NAME}/O=system:masters" >/dev/null
openssl x509 -req -in "$CSR_FILE" -CA "$CLIENT_CA_CERT_FILE" -CAkey "${CLIENT_CA_KEY_FILE}" -CAcreateserial -out "$CRT_FILE" -days 365 -extensions v3_req -extfile <(echo -e "[v3_req]\nkeyUsage=critical,digitalSignature,keyEncipherment\nextendedKeyUsage=clientAuth") 2>/dev/null

kubectl config --kubeconfig="${KUBECONFIG_FILE}" set-cluster "${CLUSTER_NAME}" \
  --server="https://api.virtual-garden.local.gardener.cloud" \
  --certificate-authority=<(cat "${CLUSTER_CA_CERT_FILE}") \
  --embed-certs=true >/dev/null
kubectl config --kubeconfig="${KUBECONFIG_FILE}" set-credentials "${USER_NAME}" \
  --client-key="$KEY_FILE" \
  --client-certificate="$CRT_FILE" \
  --embed-certs=true >/dev/null
kubectl config --kubeconfig="${KUBECONFIG_FILE}" set-context "${USER_NAME}-context" \
  --cluster="${CLUSTER_NAME}" \
  --user="${USER_NAME}" >/dev/null
kubectl config --kubeconfig="${KUBECONFIG_FILE}" use-context "${USER_NAME}-context" >/dev/null

cat "${KUBECONFIG_FILE}"
