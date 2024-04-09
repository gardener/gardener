#!/usr/bin/env bash
#
# Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#      http://www.apache.org/licenses~LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -e

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"
REPO_ROOT_DIR="$(realpath "$SCRIPT_DIR"/../../..)"

usage() {
  echo "Usage:"
  echo "> configure-admission.sh [ -h | <garden-kubeconfig> <apply|delete> ] [kubectl options]"
  echo
  echo ">> For example: configure-admission.sh ~/.kube/garden-kubeconfig.yaml delete --ignore-not-found"

  exit 0
}

if [ "$1" == "-h" ]; then
  usage
fi

if [ "$2" != "apply" ] && [ "$2" != "delete" ]; then
  usage
fi

garden_kubeconfig=$1
command=$2
shift 2

for p in $(yq '. | select(.kind == "ControllerDeployment") | select(.metadata.name == "provider-*" or .metadata.name == "networking-cilium") | .metadata.name' "$REPO_ROOT_DIR"/example/provider-extensions/garden/controllerregistrations/* | grep -v -E "^---$"); do
  echo "Found \"$p\" in $REPO_ROOT_DIR/example/provider-extensions/garden/controllerregistrations. Trying to configure its admission controller..."
  if PROVIDER_RELEASES=$(curl --fail -s -L -H "Accept: application/vnd.github+json" "https://api.github.com/repos/gardener/gardener-extension-$p/releases"); then
    LATEST_RELEASE=$(jq -r '.[].tag_name' <<< "$PROVIDER_RELEASES" | head -n 1)
    ADMISSION_NAME=${p/provider/admission}
    if [ "$ADMISSION_NAME" == "networking-cilium" ]; then
      ADMISSION_NAME="admission-cilium"
    fi
    echo "Identified $LATEST_RELEASE as latest release of $ADMISSION_NAME. Trying to deploy it..."
    ADMISSION_GIT_ROOT=$(mktemp -d)
    ADMISSION_FILE=$(mktemp)
    curl --fail -L -o "$ADMISSION_FILE" "https://github.com/gardener/gardener-extension-$p/archive/refs/tags/$LATEST_RELEASE.tar.gz"
    tar xfz "$ADMISSION_FILE" -C "$ADMISSION_GIT_ROOT" --strip-components 1
    helm template --namespace garden --set global.image.tag="$LATEST_RELEASE" gardener-extension-"$ADMISSION_NAME" "$ADMISSION_GIT_ROOT"/charts/gardener-extension-"$ADMISSION_NAME"/charts/application > "$ADMISSION_GIT_ROOT"/virtual-resources.yaml
    helm template --namespace garden --set global.image.tag="$LATEST_RELEASE" --set global.kubeconfig="$(cat "$garden_kubeconfig" | sed 's/127.0.0.1:.*$/kubernetes.default.svc.cluster.local/g')" --set global.vpa.enabled="false" gardener-extension-"$ADMISSION_NAME" "$ADMISSION_GIT_ROOT"/charts/gardener-extension-"$ADMISSION_NAME"/charts/runtime > "$ADMISSION_GIT_ROOT"/runtime-resources.yaml
    kubectl --kubeconfig "$garden_kubeconfig" "$command" "$@" -f "$ADMISSION_GIT_ROOT/virtual-resources.yaml"
    kubectl --kubeconfig "$garden_kubeconfig" "$command" "$@" -f "$ADMISSION_GIT_ROOT/runtime-resources.yaml"
    if [ "$command" == "apply" ]; then
      kubectl --kubeconfig "$garden_kubeconfig" wait --for=condition=available deployment -l app.kubernetes.io/name=gardener-extension-"$ADMISSION_NAME" -n garden --timeout 5m
    fi
    rm -rf "$ADMISSION_FILE" "$ADMISSION_GIT_ROOT"
    echo "Successfully deployed $ADMISSION_NAME:$LATEST_RELEASE."
  else
    echo "Github repository releases of \"$p\" not found."
  fi
done
