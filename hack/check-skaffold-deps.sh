#!/usr/bin/env bash
#
# Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

echo "> Check Skaffold Dependencies"

check_successful=true
repo_root="$(git rev-parse --show-toplevel)"

function check() {
  if ! $repo_root/hack/check-skaffold-deps-for-binary.sh --skaffold-file $1 --binary $2 --skaffold-config $3; then
    check_successful=false
  fi
}

# skaffold.yaml
check "skaffold.yaml" "gardener-admission-controller"      "controlplane"
check "skaffold.yaml" "gardener-apiserver"                 "controlplane"
check "skaffold.yaml" "gardener-controller-manager"        "controlplane"
check "skaffold.yaml" "gardener-extension-provider-local"  "provider-local"
check "skaffold.yaml" "gardener-resource-manager"          "gardenlet"
check "skaffold.yaml" "gardener-scheduler"                 "controlplane"
check "skaffold.yaml" "gardenlet"                          "gardenlet"

# skaffold-operator.yaml
check "skaffold-operator.yaml" "gardener-operator"             "gardener-operator"
check "skaffold-operator.yaml" "gardener-resource-manager"     "gardener-operator"
check "skaffold-operator.yaml" "gardener-admission-controller" "gardener-operator"
check "skaffold-operator.yaml" "gardener-apiserver"            "gardener-operator"
check "skaffold-operator.yaml" "gardener-controller-manager"   "gardener-operator"
check "skaffold-operator.yaml" "gardener-scheduler"            "gardener-operator"

if [ "$check_successful" = false ] ; then
  exit 1
fi
