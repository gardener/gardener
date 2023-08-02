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
WHICH="charts cmd example extensions pkg plugin test"
MODE="paralell"

parse_flags() {
  while test $# -gt 0; do
    case "$1" in
      --what)
        shift
        if [[ -n "$1" ]]; then
          WHAT="$1"
        fi
        ;;
      --mode)
        shift
        if [[ -n "$1" ]]; then
        MODE="$1"
        fi
        ;;
      --which)
        shift
        if [[ -n "$1" ]]; then
          WHICH="$1"
        fi
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
  IFS=' ' read -ra entries <<< "$WHICH"
  for entry in "${entries[@]}"; do
    WHICH=${WHICH//$entry/./$entry/...}
  done
}

run_target() {
  local target=$1
  case "$target" in
    protobuf)
      $REPO_ROOT/hack/update-protobuf.sh
      ;;
    codegen)
      $REPO_ROOT/hack/update-codegen.sh
      ;;
    manifests)
      if [[ "$MODE" == "sequential" ]]; then
        # For sequential mode, we need paths to be of the form ./charts/.., ./extensions/.. etc.
        overwrite_paths
        $REPO_ROOT/hack/generate-sequential.sh $WHICH
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
      echo "Unknown target: $target"
      ;;
  esac
}

parse_flags "$@"

for target in $WHAT; do
  run_target "$target"
done
