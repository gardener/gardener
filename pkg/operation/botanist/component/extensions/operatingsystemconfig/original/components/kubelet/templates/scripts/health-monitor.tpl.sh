#!/bin/bash
set -o nounset
set -o pipefail

function kubelet_monitoring {
  echo "Wait for 2 minutes for kubelet to be functional"
  sleep 120
  local -r max_seconds=10
  local output=""

  function kubectl {
    {{ .pathBinaries }}/kubectl --kubeconfig {{ .pathKubeletKubeconfigReal }} "$@"
  }

  function restart_kubelet {
    pkill -f "kubelet"
  }

  function patch_internal_ip {
    echo "Updating Node object $2 with InternalIP $3."
    curl \
      -XPATCH \
      -H "Content-Type: application/strategic-merge-patch+json" \
      -H "Accept: application/json" \
      "$1/api/v1/nodes/$2/status" \
      --data "{\"status\":{\"addresses\":[{\"address\": \"$3\", \"type\":\"InternalIP\"}]}}" \
      --cacert <(base64 -d <<< $(kubectl config view -o jsonpath={.clusters[0].cluster.certificate-authority-data} --raw)) \
      --key /var/lib/kubelet/pki/kubelet-client-current.pem \
      --cert /var/lib/kubelet/pki/kubelet-client-current.pem \
    > /dev/null 2>&1
  }

  timeframe=600
  toggle_threshold=5
  count_kubelet_alternating_between_ready_and_not_ready_within_timeframe=0
  time_kubelet_not_ready_first_occurrence=0
  last_kubelet_ready_state="True"

  while [ 1 ]; do
    # Check whether the kubelet's /healthz endpoint reports unhealthiness
    if ! output=$(curl -m $max_seconds -f -s -S http://127.0.0.1:10248/healthz 2>&1); then
      echo $output
      echo "Kubelet is unhealthy!"
      restart_kubelet
      sleep 60
      continue
    fi

    node_name=
    if [[ -s "{{ .pathNodeName }}" ]]; then
      node_name="$(cat "{{ .pathNodeName }}")"
    fi
    if [[ -z "$node_name" ]]; then
      echo "Node name is not known yet, waiting..."
      sleep 20
      continue
    fi

    node_object="$(kubectl get node "$node_name" -o json)"
    node_status="$(echo $node_object | jq -r '.status')"
    if [[ -z "$node_status" ]] || [[ "$node_status" == "null" ]]; then
      echo "Node object for this hostname not found in the system, waiting."
      sleep 20
      count_kubelet_alternating_between_ready_and_not_ready_within_timeframe=0
      time_kubelet_not_ready_first_occurrence=0
      continue
    fi

    # Check whether the kubelet does report an InternalIP node address
    node_ip_internal="$(echo $node_status | jq -r '.addresses[] | select(.type=="InternalIP") | .address')"
    node_ip_external="$(echo $node_status | jq -r '.addresses[] | select(.type=="ExternalIP") | .address')"
    if [[ -z "$node_ip_internal" ]] && [[ -z "$node_ip_external" ]]; then
      echo "Kubelet has not reported an InternalIP nor an ExternalIP node address yet.";
      if ! [[ -z ${K8S_NODE_IP_INTERNAL_LAST_SEEN+x} ]]; then
        echo "Check if last seen InternalIP "$K8S_NODE_IP_INTERNAL_LAST_SEEN" can be used";
        if ip address show | grep $K8S_NODE_IP_INTERNAL_LAST_SEEN > /dev/null; then
          echo "Last seen InternalIP "$K8S_NODE_IP_INTERNAL_LAST_SEEN" is still up-to-date";
          server="$(kubectl config view -o jsonpath={.clusters[0].cluster.server})"
          if patch_internal_ip $server $node_name $K8S_NODE_IP_INTERNAL_LAST_SEEN; then
            echo "Successfully updated Node object."
            continue
          else
            echo "An error occurred while updating the Node object."
          fi
        fi
      fi
      echo "Updating Node object is not possible. Restarting Kubelet.";
      restart_kubelet
      sleep 20
      continue
    elif ! [[ -z "$node_ip_internal" ]]; then
      export K8S_NODE_IP_INTERNAL_LAST_SEEN="$node_ip_internal"
    fi

    # Check whether kubelet ready status toggles between true and false and reboot VM if happened too often.
    if status="$(echo $node_status | jq -r '.conditions[] | select(.type=="Ready") | .status')"; then
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

    sleep $SLEEP_SECONDS
  done
}

SLEEP_SECONDS=10
echo "Start health monitoring for kubelet"
kubelet_monitoring
