#!/usr/bin/env bash
#
# Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

out_dir=$(mktemp -d)
function cleanup_output {
  rm -rf "$out_dir"
}
trap cleanup_output EXIT

declare -A binary_to_skaffold_config_name
binary_to_skaffold_config_name["gardener-admission-controller"]="controlplane"
binary_to_skaffold_config_name["gardener-apiserver"]="controlplane"
binary_to_skaffold_config_name["gardener-controller-manager"]="controlplane"
binary_to_skaffold_config_name["gardener-extension-provider-local"]="provider-local"
binary_to_skaffold_config_name["gardener-resource-manager"]="gardenlet"
binary_to_skaffold_config_name["gardener-seed-admission-controller"]="gardenlet"
binary_to_skaffold_config_name["gardener-scheduler"]="controlplane"
binary_to_skaffold_config_name["gardenlet"]="gardenlet"

skaffold_yaml="$(cat "$(dirname "$0")/../skaffold.yaml")"

for key in "${!binary_to_skaffold_config_name[@]}"; do
  binary_name="$key"
  skaffold_config_name="${binary_to_skaffold_config_name[${key}]}"

  path_current_skaffold_dependencies="${out_dir}/current-skaffold-deps-$binary_name.txt"
  path_actual_dependencies="${out_dir}/actual-deps-$binary_name.txt"

  echo "$skaffold_yaml" |\
    yq eval "select(.metadata.name == \"$skaffold_config_name\") | .build.artifacts[] | select(.ko.main == \"./cmd/$binary_name\") | .ko.dependencies.paths[]?" - |\
    sort |\
    uniq > "$path_current_skaffold_dependencies"

  go list -f '{{ join .Deps "\n" }}' "./cmd/$binary_name" |\
    grep "github.com/gardener/gardener/" |\
    sed 's/github\.com\/gardener\/gardener\///g' |\
    sort |\
    uniq > "$path_actual_dependencies"

  # always add vendor directory
  echo "vendor" >> "$path_actual_dependencies"

  echo -n ">> Checking defined dependencies in Skaffold config '$skaffold_config_name' for '$binary_name'..."
  if ! diff="$(diff "$path_current_skaffold_dependencies" "$path_actual_dependencies")"; then
    check_successful=false

    echo
    echo ">>> The following actual dependencies are missing in skaffold.yaml (need to be added):"
    echo "$diff" | grep '>' | awk '{print $2}'
    echo
    echo ">>> The following dependencies defined in skaffold.yaml are not needed actually (need to be removed):"
    echo "$diff" | grep '<' | awk '{print $2}'
    echo
  else
    echo " success."
  fi
done

if [ "$check_successful" = false ] ; then
  exit 1
fi
