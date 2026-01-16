#!/usr/bin/env bash
#
# SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

set -o errexit
set -o nounset
set -o pipefail

CURRENT_DIR="$(dirname $0)"
PROJECT_ROOT="${CURRENT_DIR}"/..
if [ "${PROJECT_ROOT#/}" == "${PROJECT_ROOT}" ]; then
  PROJECT_ROOT="./$PROJECT_ROOT"
fi

pushd "$PROJECT_ROOT" > /dev/null

extract_mockgen_packages() {
  local file=$1

  grep -E '//go:generate.*mockgen' "$file" | while read -r line; do
    # Strip everything before mockgen
    line="${line#*mockgen}"

    # Tokenize
    read -ra args <<< "$line"

    skip_next=false
    for arg in "${args[@]}"; do
      if $skip_next; then
        skip_next=false
        continue
      fi

      case "$arg" in
        -package|-destination|-source)
          skip_next=true
          ;;
        -package=*|-destination=*|-source=*)
          ;;
        -*)
          ;;
        *)
          # First non-flag token → source package
          echo "$arg"
          break
          ;;
      esac
    done
  done | sort -u
}

extract_api_dir_packages() {
  local file=$1

  grep -E '//go:generate.*gen-crd-api-reference-docs' "$file" | while read -r line; do
    # Extract -api-dir argument
    if [[ "$line" =~ -api-dir[[:space:]]+([^[:space:]]+) ]]; then
      echo "${BASH_REMATCH[1]}"
    elif [[ "$line" =~ -api-dir=([^[:space:]]+) ]]; then
      echo "${BASH_REMATCH[1]}"
    fi
  done | sort -u
}

should_generate_api_docs() {
  local dir=$1
  local file=$2

  local packages
  packages=$(extract_api_dir_packages "$file")

  # No detectable package → be safe
  if [ -z "$packages" ]; then
    return 0
  fi

  for pkg in $packages; do
    # Strip module prefix to get repo-relative path
    local pkg_path="${pkg#github.com/gardener/gardener/}"

    # If API directory changed → regenerate
    if ! git diff --quiet master -- "$pkg_path" 2>/dev/null; then
      return 0
    fi
  done

  # Check if output files exist
  local generated
  generated=$(grep -E '//go:generate.*gen-crd-api-reference-docs' "$file" |
    sed -n 's/.*-out-file[[:space:]]*\([^[:space:]]*\).*/\1/p')

  for gen_file in $generated; do
    # Resolve relative path from directory
    local full_path="$dir/$gen_file"
    if [ ! -f "$full_path" ]; then
      return 0
    fi
  done

  return 1
}

