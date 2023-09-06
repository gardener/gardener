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
MANIFESTS_FOLDERS=""
MODE="parallel"
AVAILABLE_CODEGEN_OPTIONS=(
  "authentication"
  "core"
  "extensions"
  "resources"
  "operator"
  "seedmanagement"
  "operations"
  "settings"
  "operatorconfig"
  "controllermanager"
  "admissioncontroller"
  "scheduler"
  "gardenlet"
  "resourcemanager"
  "shoottolerationrestriction"
  "shootdnsrewriting"
  "provider_local"
  "extensions_config"
)
DEFAULT_MANIFESTS_FOLDERS=(
  "charts"
  "cmd"
  "example"
  "extensions"
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
      --codegengroups)
        shift
        CODEGEN_GROUPS="${1:-$CODEGEN_GROUPS}"
        ;;
      --manifestsfolders)
        shift
        MANIFESTS_FOLDERS="${1:-$MANIFESTS_FOLDERS}"
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
  local options=()
  IFS=' ' read -ra options <<< "$@"
  local updated_paths=()

  for option in "${options[@]}"; do
    updated_paths+=("./$option/...")
  done

  echo "${updated_paths[*]}"
}

run_target() {
  local target=$1
  case "$target" in
    protobuf)
      $REPO_ROOT/hack/update-protobuf.sh
      ;;
    codegen)
      local which=$CODEGEN_GROUPS
      local valid_options=()
      local invalid_options=()

      if [[ -z "$which" ]]; then
        which=("${AVAILABLE_CODEGEN_OPTIONS[@]}")
        valid_options=("${AVAILABLE_CODEGEN_OPTIONS[@]}")
      else
        IFS=' ' read -ra WHICH_ARRAY <<< "$which"
        for option in "${WHICH_ARRAY[@]}"; do
            valid=false
        
            for valid_option in "${AVAILABLE_CODEGEN_OPTIONS[@]}"; do
                if [[ "$option" == "$valid_option" ]]; then
                    valid=true
                    break
                fi
            done
        
            if $valid; then
                valid_options+=("$option")
            else
                invalid_options+=("$option")
            fi
        done
        
        if [[ ${#invalid_options[@]} -gt 0 ]]; then
            printf "Skipping invalid options: %s, Available options are: %s\n\n" "${invalid_options[*]}" "${AVAILABLE_CODEGEN_OPTIONS[*]}"
        fi
      fi

      if [[ ${#valid_options[@]} -gt 0 ]]; then
        printf "\n> Generating codegen for groups: %s\n" "${valid_options[*]}"
        $REPO_ROOT/hack/update-codegen.sh --which "${valid_options[*]}" --mode "$MODE"
      else
        printf "!! No valid groups provided for codegen, Available groups are: %s\n\n"  "${AVAILABLE_CODEGEN_OPTIONS[*]}"
      fi
      ;;
    manifests)
      local which=$MANIFESTS_FOLDERS
      if [[ -z "$which" ]]; then
        which=("${DEFAULT_MANIFESTS_FOLDERS[@]}")
      fi

      printf "\n> Generating manifests for folders: %s\n" "${which[*]}"
      if [[ "$MODE" == "sequential" ]]; then
        # In sequential mode, paths need to be converted to go package notation (e.g., ./charts/...)
        $REPO_ROOT/hack/generate-sequential.sh $(overwrite_paths "${which[@]}")
      elif [[ "$MODE" == "parallel" ]]; then
        $REPO_ROOT/hack/generate-parallel.sh "${which[@]}"
      else
        printf "!! Invalid mode, Specify either 'parallel' or 'sequential'\n\n"
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
      printf "Unknown target: $target. Available targets are 'protobuf', 'codegen', 'manifests', 'logcheck', 'gomegacheck', 'monitoring-docs'.\n\n"
      ;;
  esac
}

parse_flags "$@"

IFS=' ' read -ra TARGETS <<< "$WHAT"
for target in "${TARGETS[@]}"; do
  run_target "$target"
done
