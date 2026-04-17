#!/usr/bin/env bash
#
# SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

usage() {
  cat <<EOF
Usage: $(basename "$0") <command> [flags]

Commands:
  virtual-garden (vg)       Generate admin kubeconfig for the virtual garden (client certificate)
  self-hosted-shoot (shs)   Generate admin kubeconfig for a self-hosted shoot (client certificate)
  shoot                     Generate admin/viewer kubeconfig for a hosted shoot (Gardener API)

If no command is given, defaults to 'shoot'.

Run '$(basename "$0") <command> --help' for command-specific flags.
EOF
  exit "${1:-1}"
}

# --- shoot (default) -----------------------------------------------------------

cmd_shoot() {
  local namespace="garden-local"
  local shoot_name="local"
  local kubeconfig_type="admin"

  while test $# -gt 0; do
    case "$1" in
      --namespace)
        shift; namespace="${1:-$namespace}" ;;
      --shoot-name)
        shift; shoot_name="${1:-$shoot_name}" ;;
      --admin)
        kubeconfig_type="admin" ;;
      --viewer)
        kubeconfig_type="viewer" ;;
      --help|-h)
        cat <<EOF
Usage: $(basename "$0") shoot [flags]

Generate admin/viewer kubeconfig for a hosted shoot via the Gardener API.

Flags:
  --namespace <ns>      Shoot namespace (default: garden-local)
  --shoot-name <name>   Shoot name (default: local)
  --admin               Generate admin kubeconfig (default)
  --viewer              Generate viewer kubeconfig
EOF
        exit 0 ;;
      *)
        echo "Unknown argument: $1" >&2; exit 1 ;;
    esac
    shift
  done

  if [[ "${kubeconfig_type}" == "admin" ]]; then
    local endpoint="adminkubeconfig"
    local kind="AdminKubeconfigRequest"
  else
    local endpoint="viewerkubeconfig"
    local kind="ViewerKubeconfigRequest"
  fi

  cat <<EOF | kubectl create --raw /apis/core.gardener.cloud/v1beta1/namespaces/"${namespace}"/shoots/"${shoot_name}"/"${endpoint}" -f - | jq -r '.status.kubeconfig' | base64 -d
{
    "apiVersion": "authentication.gardener.cloud/v1alpha1",
    "kind": "${kind}",
    "spec": {
        "expirationSeconds": 3600
    }
}
EOF
}

# --- virtual-garden ------------------------------------------------------------

cmd_virtual_garden() {
  local garden_name="${GARDEN_NAME:-local}"
  local runtime_kubeconfig="${RUNTIME_CLUSTER_KUBECONFIG:-${SCRIPT_DIR}/../../dev-setup/kubeconfigs/runtime/kubeconfig}"

  while test $# -gt 0; do
    case "$1" in
      --garden-name)
        shift; garden_name="${1:-$garden_name}" ;;
      --runtime-kubeconfig)
        shift; runtime_kubeconfig="${1:-$runtime_kubeconfig}" ;;
      --help|-h)
        cat <<EOF
Usage: $(basename "$0") virtual-garden [flags]

Generate admin kubeconfig for the virtual garden using a client certificate
signed by the cluster's client CA.

Flags:
  --garden-name <name>            Garden name (default: local, env: GARDEN_NAME)
  --runtime-kubeconfig <path>     Path to runtime cluster kubeconfig
                                  (default: dev-setup/kubeconfigs/runtime/kubeconfig,
                                   env: RUNTIME_CLUSTER_KUBECONFIG)
EOF
        exit 0 ;;
      *)
        echo "Unknown argument: $1" >&2; exit 1 ;;
    esac
    shift
  done

  generate_client_cert_kubeconfig "virtual-garden" \
    "$(get_virtual_garden_certs "${runtime_kubeconfig}" "${garden_name}")"
}

# --- self-hosted-shoot ---------------------------------------------------------

cmd_self_hosted_shoot() {
  local shoot_namespace="${SHOOT_NAMESPACE:-garden}"
  local shoot_name="${SHOOT_NAME:-root}"

  while test $# -gt 0; do
    case "$1" in
      --shoot-namespace)
        shift; shoot_namespace="${1:-$shoot_namespace}" ;;
      --shoot-name)
        shift; shoot_name="${1:-$shoot_name}" ;;
      --help|-h)
        cat <<EOF
Usage: $(basename "$0") self-hosted-shoot [flags]

Generate admin kubeconfig for a self-hosted shoot using a client certificate
signed by the cluster's client CA.

Flags:
  --shoot-namespace <ns>    Shoot namespace (default: garden, env: SHOOT_NAMESPACE)
  --shoot-name <name>       Shoot name (default: root, env: SHOOT_NAME)
EOF
        exit 0 ;;
      *)
        echo "Unknown argument: $1" >&2; exit 1 ;;
    esac
    shift
  done

  generate_client_cert_kubeconfig "self-hosted-shoot--${shoot_namespace}--${shoot_name}" \
    "$(get_self_hosted_shoot_certs "${shoot_namespace}" "${shoot_name}")"
}

# --- shared helpers for client-cert kubeconfigs --------------------------------

