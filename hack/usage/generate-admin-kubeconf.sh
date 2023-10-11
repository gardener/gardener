#!/usr/bin/env bash
#
# Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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