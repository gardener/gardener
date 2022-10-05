#!/bin/bash

kubectl get -A serviceaccounts    | awk -F '[ ]' '{$NF=""; print}'      | sed -E 's/ +$//' > 0-serviceaccounts.list
kubectl get -A deployments        | awk -F '[ ]' '{$NF=""; print}'      | sed -E 's/ +$//' > 0-deployments.list
kubectl get -A service            | awk -F '[ ]' '{$NF=""; print}'      | sed -E 's/ +$//' > 0-service.list
kubectl get -A clusterrolebinding | awk -F '[ ]' '{$NF=""; print}'      | sed -E 's/ +$//' > 0-clusterrolebinding.list
kubectl get -A clusterrole        | awk '{print $1}'                    | sed -E 's/ +$//' > 0-clusterrole.list
kubectl get -A vpa                | awk '{printf "%-40s %s\n", $1, $2}' | sed -E 's/ +$//' > 0-vpa.list

kubectl get deployments -n garden              -l component=kube-state-metrics -o yaml > 1-kube-state-metrics.garden.yaml
kubectl get deployments -n shoot--local--local -l component=kube-state-metrics -o yaml > 1-kube-state-metrics.shoot--local--local.yaml

kubectl get clusterrole -l component=kube-state-metrics -o yaml > 1-kube-state-metrics.clusterrole.yaml

kubectl port-forward -n shoot--local--local prometheus-0 9090 &
until curl -s localhost:9090 >/dev/null; do sleep 1; done
curl localhost:9090/api/v1/status/config | jq .data.yaml -r                             > 2-prometheus.controlplane.config.yaml
curl localhost:9090/api/v1/rules | yq -P | grep -v -E 'evaluationTime:|lastEvaluation:' > 2-prometheus.controlplane.rules.yaml
kill %1

kubectl port-forward -n garden prometheus-0 9090 &
until curl -s localhost:9090 >/dev/null; do sleep 1; done
curl localhost:9090/api/v1/status/config | jq .data.yaml -r                             > 3-prometheus.cache.config.yaml
curl localhost:9090/api/v1/rules | yq -P | grep -v -E 'evaluationTime:|lastEvaluation:' > 3-prometheus.cache.rules.yaml
kill %2

kubectl port-forward -n garden aggregate-prometheus-0 9090 &
until curl -s localhost:9090 >/dev/null; do sleep 1; done
curl localhost:9090/api/v1/status/config | jq .data.yaml -r                             > 4-prometheus.aggregate.config.yaml
curl localhost:9090/api/v1/rules | yq -P | grep -v -E 'evaluationTime:|lastEvaluation:' > 4-prometheus.aggregate.rules.yaml
kill %3
