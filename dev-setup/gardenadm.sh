#!/usr/bin/env bash
# SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

set -o errexit
set -o pipefail

COMMAND="${1:-up}"
VALID_COMMANDS=("up" "down")

SCENARIO="${SCENARIO:-high-touch}"
VALID_SCENARIOS=("high-touch" "medium-touch")

valid_scenario=false
for scenario in "${VALID_SCENARIOS[@]}"; do
  if [[ "$SCENARIO" == "$scenario" ]]; then
    valid_scenario=true
    break
  fi
done
if ! $valid_scenario; then
  echo "Error: Invalid scenario '${SCENARIO}'. Valid options are: ${VALID_SCENARIOS[*]}." >&2
  exit 1
fi

case "$COMMAND" in
  up)
    # Prepare resources and generate manifests.
    # The manifests are copied to the high-touch machine pods or can be passed to the `--config-dir` flag of `gardenadm bootstrap`.
    skaffold build \
      -p "$SCENARIO" \
      -m gardenadm,provider-local-node,provider-local \
      -q \
      --cache-artifacts="$($(dirname "$0")/get-skaffold-cache-artifacts.sh gardenadm)" \
      |\
    skaffold render \
      -p "$SCENARIO" \
      -m provider-local-node,provider-local \
      -o "$(dirname "$0")/gardenadm/resources/generated/$SCENARIO/manifests.yaml" \
      --build-artifacts \
      -

    if [[ "$SCENARIO" == "high-touch" ]]; then
      skaffold run \
        -n gardenadm-high-touch \
        -m provider-local-node,machine
    fi
    ;;

  down)
    skaffold delete \
      -n "gardenadm-$SCENARIO"
   ;;

  *)
    echo "Error: Invalid command '${COMMAND}'. Valid options are: ${VALID_COMMANDS[*]}." >&2
    exit 1
   ;;
esac
