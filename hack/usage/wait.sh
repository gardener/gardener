#!/usr/bin/env bash
#
# Copyright (c) 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

# This script waits until all conditions are passed for a given resource in a kubernetes cluster.
# It takes the resource type, object name, and a list of conditions as arguments
set -euo pipefail

if [ "$#" -lt 3 ]; then
  echo "Usage: $0 <resource_type> <object_name> <condition_1> <condition_2> ... <condition_n>
Note: Namespace will be used from the 'NAMESPACE' environment variable if set, otherwise it is optional.
  "
  exit 1
fi

RESOURCE_TYPE=$1
OBJECT_NAME=$2
shift 2
CONDITIONS=("$@")
NAMESPACE=${NAMESPACE:-}

# The number of retries before failing
RETRY_LIMIT=60

# The interval between each retry in seconds
SLEEP_INTERVAL=5

RED='\033[0;31m'
GREEN='\033[0;32m'
NO_COLOR='\033[0m'

echo "Checking conditions for ${RESOURCE_TYPE}/${OBJECT_NAME}..."
retries=0
while [ "${retries}" -lt "${RETRY_LIMIT}" ]; do
  if [ -z "$NAMESPACE" ]; then
    CONDITION_STATES=$(kubectl get "${RESOURCE_TYPE}" "${OBJECT_NAME}" -o jsonpath='{.status.conditions}')
  else
    CONDITION_STATES=$(kubectl get "${RESOURCE_TYPE}" "${OBJECT_NAME}" -n "$NAMESPACE" -o jsonpath='{.status.conditions}')
  fi
  ALL_PASSED=true

  for condition in "${CONDITIONS[@]}"; do
    if ! echo "${CONDITION_STATES}" | jq -e '.[] | select(.type == "'"${condition}"'").status == "True"' > /dev/null; then
      echo -e "${RED}Condition: ${condition} not met for ${RESOURCE_TYPE}/${OBJECT_NAME}${NO_COLOR}"
      ALL_PASSED=false
      break
    fi
  done

  if [ "${ALL_PASSED}" = true ]; then
    echo -e "${GREEN}✅All conditions passed for ${RESOURCE_TYPE}/${OBJECT_NAME}.${NO_COLOR}"
    break
  fi

  echo "⏳ Waiting for all conditions to pass. Retry $retries/$RETRY_LIMIT"
  ((retries++))
  sleep "${SLEEP_INTERVAL}"
done

if [ "${retries}" -eq "${RETRY_LIMIT}" ]; then
  echo -e "${RED}❌ ERROR: Not all conditions were met for ${RESOURCE_TYPE}/${OBJECT_NAME} after ${RETRY_LIMIT} retries"
  exit 1
fi
