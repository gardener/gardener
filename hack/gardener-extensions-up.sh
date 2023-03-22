#!/usr/bin/env bash
# Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.


set -o errexit
set -o nounset
set -o pipefail

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"

SEED_NAME=""
PATH_SEED_KUBECONFIG=""
PATH_GARDEN_KUBECONFIG=""

parse_flags() {
  while test $# -gt 0; do
    case "$1" in
    --seed-name)
      shift; SEED_NAME="$1"
      ;;
    --path-seed-kubeconfig)
      shift; PATH_SEED_KUBECONFIG="$1"
      ;;
    --path-garden-kubeconfig)
      shift; PATH_GARDEN_KUBECONFIG="$1"
      ;;
    esac

    shift
  done
}

parse_flags "$@"

echo "Configure seed cluster"
"$SCRIPT_DIR"/../example/provider-extensions/seed/configure-seed.sh "$PATH_GARDEN_KUBECONFIG" "$PATH_SEED_KUBECONFIG" "$SEED_NAME"
echo "Start bootstrapping Gardener"
SKAFFOLD_DEFAULT_REPO=localhost:5001 SKAFFOLD_PUSH=true skaffold run -m etcd,controlplane,extensions-env -p extensions
echo "Registering controllers"
kubectl --kubeconfig "$PATH_GARDEN_KUBECONFIG" --server-side=true --force-conflicts=true apply -f "$SCRIPT_DIR"/../example/provider-extensions/garden/controllerregistrations
echo "Creating CloudProfiles"
kubectl --kubeconfig "$PATH_GARDEN_KUBECONFIG" --server-side=true apply -f "$SCRIPT_DIR"/../example/provider-extensions/garden/cloudprofiles
"$SCRIPT_DIR"/../example/provider-extensions/seed/create-seed.sh "$PATH_GARDEN_KUBECONFIG" "$PATH_SEED_KUBECONFIG" "$SEED_NAME"
