#!/usr/bin/env bash
# SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0


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
