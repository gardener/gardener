#!/usr/bin/env bash
#
# SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

set -e

PORT=2345
while getopts "hp:" opt; do
  case $opt in
    h)
      echo "Usage: $(basename "$0") [-p PORT]"
      echo ""
      echo "Attaches a dlv debugger to the gardenlet pod in the local dev setup."
      echo ""
      echo "Options:"
      echo "  -p PORT   Local port to use for the dlv debug server and kubectl port-forward (default: 2345)"
      echo "  -h        Show this help message"
      exit 0
      ;;
    p) PORT="$OPTARG" ;;
  esac
done

POD=$(kubectl get pod -n garden -l role=gardenlet -o jsonpath='{.items[0].metadata.name}')

POD_COUNT=$(kubectl get pod -n garden -l role=gardenlet -o jsonpath='{.items[*].metadata.name}' | wc -w)
if [ "$POD_COUNT" -gt 1 ]; then
    msg="Warning: more than one gardenlet pod found."
    msg="$msg This usually should not happen in the local debugging setup, except during rollouts or if you manually scale up the replicas."
    msg="$msg Expect breakpoints to not trigger properly."

    printf '%s\n' "$msg"
fi

echo "Attaching dlv to pod $POD"

kubectl exec -n garden "$POD" -- /duct-tape/go/bin/dlv attach 1 --headless --listen=":$PORT" --continue --accept-multiclient --api-version=2 &
DLV_PID=$!

sleep 2
echo "Forwarding port $PORT"
kubectl port-forward -n garden "$POD" "$PORT:$PORT"

wait $DLV_PID
