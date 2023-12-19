#!/usr/bin/env bash
# SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors
# SPDX-License-Identifier: Apache-2.0

# This script waits until all conditions are passed for a given resource in a kubernetes cluster.
# It takes the resource type, object name, and a list of conditions as arguments
set -euo pipefail

if [ "$#" -lt 3 ]; then
  echo "Usage: $0 <resource_type> <object_name> <condition_1> <condition_2> ... <condition_n>
Note: Namespace/Timeout will be used from the 'NAMESPACE'/'TIMEOUT' environment variable if set, otherwise it is optional.
      TIMEOUT: The operation will be retried until the timeout[default 600 seconds] is reached, with a 5 second sleep interval between each retry.
"
  exit 1
fi

RESOURCE_TYPE=$1
OBJECT_NAME=$2
shift 2
CONDITIONS=("$@")
NAMESPACE=${NAMESPACE:-}

# The number of retries before failing
TIMEOUT=${TIMEOUT:-600}

# The interval between each retry in seconds
SLEEP_INTERVAL=${SLEEP_INTERVAL:-5}

RED='\033[0;31m'
GREEN='\033[0;32m'
NO_COLOR='\033[0m'

echo "⏳ Checking conditions for ${RESOURCE_TYPE}/${OBJECT_NAME}..."
retries=0
while [ "${retries}" -lt "${TIMEOUT}" ]; do
  if [ -z "$NAMESPACE" ]; then
    # Get the condition types in jsonpath format and pipe to yq to extract the value of conditions
    CONDITION_STATES=$(kubectl get "${RESOURCE_TYPE}" "${OBJECT_NAME}" -o json | yq '.status.conditions') || true
  else
    # Get the condition types in jsonpath format and pipe to yq to extract the value of conditions
    CONDITION_STATES=$(kubectl get "${RESOURCE_TYPE}" "${OBJECT_NAME}" -n "$NAMESPACE" -o json | yq '.status.conditions') || true
  fi

  # A flag to indicate if all conditions have passed
  ALL_PASSED=true
  # Iterate through each condition
  for condition in "${CONDITIONS[@]}"; do
    if ! echo "${CONDITION_STATES}" | yq -e '.[] | select(.type == "'"${condition}"'").status == "True"' &>/dev/null; then
      ALL_PASSED=false
      break
    fi
  done

  # If all conditions have passed, break the loop
  if [ "${ALL_PASSED}" = true ]; then
    echo -e "${GREEN}✅ All conditions passed for ${RESOURCE_TYPE}/${OBJECT_NAME}.${NO_COLOR}"
    break
  fi

  retries=$((retries + SLEEP_INTERVAL))
  sleep "${SLEEP_INTERVAL}"
done

if [ "${retries}" -ge "${TIMEOUT}" ]; then
  echo -e "${RED}❌ ERROR: ${condition} not met for ${RESOURCE_TYPE}/${OBJECT_NAME} after ${TIMEOUT} seconds.${NO_COLOR}"
  exit 1
fi
