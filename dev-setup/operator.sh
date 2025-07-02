#!/usr/bin/env bash
# SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

set -o errexit
set -o pipefail

COMMAND="${1:-up}"
VALID_COMMANDS=("up" "dev" "debug" "down")

case "$COMMAND" in
  up)
    skaffold run \
      --cache-artifacts="$($(dirname "$0")/get-skaffold-cache-artifacts.sh)"
   ;;

  dev)
    skaffold dev
   ;;

  debug)
    skaffold debug
   ;;

  down)
    kubectl annotate garden --all confirmation.gardener.cloud/deletion=true
    kubectl delete   garden --all --ignore-not-found --wait --timeout 5m
    skaffold delete
    ;;

  *)
    echo "Error: Invalid command '${COMMAND}'. Valid options are: ${VALID_COMMANDS[*]}." >&2
    exit 1
   ;;
esac
