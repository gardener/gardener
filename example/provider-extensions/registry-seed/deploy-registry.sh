#!/usr/bin/env bash
#
# SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

set -e

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"

usage() {
  echo "Usage:"
  echo "> deploy-registry.sh [ -h | <kubeconfig> <registry> ]"
  echo
  echo ">> For example: deploy-registry.sh ~/.kube/kubeconfig.yaml registry.gardener.cloud"

  exit 0
}

if [ "$1" == "-h" ] || [ "$#" -ne 2 ]; then
  usage
fi

if ! [ -x "$(command -v "htpasswd")" ]; then
  echo "ERROR: htpasswd is not present. Exiting..."
  exit 1
fi

kubeconfig=$1
registry=$2

if kubectl --kubeconfig "$kubeconfig" get secrets -n registry registry-password; then
  echo "Container registry password found in seed cluster"
  password=$(kubectl --kubeconfig "$kubeconfig" get secrets -n registry registry-password -o yaml | yq -e .data.password | base64 -d)
else
  echo "Generating new password for container registry $registry"
  password=$(openssl rand -base64 20)
fi

mkdir -p "$SCRIPT_DIR"/htpasswd
htpasswd -Bbn gardener "$password" > "$SCRIPT_DIR"/htpasswd/auth

echo "Creating basic auth secret for registry"
kubectl --kubeconfig "$kubeconfig" --server-side=true apply -f "$SCRIPT_DIR"/load-balancer/base/namespace.yaml
kubectl create secret generic -n registry registry-htpasswd --from-file="$SCRIPT_DIR"/htpasswd/auth --dry-run=client -o yaml | \
  kubectl --kubeconfig "$kubeconfig" --server-side=true apply  -f -
kubectl rollout restart statefulsets -n registry -l app=registry --kubeconfig "$kubeconfig"
kubectl --kubeconfig "$kubeconfig" apply -f - << EOF
apiVersion: v1
kind: Secret
metadata:
  name: registry-cache-config
  namespace: registry
type: Opaque
stringData:
  registry-host: $registry
  config.yml: |
    version: 0.1
    log:
      fields:
        service: registry
    storage:
      delete:
        enabled: true
      cache:
        blobdescriptor: inmemory
      filesystem:
        rootdirectory: /var/lib/registry
    http:
      addr: 127.0.0.1:5000
      headers:
        X-Content-Type-Options: [nosniff]
    health:
      storagedriver:
        enabled: true
        interval: 10s
        threshold: 3
    proxy:
      remoteurl: https://$registry
      username: gardener
      password: '$password'
  hosts.toml: |
    server = "https://$registry"

    [host."http://127.0.0.1:5000"]
      capabilities = ["pull", "resolve"]
  start-seed-registry-cache.conf: |
    [Service]\n
    ExecStartPre=bash /var/opt/docker/start-seed-registry-cache.sh\n
  stop-seed-registry-cache.conf: |
    [Service]\n
    ExecStopPost=bash /var/opt/docker/stop-seed-registry-cache.sh\n
  start-seed-registry-cache.sh: |
    #!/usr/bin/env bash
    if [[ "\$(ctr task ls | grep seed-registry-cache | awk '{print \$3}')" == "RUNNING" ]]; then
      echo "seed-registry-cache is already running"
      exit 0
    fi
    if [[ "\$(ctr container ls | grep seed-registry-cache | awk '{print \$1}')" == "seed-registry-cache" ]]; then
      echo "removing old seed-registry-cache container"
      ctr task kill seed-registry-cache
      ctr task rm seed-registry-cache
      ctr container rm seed-registry-cache
    fi
    if [[ "\$(ctr snapshot ls | grep seed-registry-cache | awk '{print \$1}')" == "seed-registry-cache" ]]; then
      echo "removing old seed-registry-cache snapshot"
      ctr snapshot rm seed-registry-cache
    fi
    echo "Pulling registry-cache image"
    ctr image pull europe-docker.pkg.dev/gardener-project/releases/3rd/registry:3.0.0
    echo "Starting registry-cache"
    ctr run --detach --mount type=bind,src=/var/opt/docker/seed-registry-cache-config.yml,dst=/etc/distribution/config.yml,options=rbind:ro --net-host europe-docker.pkg.dev/gardener-project/releases/3rd/registry:3.0.0 seed-registry-cache
  stop-seed-registry-cache.sh: |
    #!/usr/bin/env bash
    echo "stopping seed-registry-cache"
    ctr task kill seed-registry-cache
    ctr task rm seed-registry-cache
    ctr container rm seed-registry-cache
EOF

echo "Creating pull secret in garden namespace"
kubectl apply -f "$SCRIPT_DIR"/../../00-namespace-garden.yaml --kubeconfig "$kubeconfig" --server-side=true
kubectl create secret docker-registry -n garden gardener-images --docker-server="$registry" --docker-username=gardener --docker-password="$password" --docker-email=gardener@localhost --dry-run=client -o yaml | \
  kubectl --kubeconfig "$kubeconfig" --server-side=true apply  -f -

echo "Deploying container registry $registry"
kubectl --kubeconfig "$kubeconfig" --server-side=true apply -f "$SCRIPT_DIR"/registry/registry.yaml

echo "Waiting max 5m until registry endpoint is available"
start_time=$(date +%s)
until [ "$(curl --write-out '%{http_code}' --silent --output /dev/null https://"$registry"/v2/)" -eq "401" ]; do
  elapsed_time=$(($(date +%s) - ${start_time}))
  if [ $elapsed_time -gt 300 ]; then
    echo "Timeout"
    exit 1
  fi
  sleep 1
done

echo "Run docker login for registry $registry"
docker login "$registry" -u gardener -p "$password"

echo "Saving password in seed cluster"
kubectl create secret generic -n registry registry-password --from-literal=password="$password" --dry-run=client -o yaml | \
  kubectl --kubeconfig "$kubeconfig" --server-side=true apply  -f -
