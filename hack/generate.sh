#!/usr/bin/env bash
#
# Copyright 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

WHAT="protobuf codegen manifests logcheck gomegacheck monitoring-docs"
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
    gomegacheck)
      cd "$REPO_ROOT/$GOMEGACHECK_DIR" && go generate ./...
      ;;
    monitoring-docs)
      $REPO_ROOT/hack/generate-monitoring-docs.sh
      ;;
    *)
      printf "ERROR: Unknown target: $target. Available targets are 'protobuf', 'codegen', 'manifests', 'logcheck', 'gomegacheck', 'monitoring-docs'.\n\n"
      ;;
  esac
}

parse_flags "$@"

IFS=' ' read -ra TARGETS <<< "$WHAT"
for target in "${TARGETS[@]}"; do
  run_target "$target"
done
