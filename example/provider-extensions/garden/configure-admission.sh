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

for p in $(yq '. | select(.kind == "ControllerDeployment") | select(.metadata.name == "provider-*" or .metadata.name == "networking-*") | .metadata.name' "$REPO_ROOT_DIR"/example/provider-extensions/garden/controllerregistrations/* | grep -v -E "^---$"); do
  echo "Found \"$p\" in $REPO_ROOT_DIR/example/provider-extensions/garden/controllerregistrations. Trying to configure its admission controller..."
  if PROVIDER_RELEASES=$(curl --fail -s -L -H "Accept: application/vnd.github+json" "https://api.github.com/repos/gardener/gardener-extension-$p/releases"); then
    ADMISSION_NAME=$(sed -E 's/(provider|networking)/admission/g' <<< $p)
    RELEASE=$(jq -r '.[].tag_name' <<< "$PROVIDER_RELEASES" | head -n 1)
    echo "Identified \"$RELEASE\" as latest release of $ADMISSION_NAME."
    IMAGE=$(yq ". | select(.kind == \"ControllerDeployment\") | select(.metadata.name == \"$p\") | .helm.values.image" "$REPO_ROOT_DIR"/example/provider-extensions/garden/controllerregistrations/*)
    if [ -n "$(yq '.tag' <<< "$IMAGE")" ]; then
      CONTROLLER_DEPLOYMENT_RELEASE=$(yq '.tag' <<< "$IMAGE")
    else
      CONTROLLER_DEPLOYMENT_RELEASE=${IMAGE#*:}
    fi
    echo "Identified \"$CONTROLLER_DEPLOYMENT_RELEASE\" as currently used release of \"$p\" ControllerDeployment from its '.providerConfig.values.image.tag' field."
    if [[ $(jq -r "any(.[].tag_name; . == \"$CONTROLLER_DEPLOYMENT_RELEASE\")" <<< "$PROVIDER_RELEASES") == "true" ]]; then
      RELEASE=$CONTROLLER_DEPLOYMENT_RELEASE
    else
      echo "Could not find \"$CONTROLLER_DEPLOYMENT_RELEASE\" release in the set of github repository releases of \"$p\"."
      echo "Using latest release instead."
    fi
    echo "Trying to deploy $ADMISSION_NAME with $RELEASE ..."
    ADMISSION_GIT_ROOT=$(mktemp -d)
    ADMISSION_FILE=$(mktemp)
    curl --fail -L -o "$ADMISSION_FILE" "https://github.com/gardener/gardener-extension-$p/archive/refs/tags/$RELEASE.tar.gz"
    tar xfz "$ADMISSION_FILE" -C "$ADMISSION_GIT_ROOT" --strip-components 1
    ADMISSION_CHARTS_DIR="$ADMISSION_GIT_ROOT/charts/gardener-extension-$ADMISSION_NAME/charts"
    set +e
    grep -r '.Values.global.' "$ADMISSION_CHARTS_DIR" > /dev/null
    NEW_VALUES=$?
    set -e
    if [ $NEW_VALUES == 0 ]; then
      # Found .Values.global.* in the chart. Deploy it with "global" values...
      echo "Deploying $ADMISSION_NAME with deprecated global values..."
      helm template --namespace garden --set global.image.tag="$RELEASE" gardener-extension-"$ADMISSION_NAME" "$ADMISSION_CHARTS_DIR"/application > "$ADMISSION_GIT_ROOT"/virtual-resources.yaml
      helm template --namespace garden --set global.image.tag="$RELEASE" --set global.kubeconfig="$(cat "$garden_kubeconfig" | sed 's/127.0.0.1:.*$/kubernetes.default.svc.cluster.local/g')" --set global.vpa.enabled="false" gardener-extension-"$ADMISSION_NAME" "$ADMISSION_CHARTS_DIR"/runtime > "$ADMISSION_GIT_ROOT"/runtime-resources.yaml
    else
      # No .Values.global.* found in the chart. Deploy it with new values...
      echo "Deploying $ADMISSION_NAME with new values..."
      helm template --namespace garden --set image.tag="$RELEASE" --set gardener.virtualCluster.enabled="false" gardener-extension-"$ADMISSION_NAME" "$ADMISSION_CHARTS_DIR"/application > "$ADMISSION_GIT_ROOT"/virtual-resources.yaml
      helm template --namespace garden --set image.tag="$RELEASE" --set gardener.virtualCluster.enabled="false" --set kubeconfig="$(cat "$garden_kubeconfig" | sed 's/127.0.0.1:.*$/kubernetes.default.svc.cluster.local/g')" --set vpa.enabled="false" gardener-extension-"$ADMISSION_NAME" "$ADMISSION_CHARTS_DIR"/runtime > "$ADMISSION_GIT_ROOT"/runtime-resources.yaml
    fi
    kubectl --kubeconfig "$garden_kubeconfig" "$command" "$@" -f "$ADMISSION_GIT_ROOT/virtual-resources.yaml"
    kubectl --kubeconfig "$garden_kubeconfig" "$command" "$@" -f "$ADMISSION_GIT_ROOT/runtime-resources.yaml"
    if [ "$command" == "apply" ]; then
      kubectl --kubeconfig "$garden_kubeconfig" wait --for=condition=available deployment -l app.kubernetes.io/name=gardener-extension-"$ADMISSION_NAME" -n garden --timeout 5m
    fi
    rm -rf "$ADMISSION_FILE" "$ADMISSION_GIT_ROOT"
    echo "Successfully deployed $ADMISSION_NAME:$RELEASE."
  else
    echo "Github repository releases of \"$p\" not found."
  fi
done
