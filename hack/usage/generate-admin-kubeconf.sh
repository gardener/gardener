#!/usr/bin/env bash
#
# SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

set -e

namespace="garden-local"
shoot_name="local"

parse_flags() {
  while test $# -gt 0; do
    case "$1" in
      --namespace)
        shift
        namespace="${1:-$namespace}"
        ;;
      --shoot-name)
        shift
        shoot_name="${1:-$shoot_name}"
        ;;
      *)
        echo "Unknown argument: $1"
        exit 1
        ;;
    esac
    shift
  done
}

parse_flags "$@"

cat << EOF | kubectl create --raw /apis/core.gardener.cloud/v1beta1/namespaces/"${namespace}"/shoots/"${shoot_name}"/adminkubeconfig -f - | jq -r '.status.kubeconfig' | base64 -d
{
    "apiVersion": "authentication.gardener.cloud/v1alpha1",
    "kind": "AdminKubeconfigRequest",
    "spec": {
        "expirationSeconds": 3600
    }
}
EOF
