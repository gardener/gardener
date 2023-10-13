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

skaffold_file=""
binary_name=""
skaffold_config_name=""

parse_flags() {
  while test $# -gt 0; do
    case "$1" in
      --skaffold-file)
        shift; skaffold_file="$1"
        ;;
      --binary)
        shift; binary_name="$1"
        ;;
      --skaffold-config)
        shift; skaffold_config_name="$1"
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

out_dir=$(mktemp -d)
function cleanup_output {
  rm -rf "$out_dir"
}
trap cleanup_output EXIT

repo_root="$(git rev-parse --show-toplevel)"
skaffold_yaml="$(cat "$repo_root/$skaffold_file")"

path_current_skaffold_dependencies="${out_dir}/current-$skaffold_file-deps-$binary_name.txt"
path_actual_dependencies="${out_dir}/actual-$skaffold_file-deps-$binary_name.txt"

echo "$skaffold_yaml" |\
  yq eval "select(.metadata.name == \"$skaffold_config_name\") | .build.artifacts[] | select(.ko.main == \"./cmd/$binary_name\") | .ko.dependencies.paths[]?" - |\
  sort |\
  uniq > "$path_current_skaffold_dependencies"

module_name=$(go list -m)
module_prefix="$module_name/"
go list -f '{{ join .Deps "\n" }}' "./cmd/$binary_name" |\
  grep "$module_prefix" |\
  sed "s@$module_prefix@@g" |\
  sort |\
  uniq > "$path_actual_dependencies"

# always add vendor directory and VERSION file
echo "vendor" >> "$path_actual_dependencies"
echo "VERSION" >> "$path_actual_dependencies"

# sort dependencies
sort -o $path_current_skaffold_dependencies{,}
sort -o $path_actual_dependencies{,}

echo -n ">> Checking defined dependencies in Skaffold config '$skaffold_config_name' for '$binary_name' in '$skaffold_file'..."
if ! diff="$(diff "$path_current_skaffold_dependencies" "$path_actual_dependencies")"; then
  echo
  echo ">>> The following actual dependencies are missing in $skaffold_file (need to be added):"
  echo "$diff" | grep '>' | awk '{print $2}'
  echo
  echo ">>> The following dependencies defined in $skaffold_file are not needed actually (need to be removed):"
  echo "$diff" | grep '<' | awk '{print $2}'
  echo

  exit 1
else
  echo " success."
fi
