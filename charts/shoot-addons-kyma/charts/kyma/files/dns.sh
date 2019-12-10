#!/bin/bash
set -e

echo "---> Configuring DNS entry"
SHOOT_DOMAIN="$(kubectl -n kube-system get configmap shoot-info -o jsonpath='{.data.domain}')"
DOMAIN="$SUBDOMAIN.$SHOOT_DOMAIN"

kubectl -n istio-system annotate service istio-ingressgateway dns.gardener.cloud/class='garden' dns.gardener.cloud/dnsnames='*.'$DOMAIN'' --overwrite
echo "---> Installation finished, browse to https://console.${DOMAIN}"
echo "---> The user name is 'admin@kyma.cx', get the password with: kubectl get secret admin-user -n kyma-system -o jsonpath='{.data.password}' | base64 -D"
