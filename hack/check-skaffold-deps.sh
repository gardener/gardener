#!/usr/bin/env bash
#
# SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

set -e

operation="${1:-check}"

echo "> ${operation} Skaffold Dependencies"

success=true
repo_root="$(git rev-parse --show-toplevel)"

function run() {
  if ! "$repo_root"/hack/check-skaffold-deps-for-binary.sh "$operation" --skaffold-file "$1" --binary "$2" --skaffold-config "$3"; then
    success=false
  fi
}

# skaffold.yaml
run "skaffold.yaml" "gardener-admission-controller"             "controlplane"
run "skaffold.yaml" "gardener-apiserver"                        "controlplane"
run "skaffold.yaml" "gardener-controller-manager"               "controlplane"
run "skaffold.yaml" "gardener-extension-provider-local"         "provider-local"
run "skaffold.yaml" "gardener-node-agent"                       "gardenlet"
run "skaffold.yaml" "gardener-resource-manager"                 "gardenlet"
run "skaffold.yaml" "gardener-scheduler"                        "controlplane"
run "skaffold.yaml" "gardenlet"                                 "gardenlet"
run "skaffold.yaml" "machine-controller-manager-provider-local" "provider-local"

# skaffold-operator.yaml
run "skaffold-operator.yaml" "gardener-admission-controller"             "gardener-operator"
run "skaffold-operator.yaml" "gardener-apiserver"                        "gardener-operator"
run "skaffold-operator.yaml" "gardener-controller-manager"               "gardener-operator"
run "skaffold-operator.yaml" "gardener-operator"                         "gardener-operator"
run "skaffold-operator.yaml" "gardener-resource-manager"                 "gardener-operator"
run "skaffold-operator.yaml" "gardener-scheduler"                        "gardener-operator"
run "skaffold-operator.yaml" "gardener-extension-provider-local"         "provider-local"
run "skaffold-operator.yaml" "machine-controller-manager-provider-local" "provider-local"
run "skaffold-operator.yaml" "gardener-extension-admission-local"        "provider-local"

# skaffold-operator-garden.yaml
run "skaffold-operator-garden.yaml" "gardener-node-agent"                 "gardenlet"
run "skaffold-operator-garden.yaml" "gardener-resource-manager"           "gardenlet"
run "skaffold-operator-garden.yaml" "gardenlet"                           "gardenlet"

# skaffold-gardenadm.yaml
run "skaffold-gardenadm.yaml" "gardenadm"                                 "gardenadm"
run "skaffold-gardenadm.yaml" "gardener-node-agent"                       "gardenadm"
run "skaffold-gardenadm.yaml" "gardener-resource-manager"                 "gardenadm"
run "skaffold-gardenadm.yaml" "gardener-extension-provider-local"         "provider-local"
run "skaffold-gardenadm.yaml" "machine-controller-manager-provider-local" "provider-local"

if ! $success ; then
  exit 1
fi
