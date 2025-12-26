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
    if ! git diff --quiet HEAD -- "$pkg_path" 2>/dev/null; then
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
  
  # Check if directory has only mockgen directives
  has_only_mockgen=true
  has_mockgen=false
  
  for file in "$dir"/*.go; do
    if [ ! -f "$file" ]; then
      continue
    fi
    
    if grep -q '//go:generate' "$file"; then
      if grep -q '//go:generate.*mockgen' "$file"; then
        has_mockgen=true
        # Check if there are non-mockgen directives
        if grep '//go:generate' "$file" | grep -qv 'mockgen'; then
          has_only_mockgen=false
          break
        fi
      else
        has_only_mockgen=false
        break
      fi
    fi
  done
  
  # If directory has only mockgen directives, check if generation is needed
  if [ "$has_only_mockgen" = true ] && [ "$has_mockgen" = true ]; then
    needs_gen=false
    for file in "$dir"/*.go; do
      if [ ! -f "$file" ]; then
        continue
      fi
      if grep -q '//go:generate.*mockgen' "$file" && should_generate_mocks "$dir" "$file"; then
        needs_gen=true
        break
      fi
    done
    
    if [ "$needs_gen" = true ]; then
      echo "github.com/gardener/gardener/$dir"
    fi
  else
    # Directory has non-mockgen directives or no mockgen, always generate
    echo "github.com/gardener/gardener/$dir"
  fi
done | parallel --will-cite 'echo "Generate {}"; go generate {}'
