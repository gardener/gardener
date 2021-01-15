#!/bin/bash
set -o nounset
set -o pipefail

function containerd_monitoring {
  echo "ContainerD monitor has started !"
  while [ 1 ]; do
    if ! timeout 60 ctr c list > /dev/null; then
      echo "ContainerD daemon failed!"
      pkill containerd
      sleep 30
    else
      sleep $SLEEP_SECONDS
    fi
  done
}

SLEEP_SECONDS=10
echo "Start health monitoring for containerd"
containerd_monitoring
