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
WHICH=""
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
      --which)
        shift
        WHICH="${1:-$WHICH}"
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
  local which=$WHICH
  IFS=' ' read -ra entries <<< "$which"
  for entry in "${entries[@]}"; do
    which=${which//$entry/./$entry/...}
  done
  echo "$which"
}

run_target() {
  local target=$1
  case "$target" in
    protobuf)
      $REPO_ROOT/hack/update-protobuf.sh
      ;;
    codegen)
      IFS=' ' read -ra available_options <<< "${AVAILABLE_CODEGEN_OPTIONS[@]}"
      local which=$WHICH
      if [[ -z "$which" ]]; then
        which=("${available_options[@]}")
        valid_options=("${available_options[@]}")
      else
        valid_options=()
        invalid_options=()
        
        IFS=' ' read -ra WHICH_ARRAY <<< "$which"
        for option in "${WHICH_ARRAY[@]}"; do
            valid=false
        
            for valid_option in "${available_options[@]}"; do
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
            printf "Skipping invalid options: %s, Available options are: %s\n\n" "${invalid_options[*]}" "${available_options[*]}"
        fi
      fi

      if [[ ${#valid_options[@]} -gt 0 ]]; then
        printf "\n> Generating codegen for groups: ${valid_options[*]}\n\n"
        $REPO_ROOT/hack/update-codegen.sh --which "${valid_options[*]}" --mode "$MODE"
      fi
      ;;
    manifests)
      if [[ "$MODE" == "sequential" ]]; then
        # In sequential mode, paths need to be converted to go package notation (e.g., ./charts/...)
        which=$(overwrite_paths)
        $REPO_ROOT/hack/generate-sequential.sh $which
      else
        $REPO_ROOT/hack/generate-parallel.sh $WHICH
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