get_virtual_garden_certs() {
  local runtime_kubeconfig="$1"
  local garden_name="$2"

  local tmp_dir
  tmp_dir="$(mktemp -d)"

  local cluster_ca_cert="${tmp_dir}/cluster-ca.crt"
  local client_ca_cert="${tmp_dir}/client-ca.crt"
  local client_ca_key="${tmp_dir}/client-ca.key"

  local tmp_kubeconfig="${tmp_dir}/kubeconfig"
  cp "$runtime_kubeconfig" "$tmp_kubeconfig"

  kubectl --kubeconfig "$tmp_kubeconfig" -n garden get secret -l name=ca        -o jsonpath='{..data.ca\.crt}' | base64 -d > "$cluster_ca_cert"
  kubectl --kubeconfig "$tmp_kubeconfig" -n garden get secret -l name=ca-client -o jsonpath='{..data.ca\.crt}' | base64 -d > "$client_ca_cert"
  kubectl --kubeconfig "$tmp_kubeconfig" -n garden get secret -l name=ca-client -o jsonpath='{..data.ca\.key}' | base64 -d > "$client_ca_key"

  local dns_domain
  dns_domain=$(kubectl --kubeconfig "$tmp_kubeconfig" get gardens "$garden_name" -o yaml | yq '.spec.virtualCluster.dns.domains[0].name')

  echo "${tmp_dir}:${dns_domain}"
}

get_self_hosted_shoot_certs() {
  local shoot_namespace="$1"
  local shoot_name="$2"

  local tmp_dir
  tmp_dir="$(mktemp -d)"

  local cluster_ca_cert="${tmp_dir}/cluster-ca.crt"
  local client_ca_cert="${tmp_dir}/client-ca.crt"
  local client_ca_key="${tmp_dir}/client-ca.key"

  local machine_pod_ref
  machine_pod_ref="$(kubectl get pod -A -l app=machine -o jsonpath='{.items[0].metadata.namespace}/{.items[0].metadata.name}')"
  local machine_namespace="${machine_pod_ref%%/*}"
  local machine_pod="${machine_pod_ref##*/}"

  remote_kubectl() {
    kubectl -n "${machine_namespace}" exec "${machine_pod}" -- kubectl --kubeconfig /etc/kubernetes/admin.conf "$@"
  }

  remote_kubectl -n kube-system get secret -l name=ca        -o jsonpath='{..data.ca\.crt}' | base64 -d > "$cluster_ca_cert"
  remote_kubectl -n kube-system get secret -l name=ca-client -o jsonpath='{..data.ca\.crt}' | base64 -d > "$client_ca_cert"
  remote_kubectl -n kube-system get secret -l name=ca-client -o jsonpath='{..data.ca\.key}' | base64 -d > "$client_ca_key"

  local dns_domain
  dns_domain=$(remote_kubectl config view --minify -o jsonpath='{.clusters[0].cluster.server}' | sed 's|https://api\.||')

  echo "${tmp_dir}:${dns_domain}"
}

generate_client_cert_kubeconfig() {
  local cluster_name="$1"
  local cert_info="$2"

  local tmp_dir="${cert_info%%:*}"
  local dns_domain="${cert_info#*:}"

  local user_name="admin-user"
  local key_file="${tmp_dir}/${user_name}.key"
  local csr_file="${tmp_dir}/${user_name}.csr"
  local crt_file="${tmp_dir}/${user_name}.crt"
  local cluster_ca_cert="${tmp_dir}/cluster-ca.crt"
  local client_ca_cert="${tmp_dir}/client-ca.crt"
  local client_ca_key="${tmp_dir}/client-ca.key"
  local kubeconfig_file="${tmp_dir}/kubeconfig"

  openssl genrsa -out "$key_file" 2048 >/dev/null
  openssl req -new -key "$key_file" -out "$csr_file" -subj "/CN=${user_name}/O=system:masters" >/dev/null
  openssl x509 -req -in "$csr_file" -CA "$client_ca_cert" -CAkey "$client_ca_key" -CAcreateserial -out "$crt_file" -days 365 -extensions v3_req -extfile <(echo -e "[v3_req]\nkeyUsage=critical,digitalSignature,keyEncipherment\nextendedKeyUsage=clientAuth") 2>/dev/null

  kubectl config --kubeconfig="$kubeconfig_file" set-cluster "$cluster_name" \
    --server="https://api.$dns_domain" \
    --certificate-authority=<(cat "$cluster_ca_cert") \
    --embed-certs=true >/dev/null
  kubectl config --kubeconfig="$kubeconfig_file" set-credentials "$user_name" \
    --client-key="$key_file" \
    --client-certificate="$crt_file" \
    --embed-certs=true >/dev/null
  kubectl config --kubeconfig="$kubeconfig_file" set-context "$cluster_name" \
    --cluster="$cluster_name" \
    --user="$user_name" >/dev/null
  kubectl config --kubeconfig="$kubeconfig_file" use-context "$cluster_name" >/dev/null

  cat "$kubeconfig_file"
}

# --- main dispatch -------------------------------------------------------------

# Default to 'shoot' when the first argument looks like a flag or is absent.
command="${1:-shoot}"
case "$command" in
  virtual-garden|vg)
    shift; cmd_virtual_garden "$@" ;;
  self-hosted-shoot|shs)
    shift; cmd_self_hosted_shoot "$@" ;;
  shoot)
    shift; cmd_shoot "$@" ;;
  --help|-h)
    usage 0 ;;
  --*)
    # Flags without a subcommand → default to 'shoot'
    cmd_shoot "$@" ;;
  *)
    echo "Unknown command: $command" >&2
    usage 1 ;;
esac
