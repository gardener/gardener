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

REPO_ROOT="$(git rev-parse --show-toplevel)"

# skaffold.yaml
$REPO_ROOT/hack/check-skaffold-deps-for-binary.sh --skaffold-file "skaffold.yaml" --binary "gardener-admission-controller"     --skaffold-config "controlplane"
$REPO_ROOT/hack/check-skaffold-deps-for-binary.sh --skaffold-file "skaffold.yaml" --binary "gardener-apiserver"                --skaffold-config "controlplane"
$REPO_ROOT/hack/check-skaffold-deps-for-binary.sh --skaffold-file "skaffold.yaml" --binary "gardener-controller-manager"       --skaffold-config "controlplane"
$REPO_ROOT/hack/check-skaffold-deps-for-binary.sh --skaffold-file "skaffold.yaml" --binary "gardener-extension-provider-local" --skaffold-config "provider-local"
$REPO_ROOT/hack/check-skaffold-deps-for-binary.sh --skaffold-file "skaffold.yaml" --binary "gardener-resource-manager"         --skaffold-config "gardenlet"
$REPO_ROOT/hack/check-skaffold-deps-for-binary.sh --skaffold-file "skaffold.yaml" --binary "gardener-scheduler"                --skaffold-config "controlplane"
$REPO_ROOT/hack/check-skaffold-deps-for-binary.sh --skaffold-file "skaffold.yaml" --binary "gardenlet"                         --skaffold-config "gardenlet"

# skaffold-operator.yaml
$REPO_ROOT/hack/check-skaffold-deps-for-binary.sh --skaffold-file "skaffold-operator.yaml" --binary "gardener-operator"             --skaffold-config "gardener-operator"
$REPO_ROOT/hack/check-skaffold-deps-for-binary.sh --skaffold-file "skaffold-operator.yaml" --binary "gardener-resource-manager"     --skaffold-config "gardener-operator"
$REPO_ROOT/hack/check-skaffold-deps-for-binary.sh --skaffold-file "skaffold-operator.yaml" --binary "gardener-admission-controller" --skaffold-config "gardener-operator"
$REPO_ROOT/hack/check-skaffold-deps-for-binary.sh --skaffold-file "skaffold-operator.yaml" --binary "gardener-apiserver"            --skaffold-config "gardener-operator"
$REPO_ROOT/hack/check-skaffold-deps-for-binary.sh --skaffold-file "skaffold-operator.yaml" --binary "gardener-controller-manager"   --skaffold-config "gardener-operator"
$REPO_ROOT/hack/check-skaffold-deps-for-binary.sh --skaffold-file "skaffold-operator.yaml" --binary "gardener-scheduler"            --skaffold-config "gardener-operator"
