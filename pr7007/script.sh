#!/bin/bash

set -ex

mkdir -p state

# The Grafana deployments and dashboards
kubectl -n shoot--local--local get deployments | grep grafana | awk '{print $1}' \
| while read -r name; do
    kubectl -n shoot--local--local get deployment "$name" -o yaml \
      > state/"$name"-deployment.yaml
    kubectl -n shoot--local--local exec deployments/"$name" \
      -- sh -c "ls /var/lib/grafana/dashboards" \
      > state/"$name"-dashboards.list
  done

# The Grafana ingresses
kubectl -n shoot--local--local get ingresses | grep grafana | awk '{print $1}' \
| while read -r name; do
    kubectl -n shoot--local--local get ingress "$name" -o yaml \
      > state/"$name"-ingress.yaml
  done

# The Prometheus ingress
kubectl -n shoot--local--local get ingress prometheus -o yaml \
  > state/prometheus-ingress.yaml

# The alertmanager ingress
kubectl -n shoot--local--local get ingress alertmanager -o yaml \
  > state/alertmanager-ingress.yaml

# The observability secrets in the control plane
kubectl -n shoot--local--local get secrets | grep observability | awk '{print $1}' \
| while read -r name; do
    kubectl -n shoot--local--local get secret "$name" -o yaml | sha256sum \
      > state/"$name"-secret.yaml.hash
  done

# The monitoring secret in the project namespace
kubectl -n garden-local get secret local.monitoring -o yaml | sha256sum \
  > state/local.monitoring-secret.yaml.hash

# All the secret names
kubectl get secrets -A \
  > state/all-secrets.list

# DNS names in the Grafana tls secret
openssl x509 -in <(kubectl get secret -n shoot--local--local -l name=grafana-tls -o json \
                   | jq '.items[].data."tls.crt"' -r \
                   | base64 -d) -text | grep DNS \
  > state/grafana-tls-domains.txt
