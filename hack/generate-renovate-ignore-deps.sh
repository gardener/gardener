#!/usr/bin/env bash
#
# SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

set -euo pipefail

# Generates the ignoreDeps (or matchPackageNames, or any other) array section of a renovate.json5
# file with the dependencies that are shared between the local go.mod and the gardener/gardener go.mod.
#
# Usage:
#   GARDENER_HACK_DIR=<path>          (required) path to the gardener/gardener hack directory
#   RENOVATE_CONFIG=<path>            (optional) path to the renovate config file (default: renovate.json5)
#   ARRAY_KEY=<key>                   (optional) the renovate config key whose array is replaced
#                                     (default: "ignoreDeps")
#   NEEDLE=<comment>                  (optional) marker comment on the opening line of the target array,
#                                     used to disambiguate when the key appears multiple times
#                                     (default: "", meaning the first occurrence of ARRAY_KEY is used)
#   EXCLUDE_DEPS=<dep1,dep2,...>      (optional) comma-separated list of dependencies to exclude
#                                     from the generated list
#
# Note: dependencies starting with `github.com/gardener/gardener` are always excluded, as they
# are sub-modules of this repository and should not be pinned via renovate.

# Configurable defaults.
RENOVATE_CONFIG="${RENOVATE_CONFIG:-renovate.json5}"
ARRAY_KEY="${ARRAY_KEY:-ignoreDeps}"
NEEDLE="${NEEDLE:-}"

# Takes the content of a go.mod file and an array name to add the extracted dependencies to.
extract_dependencies() {
  local go_mod=$1
  local -n dependencies=$2  # nameref — modifies the caller's array directly

  while IFS= read -r line; do
    # Split by spaces and take the first field, discarding the version and any //indirect comment.
    local dependency
    dependency=$(echo "$line" | awk '{print $1}')
    dependencies+=("$dependency")
  done <<< "$go_mod"
}

echo "🪧 Generating section for '$RENOVATE_CONFIG' (key: '$ARRAY_KEY', needle: '${NEEDLE:-<none>}')"

# Only the dependency lines in a go.mod file are indented with a tab.
local_go_mod=$(grep -P '^\t' go.mod)
gardener_go_mod=$(grep -P '^\t' "$GARDENER_HACK_DIR/../go.mod")

local_dependencies=()
gardener_dependencies=()

extract_dependencies "$local_go_mod" local_dependencies
extract_dependencies "$gardener_go_mod" gardener_dependencies

echo "📜 Found ${#local_dependencies[@]} local dependencies."
echo "🚜 Found ${#gardener_dependencies[@]} gardener dependencies."

# Build a set of excluded dependencies for O(1) lookup.
declare -A excluded_deps=()
if [[ -n "${EXCLUDE_DEPS:-}" ]]; then
  IFS=',' read -ra exclude_list <<< "$EXCLUDE_DEPS"
  for dep in "${exclude_list[@]}"; do
    excluded_deps["$dep"]=1
  done
  echo "🚫 Excluding ${#excluded_deps[@]} dependencies: ${!excluded_deps[*]}"
fi

# Extract the intersection of local and gardener dependencies.
common_dependencies=()

for local_dep in "${local_dependencies[@]}"; do
  [[ -n "${excluded_deps[$local_dep]:-}" ]] && continue
  [[ "$local_dep" == github.com/gardener/gardener* ]] && continue
  for gardener_dep in "${gardener_dependencies[@]}"; do
    if [[ "$local_dep" == "$gardener_dep" ]]; then
      common_dependencies+=("$local_dep")
      break
    fi
  done
done

echo "☯️  Found ${#common_dependencies[@]} common dependencies."

# Build a JSON array string from the common dependencies.
ignore_deps=$(printf ',"%s"' "${common_dependencies[@]}")  # prepend comma to each element
ignore_deps="[${ignore_deps:1}]"                           # remove leading comma, wrap in []

# Build the pattern that matches the opening line of the target array, e.g.:
#   ignoreDeps: [
#   matchPackageNames: [ // GENERATOR-PIN
array_open="${ARRAY_KEY}: \[${NEEDLE:+ ${NEEDLE}}"

# Detect indentation: look at the line after the array opening and use its leading spaces.
# Falls back to 8 spaces if no existing entries are found.
indent=$(grep -A1 "$array_open" "$RENOVATE_CONFIG" | tail -1 | sed 's/\(^[[:space:]]*\).*/\1/' | head -1)
if [[ -z "$indent" || "$indent" == *$'\t'* ]]; then
  indent="        "  # default: 8 spaces (fits inside a packageRules block)
fi

# Escape forward slashes in the pattern for use in sed address ranges.
array_open_escaped="${array_open//\//\\/}"

# Format each dependency on its own indented line with a trailing comma, then replace the
# contents of the array delimited by the array opening line in the renovate config.
echo "$ignore_deps" | yq -o json '.[]' \
  | sed "s/^/${indent}/; s/$/,/" \
  | sed -i \
      -e "/  ${array_open_escaped}/,  /\]/{//!d;}" \
      -e "/  ${array_open_escaped}/r /dev/stdin" \
      "$RENOVATE_CONFIG"
