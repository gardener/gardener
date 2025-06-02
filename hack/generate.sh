#!/usr/bin/env bash
#
# SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

set -e

WHAT="protobuf codegen manifests logcheck"
CODEGEN_GROUPS=""
MANIFESTS_DIRS=""
MODE=""
DEFAULT_MANIFESTS_DIRS=(
  "charts"
  "cmd"
  "example"
  "extensions"
  "imagevector"
  "pkg"
  "plugin"
  "test"
  "third_party"
)

parse_flags() {
  while test $# -gt 0; do
    case "$1" in
      --what)
        shift
        WHAT="${1:-$WHAT}"
        ;;
      --mode)
        shift
        if [[ -n "$1" ]]; then
        MODE="$1"
        fi
        ;;
      --codegen-groups)
        shift
        CODEGEN_GROUPS="${1:-$CODEGEN_GROUPS}"
        ;;
      --manifests-dirs)
        shift
        MANIFESTS_DIRS="${1:-$MANIFESTS_DIRS}"
        ;;
      *)
        echo "Unknown argument: $1"
        exit 1
        ;;
    esac
    shift
  done
}

overwrite_paths() {
  local updated_paths=()

  for option in "${@}"; do
    updated_paths+=("./$option/...")
  done

  echo "${updated_paths[@]}"
}

run_target() {
  local target=$1
  case "$target" in
    protobuf)
      $REPO_ROOT/hack/update-protobuf.sh
      ;;
    codegen)
      local mode="${MODE:-sequential}"
      $REPO_ROOT/hack/update-codegen.sh --groups "$CODEGEN_GROUPS" --mode "$mode"
      ;;
    manifests)
      local which=()
      local mode="${MODE:-parallel}"

      if [[ -z "$MANIFESTS_DIRS" ]]; then
        which=("${DEFAULT_MANIFESTS_DIRS[@]}")
      else
        IFS=' ' read -ra which <<< "$MANIFESTS_DIRS"
      fi

      printf "\n> Generating manifests for folders: %s\n" "${which[*]}"
      if [[ "$mode" == "sequential" ]]; then
        # In sequential mode, paths need to be converted to go package notation (e.g., ./charts/...)
        $REPO_ROOT/hack/generate-sequential.sh $(overwrite_paths "${which[@]}")
      elif [[ "$mode" == "parallel" ]]; then
        $REPO_ROOT/hack/generate-parallel.sh "${which[@]}"
      else
        printf "ERROR: Invalid mode ('%s'). Specify either 'parallel' or 'sequential'\n\n" "$mode"
        exit 1
      fi
      ;;
    logcheck)
      cd "$REPO_ROOT/$LOGCHECK_DIR" && go generate ./...
      ;;
    *)
      printf "ERROR: Unknown target: $target. Available targets are 'protobuf', 'codegen', 'manifests', 'logcheck'.\n\n"
      ;;
  esac
}

parse_flags "$@"

IFS=' ' read -ra TARGETS <<< "$WHAT"
for target in "${TARGETS[@]}"; do
  run_target "$target"
done
