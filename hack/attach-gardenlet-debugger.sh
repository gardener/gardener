#!/usr/bin/env bash
#
# SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

set -e

PORT=56268
while getopts "p:" opt; do
  case $opt in
    p) PORT="$OPTARG" ;;
  esac
done

POD=$(kubectl get pod -n garden -l role=gardenlet -o jsonpath='{.items[0].metadata.name}')
echo "Attaching dlv to pod $POD"

kubectl exec -n garden "$POD" -- /duct-tape/go/bin/dlv attach 1 --headless --listen=":$PORT" --continue --accept-multiclient --api-version=2 &
DLV_PID=$!

sleep 2
echo "Forwarding port $PORT"
kubectl port-forward -n garden "$POD" "$PORT:$PORT"

wait $DLV_PID
