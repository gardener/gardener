{{define "health-monitor" -}}
{{/* Do not remove the indentation, this is required because this template is imported by others */ -}}
- path: /opt/bin/health-monitor
  permissions: 0755
  content: |
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
    function kubelet_monitoring {
      echo "Wait for 2 minutes for kubelet to be functional"
      sleep 120
      local -r max_seconds=10
      local output=""
      while [ 1 ]; do
        if ! output=$(curl -m $max_seconds -f -s -S http://127.0.0.1:10255/healthz 2>&1); then
          echo $output
          echo "Kubelet is unhealthy!"
          pkill kubelet
          sleep 60
        else
          sleep $SLEEP_SECONDS
        fi
      done
    }
    SLEEP_SECONDS=10
    component=$1
    echo "Start kubernetes health monitoring for $component"
    if [[ $component == "docker" ]]; then
      docker_monitoring
    elif [[ $component == "kubelet" ]]; then
      kubelet_monitoring
    else
      echo "Health monitoring for component $component is not supported!"
    fi
{{- end}}