should_generate_crds() {
  local dir=$1
  local file=$2
  local script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

  # Source custom_packages array, get_group_package and parse_flags functions from generate-crds.sh
  source <(sed -n '/^declare -A custom_packages/p; /^get_group_package[[:space:]]*(/,/^}/p; /^parse_flags(/,/^}/p' "$script_dir/generate-crds.sh") 2>/dev/null || return 0

  # Extract groups from go:generate directives
  local groups
  groups=$(grep -E '//go:generate.*generate-crds.sh' "$file" | while read -r line; do
    # Strip everything before generate-crds.sh
    line="${line#*generate-crds.sh}"

    # Tokenize and parse using parse_flags from generate-crds.sh
    read -ra tokens <<< "$line"
    args=()
    parse_flags "${tokens[@]}"

    # Output groups (stored in args array by parse_flags)
    printf '%s\n' "${args[@]}"
  done | sort -u)

  # No detectable group → be safe
  if [ -z "$groups" ]; then
    return 0
  fi

  # Get current module path
  local current_module="$(go list -m)"

  for group in $groups; do
    local package="$(get_group_package "$group" 2>/dev/null || echo "")"

    # If we can't determine package, be safe and regenerate
    if [ -z "$package" ]; then
      return 0
    fi

    # Check if package is external (not from current module)
    if [[ "$package" != "$current_module"/* ]]; then
      # For external packages, check if go.mod changed
      if ! git diff --quiet master -- go.mod 2>/dev/null; then
        return 0
      fi
    else
      # For internal packages, check if package directory changed
      local pkg_path="${package#$current_module/}"
      if ! git diff --quiet master -- "$pkg_path" 2>/dev/null; then
        return 0
      fi
    fi
  done

  # Check if output files exist
  local output_dir="$dir"
  local prefix=""

  # Try to extract prefix from command
  if grep -q '//go:generate.*generate-crds.sh.*-p' "$file"; then
    prefix=$(grep -E '//go:generate.*generate-crds.sh' "$file" | sed -n 's/.*-p[[:space:]]*\([^[:space:]]*\).*/\1/p' | head -n1)
  fi

  for group in $groups; do
    local sanitized_group="${group%%_*}"
    local pattern="${prefix}${sanitized_group}_*.yaml"

    if ! ls "$output_dir"/$pattern &>/dev/null; then
      return 0
    fi
  done

  return 1
}

should_generate_mocks() {
  local dir=$1
  local file=$2

  local packages
  packages=$(extract_mockgen_packages "$file")

  # No detectable package → be safe
  if [ -z "$packages" ]; then
    return 0
  fi

  for pkg in $packages; do
    # If dependency is NOT a gardener package → always generate
    if [[ "$pkg" != github.com/gardener/gardener/* ]]; then
      return 0
    fi

    # Strip module prefix to get repo-relative path
    local pkg_path="${pkg#github.com/gardener/gardener/}"

    # If source package changed → regenerate
    if ! git diff --quiet master -- "$pkg_path" 2>/dev/null; then
      return 0
    fi
  done

  # Check destination files exist
  local generated
  generated=$(grep -E '//go:generate.*mockgen' "$file" |
    sed -n 's/.*-destination=\([^ ]*\).*/\1/p')

  for gen_file in $generated; do
    if [ ! -f "$dir/$gen_file" ]; then
      return 0
    fi
  done

  return 1
}

# Collect directories that need generation
ROOTS=${ROOTS:-$(
  git grep -l '//go:generate' "$@" | awk '
    {
      if (/\//) { sub(/\/[^\/]+$/, ""); } else { $0 = "."; }
      if (!seen[$0]++) {
        print $0
      }
    }
  '
)}
popd > /dev/null

# Filter mockgen-only directories
echo "$ROOTS" | while IFS= read -r dir; do
  if [ -z "$dir" ]; then
    continue
  fi
  
  # Check if directory has only mockgen or api-docs directives
  has_only_skippable=true
  has_skippable=false
  
  for file in "$dir"/*.go; do
    if [ ! -f "$file" ]; then
      continue
    fi
    
    if grep -q '//go:generate' "$file"; then
      # Check if file has mockgen, gen-crd-api-reference-docs, or generate-crds.sh
      if grep -q '//go:generate.*mockgen' "$file" || grep -q '//go:generate.*gen-crd-api-reference-docs' "$file" || grep -q '//go:generate.*generate-crds.sh' "$file"; then
        has_skippable=true
        # Check if there are other non-skippable directives
        if grep '//go:generate' "$file" | grep -v 'mockgen' | grep -v 'gen-crd-api-reference-docs' | grep -qv 'generate-crds.sh'; then
          has_only_skippable=false
          break
        fi
      else
        has_only_skippable=false
        break
      fi
    fi
  done
  
  # If directory has only skippable directives, check if generation is needed
  if [ "$has_only_skippable" = true ] && [ "$has_skippable" = true ]; then
    needs_gen=false
    for file in "$dir"/*.go; do
      if [ ! -f "$file" ]; then
        continue
      fi
      if grep -q '//go:generate.*mockgen' "$file" && should_generate_mocks "$dir" "$file"; then
        needs_gen=true
        break
      fi
      if grep -q '//go:generate.*gen-crd-api-reference-docs' "$file" && should_generate_api_docs "$dir" "$file"; then
        needs_gen=true
        break
      fi
      if grep -q '//go:generate.*generate-crds.sh' "$file" && should_generate_crds "$dir" "$file"; then
        needs_gen=true
        break
      fi
    done
    
    if [ "$needs_gen" = true ]; then
      echo "github.com/gardener/gardener/$dir"
    else
      echo "Skipping github.com/gardener/gardener/$dir (no changes detected)" >&2
    fi
  else
    # Directory has non-skippable directives, always generate
    echo "github.com/gardener/gardener/$dir"
  fi
done | parallel --will-cite 'echo "Generate {}"; go generate {}'
