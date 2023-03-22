#!/bin/bash
set -o nounset
set -o pipefail

function docker_monitoring {
  echo "Docker monitor has started !"
  while [ 1 ]; do
    if ! timeout 60 docker ps > /dev/null; then
      echo "Docker daemon failed!"
      pkill docker
      sleep 30
    else
      sleep $SLEEP_SECONDS
    fi
  done
}

SLEEP_SECONDS=10
echo "Start health monitoring for docker"
docker_monitoring
