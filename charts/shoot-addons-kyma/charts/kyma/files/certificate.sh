#!/bin/bash
set -e

echo "---> Get Shoot Domain"
SHOOT_DOMAIN="$(kubectl -n kube-system get configmap shoot-info -o jsonpath='{.data.domain}')"
DOMAIN="$SUBDOMAIN.$SHOOT_DOMAIN"

echo "---> Requesting certificate for domain ${DOMAIN}"
cat <<EOF | kubectl apply -f -
---
apiVersion: cert.gardener.cloud/v1alpha1
kind: Certificate
metadata:
  name: kyma-cert
  namespace: kyma-installer
spec:
  commonName: "*.$DOMAIN"
EOF

while :
do
STATUS="$(kubectl get -n kyma-installer certificate.cert.gardener.cloud kyma-cert -o jsonpath='{.status.state}')"
if [ "$STATUS" = "Ready" ]; then
    break
else
    echo "Waiting for Certicate generation, status is ${STATUS}"
    sleep $DELAY
fi
done

CERT_SECRET_NAME=$(kubectl get -n kyma-installer certificate kyma-cert -o jsonpath="{.spec.secretRef.name}")
echo "---> Getting certificate from secret"
TLS_CERT=$(kubectl get -n kyma-installer secret  $CERT_SECRET_NAME -o jsonpath="{.data['tls\.crt']}" | sed 's/ /\\ /g' | tr -d '\n')
TLS_KEY=$(kubectl get -n kyma-installer secret  $CERT_SECRET_NAME -o jsonpath="{.data['tls\.key']}" | sed 's/ /\\ /g' | tr -d '\n')

echo "---> Configuring Kyma Installer"
cat <<EOF | kubectl apply -f -
---
apiVersion: v1
data:
  global.domainName: "${DOMAIN}"
  global.tlsCrt: "${TLS_CERT}"
  global.tlsKey: "${TLS_KEY}"
kind: ConfigMap
metadata:
  labels:
    installer: overrides
  name: owndomain-overrides
  namespace: kyma-installer
EOF
