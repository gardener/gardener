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

      function kubectl {
        /opt/bin/hyperkube kubectl --kubeconfig /var/lib/kubelet/kubeconfig-real "$@"
      }

      timeframe=600
      toggle_threshold=5
      count_kubelet_alternating_between_ready_and_not_ready_within_timeframe=0
      time_kubelet_not_ready_first_occurrence=0
      last_kubelet_ready_state="True"

      while [ 1 ]; do
        if ! output=$(curl -m $max_seconds -f -s -S http://127.0.0.1:10255/healthz 2>&1); then
          echo $output
          echo "Kubelet is unhealthy!"
          pkill kubelet
          sleep 60
          continue
        fi

        # Check whether kubelet ready status toggles between true and false and reboot VM if happened too often.
        if status="$(kubectl get nodes -l kubernetes.io/hostname=$(hostname) -o json | jq -r '.items[0].status.conditions[] | select(.type=="Ready") | .status')"; then
          if [[ "$status" != "True" ]]; then
            if [[ $time_kubelet_not_ready_first_occurrence == 0 ]]; then
              time_kubelet_not_ready_first_occurrence=$(date +%s)
              echo "Start tracking kubelet ready status toggles."
            fi
          else
            if [[ $time_kubelet_not_ready_first_occurrence != 0 ]]; then
              if [[ "$last_kubelet_ready_state" != "$status" ]]; then
                count_kubelet_alternating_between_ready_and_not_ready_within_timeframe=$((count_kubelet_alternating_between_ready_and_not_ready_within_timeframe+1))
                echo "count_kubelet_alternating_between_ready_and_not_ready_within_timeframe=$count_kubelet_alternating_between_ready_and_not_ready_within_timeframe"
                if [[ $count_kubelet_alternating_between_ready_and_not_ready_within_timeframe -ge $toggle_threshold ]]; then
                  sudo reboot
                fi
              fi
            fi
          fi

          if [[ $time_kubelet_not_ready_first_occurrence != 0 && $(($(date +%s)-$time_kubelet_not_ready_first_occurrence)) -ge $timeframe ]]; then
            count_kubelet_alternating_between_ready_and_not_ready_within_timeframe=0
            time_kubelet_not_ready_first_occurrence=0
            echo "Resetting kubelet ready status toggle tracking."
          fi

          last_kubelet_ready_state="$status"
        fi

        # Check whether kubelet reports "PLEG not healthy" too often within the last 10 minutes and reboot VM if necessary.
        if count_pleg_not_healthy="$(journalctl --since="$(date --date '-10min' "+%Y-%m-%d %T")" -u kubelet | grep "PLEG is not healthy" | wc -l)"; then
          if [[ $count_pleg_not_healthy -ge 10 ]]; then
            sudo reboot
          fi
        fi

        sleep $SLEEP_SECONDS
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
