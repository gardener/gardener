#!/usr/bin/env bash
# Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.


set -o errexit
set -o nounset
set -o pipefail

CLUSTER_NAME=""
PATH_KUBECONFIG=""
KEEP_BACKUPBUCKETS_DIRECTORY=false

parse_flags() {
  while test $# -gt 0; do
    case "$1" in
    --cluster-name)
      shift; CLUSTER_NAME="$1"
      ;;
    --path-kubeconfig)
      shift; PATH_KUBECONFIG="$1"
      ;;
    --keep-backupbuckets-dir)
      KEEP_BACKUPBUCKETS_DIRECTORY=false
      ;;
    esac

    shift
  done
}

parse_flags "$@"

kind delete cluster \
  --name "$CLUSTER_NAME"

rm -f  "$PATH_KUBECONFIG"

if [[ "$KEEP_BACKUPBUCKETS_DIRECTORY" == "false" ]]; then
  rm -rf "$(dirname "$0")/../dev/local-backupbuckets"
fi
