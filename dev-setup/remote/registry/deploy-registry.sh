#!/usr/bin/env bash
#
# SPDX-FileCopyrightText: Contributors to the Gardener project
#
# SPDX-License-Identifier: Apache-2.0

set -e

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"

usage() {
  echo "Usage:"
  echo "> deploy-registry.sh [ -h | <kubeconfig> <registry> <virtual-garden-kubeconfig (optional)> ]"
  echo
  echo ">> For example: deploy-registry.sh ~/.kube/kubeconfig.yaml registry.gardener.cloud"

  exit 1
}

if [ "$1" == "-h" ] || [ "$#" -ne 2 ] && [ "$#" -ne 3 ]; then
  usage
fi

if ! [ -x "$(command -v "htpasswd")" ]; then
  echo "ERROR: htpasswd is not present. Exiting..."
  exit 1
fi

kubeconfig=$1
registry=$2
virtual_garden_kubeconfig=${3:-}

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
    ctr image pull europe-docker.pkg.dev/gardener-project/releases/3rd/registry:3.1.1
    echo "Starting registry-cache"
    ctr run --detach --mount type=bind,src=/var/opt/docker/seed-registry-cache-config.yml,dst=/etc/distribution/config.yml,options=rbind:ro --net-host europe-docker.pkg.dev/gardener-project/releases/3rd/registry:3.1.1 seed-registry-cache
  stop-seed-registry-cache.sh: |
    #!/usr/bin/env bash
    echo "stopping seed-registry-cache"
    ctr task kill seed-registry-cache
    ctr task rm seed-registry-cache
    ctr container rm seed-registry-cache
  etc-setup-hook.sh: |
    #!/usr/bin/env bash
    # This etc-setup hook restores the dev-setup seed-registry-cache wiring after GardenLinux wipes the
    # /etc overlay during an in-place OS version upgrade. It is installed at
    # /var/lib/gardenlinux/etc-setup-hooks/0-registry so it runs BEFORE the extension's 00-gardener hook.
    #
    # The seed registry is password protected and lives behind a local mirror at 127.0.0.1:5000. Both
    # pieces of /etc wiring that route to and start that mirror are destroyed by the wipe:
    #   * /etc/containerd/certs.d/<host>/hosts.toml - tells containerd to pull via 127.0.0.1:5000;
    #   * the start-seed-registry-cache.conf ExecStartPre drop-in on gardener-node-agent.service - it
    #     runs start-seed-registry-cache.sh (which survives under /var/opt) to (re)start the mirror
    #     container BEFORE the gardener-node-agent process launches.
    # Both survive under /var only as inline OSC content; the durable copies live under /var/opt, but the
    # /etc-side files are gone. We restore them here so that, by the time the 0-gardener hook restarts
    # gardener-node-agent, systemd's ExecStartPre starts the mirror and containerd routes to it - letting
    # gardener-node-agent pull its own (ko-built) image from the seed registry cache during its reconcile.
    #
    set -o errexit
    set -o nounset
    set -o pipefail

    echo "> Restoring seed-registry-cache /etc wiring after overlay wipe"

    mkdir -p "/etc/containerd/certs.d/$registry"
    printf 'server = "https://$registry"\n\n[host."http://127.0.0.1:5000"]\n  capabilities = ["pull", "resolve"]\n' \
      > "/etc/containerd/certs.d/$registry/hosts.toml"
    chmod 0640 "/etc/containerd/certs.d/$registry/hosts.toml"

    mkdir -p /etc/systemd/system/gardener-node-agent.service.d
    printf '[Service]\nExecStartPre=bash /var/opt/docker/start-seed-registry-cache.sh\n' \
      > /etc/systemd/system/gardener-node-agent.service.d/start-seed-registry-cache.conf

    echo "> Done restoring seed-registry-cache /etc wiring"
EOF

echo "Creating pull secret in garden namespace"
kubectl --kubeconfig "$kubeconfig" create namespace garden --dry-run=client -o yaml |
  kubectl --kubeconfig "$kubeconfig" --server-side=true apply  -f -
kubectl --kubeconfig "$kubeconfig" create secret docker-registry -n garden gardener-images --docker-server="$registry" --docker-username=gardener --docker-password="$password" --docker-email=gardener@localhost --dry-run=client -o yaml | \
  yq '.metadata.labels["gardener.cloud/role"] = "helm-pull-secret"' | \
  kubectl --kubeconfig "$kubeconfig" --server-side=true apply  -f -

echo "Creating registry domain ConfigMap"
kubectl --kubeconfig "$kubeconfig" create configmap -n registry registry-domain --from-literal=domain="$registry" --dry-run=client -o yaml | \
  kubectl --kubeconfig "$kubeconfig" --server-side=true apply  -f -

if [[ -n "$virtual_garden_kubeconfig" ]]; then
  echo "Creating pull secret in garden namespace of virtual garden"
  kubectl --kubeconfig "$virtual_garden_kubeconfig" create secret docker-registry -n garden gardener-images --docker-server="$registry" --docker-username=gardener --docker-password="$password" --docker-email=gardener@localhost --dry-run=client -o yaml | \
    kubectl --kubeconfig "$virtual_garden_kubeconfig" --server-side=true apply  -f -
fi

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

echo "Saving password in runtime cluster"
kubectl create secret generic -n registry registry-password --from-literal=password="$password" --dry-run=client -o yaml | \
  kubectl --kubeconfig "$kubeconfig" --server-side=true apply  -f -
