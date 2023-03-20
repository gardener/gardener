#!/bin/bash
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

set -o nounset
set -o pipefail

function containerd_monitoring {
  echo "containerd monitor has started !"
  while [ 1 ]; do
    start_timestamp=$(date +%s)
    until ctr c list > /dev/null; do
      CONTAINERD_PID=$(systemctl show --property MainPID containerd --value)

      if [ $CONTAINERD_PID -eq 0 ]; then
          echo "Connection to containerd socket failed (process not started), retrying in $SLEEP_SECONDS seconds..."
          break
      fi

      now=$(date +%s)
      time_elapsed="$(($now-$start_timestamp))"

      if [ $time_elapsed -gt 60 ]; then
        echo "containerd daemon unreachable for more than 60s. Sending SIGTERM to PID $CONTAINERD_PID"
        kill -n 15 $CONTAINERD_PID
        sleep 20
        break 2
      fi
      echo "Connection to containerd socket failed, retrying in $SLEEP_SECONDS seconds..."
      sleep $SLEEP_SECONDS
    done
    sleep $SLEEP_SECONDS
  done
}

SLEEP_SECONDS=10
echo "Start health monitoring for containerd"
containerd_monitoring
