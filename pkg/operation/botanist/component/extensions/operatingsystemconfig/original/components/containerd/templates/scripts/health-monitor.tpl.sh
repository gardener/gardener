#!/bin/bash
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
