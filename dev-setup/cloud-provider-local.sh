#!/usr/bin/env bash
# SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

set -o errexit
set -o pipefail

source "$(dirname "$0")/scenario.sh"

detect_kind_cluster_name

COMMAND="${1:-up}"
VALID_COMMANDS=("up" "dev" "debug" "down")

case "$COMMAND" in
  up)
    skaffold run
   ;;

  dev)
    skaffold dev
   ;;

  debug)
    skaffold debug
   ;;

  down)
    skaffold delete
    ;;

  *)
    echo "Error: Invalid command '${COMMAND}'. Valid options are: ${VALID_COMMANDS[*]}." >&2
    exit 1
   ;;
esac
