#!/usr/bin/env bash
#
# SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

set -e

skaffold_file=""
binary_name=""
skaffold_config_name=""

parse_flags() {
  operation="$1"
  shift

  while test $# -gt 1; do
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
  sort -f |\
  uniq > "$path_current_skaffold_dependencies"

echo "cmd/$binary_name" > "$path_actual_dependencies"
module_name=$(go list -m)
module_prefix="$module_name/"
go list -f '{{ join .Deps "\n" }}' "./cmd/$binary_name" |\
  grep "$module_prefix" |\
  sed "s@$module_prefix@@g" |\
  sort -f |\
  uniq >> "$path_actual_dependencies"

# always add VERSION file
echo "VERSION" >> "$path_actual_dependencies"
# add vendor if the vendor/ dir exists
if [[ -d "$repo_root/vendor" ]]; then
  echo "vendor" >> "$path_actual_dependencies"
fi

# sort dependencies
sort -fo "$path_current_skaffold_dependencies"{,}
sort -fo "$path_actual_dependencies"{,}

case "$operation" in
  check)
    echo -n ">> Checking defined dependencies in Skaffold config '$skaffold_config_name' for '$binary_name' in '$skaffold_file'..."
    if ! diff="$(diff "$path_current_skaffold_dependencies" "$path_actual_dependencies")"; then
      echo
      echo ">>> The following actual dependencies are missing (need to be added):"
      echo "$diff" | grep '>' | awk '{print $2}'
      echo
      echo ">>> The following dependencies are not needed actually (need to be removed):"
      echo "$diff" | grep '<' | awk '{print $2}'
      echo
      echo ">>> Run './hack/update-skaffold-deps.sh' to fix."

      exit 1
    else
      echo " success."
    fi
    ;;
  update)
    echo -n ">> Updating dependencies in Skaffold config '$skaffold_config_name' for '$binary_name' in '$skaffold_file'..."

    yq eval -i "select(.metadata.name == \"$skaffold_config_name\") |= .build.artifacts[] |= select(.ko.main == \"./cmd/$binary_name\") |= .ko.dependencies.paths |= [$(cat "$path_actual_dependencies" | sed -e 's/^/"/' -e 's/$/"/' | tr '\n' ',' | sed 's/,$//')]" "$skaffold_file"

    if ! diff="$(diff "$path_current_skaffold_dependencies" "$path_actual_dependencies")"; then
      echo
      echo ">>> Added the following dependencies:"
      echo "$diff" | grep '>' | awk '{print $2}'
      echo
      echo ">>> Removed the following dependencies:"
      echo "$diff" | grep '<' | awk '{print $2}'
      echo

      exit 1
    else
      echo " already up to date."
    fi
    ;;
  *)
    echo "Unknown operation: $operation"
    exit 1
    ;;
esac
