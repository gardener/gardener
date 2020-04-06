#!/usr/bin/env bash
#
# Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

checkPrereqs() {
  command -v host > /dev/null || echo "please install host command for lookup"
  command -v inlets > /dev/null || echo "please install the inlets command. For mac, simply use \`brew install inlets\`, for linux \`curl -sLS https://get.inlets.dev | sudo sh\`"
}

createOrUpdateWebhookSVC(){
providerName=${1:-}
[[ -z $providerName ]] && echo "Please specify the provider name (aws,gcp,azure,..etc.)!" && exit 1

namespace=${2:-}
[[ -z $namespace ]] && echo "Please specify extension namespace!" && exit 1

tmpService=$(mktemp)
kubectl get svc gardener-extension-provider-$providerName -o yaml --export > $tmpService

    cat <<EOF | kubectl apply -f -
---
apiVersion: v1
kind: Service
metadata:
  labels:
    app: gardener-extension-provider-$providerName
    app.kubernetes.io/instance: provider-$providerName
    app.kubernetes.io/name: gardener-extension-provider-$providerName
  name: gardener-extension-provider-$providerName
  namespace: $namespace
spec:
  ports:
  - port: 443
    protocol: TCP
    targetPort: 9443
  selector:
    app: inlets-server
    app.kubernetes.io/instance: provider-$providerName
    app.kubernetes.io/name: gardener-extension-provider-$providerName
  type: ClusterIP
EOF
}


createInletsLB(){
namespace=${1:-}
[[ -z $namespace ]] && echo "Please specify extension namespace!" && exit 1

cat <<EOF | kubectl apply -f -
---
apiVersion: v1
kind: Service
metadata:
  labels:
    app: inlets-lb
  name: inlets-lb
  namespace: $namespace
spec:
  externalTrafficPolicy: Cluster
  ports:
  - name: 8000-8080
    port: 8000
    protocol: TCP
    targetPort: 8080
  selector:
    app: inlets-server
  type: LoadBalancer
EOF
}

waitForInletsLBToBeReady(){
    namespace=${1:-}
    [[ -z $namespace ]] && echo "Please specify extension namespace!" && exit 1

    providerName=${2:-}
    [[ -z $providerName ]] && echo "Please specify the provider name (aws,gcp,azure,..etc.)!" && exit 1

    case $providerName in
    aws*)
      until host $(kubectl -n $namespace get svc inlets-lb -o go-template="{{ index (index  .status.loadBalancer.ingress 0).hostname }}") 2>&1 > /dev/null
      do
        sleep 2s
      done
      echo $(kubectl -n $namespace get svc inlets-lb -o go-template="{{ index (index  .status.loadBalancer.ingress 0).hostname }}")
      ;;
    *)
      until host $(kubectl -n $namespace get svc inlets-lb -o go-template="{{ index (index  .status.loadBalancer.ingress 0).ip }}") 2>&1 > /dev/null
      do
        sleep 2s
      done
      echo $(kubectl -n $namespace get svc inlets-lb -o go-template="{{ index (index  .status.loadBalancer.ingress 0).ip }}")      ;;
    esac
}

createServerPod(){
namespace=${1:-}
[[ -z $namespace ]] && echo "Please specify extension namespace!" && exit 1

providerName=${2:-}
[[ -z $providerName ]] && echo "Please specify the provider name (aws,gcp,azure,..etc.)!" && exit 1

cat <<EOF | kubectl apply -f -
---
apiVersion: v1
kind: Pod
metadata:
  labels:
    app: inlets-server
    app.kubernetes.io/instance: provider-$providerName
    app.kubernetes.io/name: gardener-extension-provider-$providerName
    networking.gardener.cloud/to-dns: allowed
    networking.gardener.cloud/to-public-networks: allowed
  name: inlets-server
  namespace: $namespace
spec:
  containers:
  - args:
    - "server"
    - "-p"
    - "8080"
    - "-t"
    - "21d809ed61915c9177fbceeaa87e307e766be5f2"
    image: inlets/inlets:2.6.3
    imagePullPolicy: IfNotPresent
    name: inlets-server
    resources:
      limits:
        cpu: 50m
        memory: 128Mi
      requests:
        cpu: 20m
        memory: 64Mi
  - args:
    - "server"
    - "--target"
    - "127.0.0.1:8080"
    - "--listen"
    - "0.0.0.0:9443"
    - "--cacert"
    - "/etc/tls/ca.crt"
    - "--cert"
    - "/etc/tls/tls.crt"
    - "--key"
    - "/etc/tls/tls.key"
    - "--disable-authentication"
    image:  "squareup/ghostunnel:v1.5.2"
    imagePullPolicy: IfNotPresent
    name: ghost-server
    volumeMounts:
    - name: inlets-tls
      mountPath: "/etc/tls"
      readOnly: true
    resources:
      limits:
        cpu: 50m
        memory: 128Mi
      requests:
        cpu: 20m
        memory: 64Mi
  - args:
    - "sleep"
    - "8000s"
    image: busybox
    imagePullPolicy: IfNotPresent
    name: debug
    resources:
      limits:
        cpu: 50m
        memory: 128Mi
      requests:
        cpu: 20m
        memory: 64Mi
  volumes:
  - name: inlets-tls
    secret:
      secretName: gardener-extension-webhook-cert
  dnsPolicy: ClusterFirst
  enableServiceLinks: true
  restartPolicy: Always

EOF
}

waitForInletsPodToBeReady(){
    namespace=${1:-}
    [[ -z $namespace ]] && echo "Please specify extension namespace!" && exit 1

    until test "$(kubectl -n $namespace get pods inlets-server --no-headers | awk '{print $2}')" = "3/3"
    do
      sleep 2s
    done
}

cleanUP() {
   namespace=${1:-}
   [[ -z $namespace ]] && echo "Please specify the extension namespace!" && exit 1

   echo "cleaning up local-dev setup.."

   echo "Deleting inlets service..."
   kubectl -n $namespace delete  svc/inlets-lb

   echo "Deleting the inlets pod..."
   kubectl -n $namespace delete  pod/inlets-server

   echo "Re-applying old service values..."
   kubectl apply -f $tmpService

   kill -9 $(pgrep inlets) 2>/dev/null
   exit 0
}

usage(){
  echo "==================================================================DISCLAIMER============================================================================"
  echo "This scripts needs to be run against the KUBECONFIG of a seed cluster, please set your KUBECONFIG accordingly"
  echo "You also need to set the \`ignoreResources\` variable in your extension chart to \`true\`, generate and apply the corresponding controller-installation"
  echo "========================================================================================================================================================"

  echo ""

  echo "===================================PRE-REQs========================================="
  echo "\`host\` commands for DNS"
  echo "\`inlets\` command. For mac, simply use \`brew install inlets\`, for linux \`curl -sLS https://get.inlets.dev | sudo sh\`"
  echo "===================================================================================="

  echo ""

  echo "========================================================USAGE======================================================================"
  echo "> ./hack/hook-me.sh <extension namespace e.g. extension-provider-aws-fpr6w> <provider e.g., aws>  <webhookserver port e.g., 8443>"
  echo "> \`make EXTENSION_NAMESPACE=<extension namespace e.g. extension-provider-aws-fpr6w> start-provider-<provider-name e.g.,aws>-local\`"
  echo "=================================================================================================================================="

  echo ""

  echo "===================================CLEAN UP COMMANDS========================================="
  echo ">  kubectl -n $namespace delete  svc/inlets-lb"
  echo ">  kubectl -n $namespace delete  pod/inlets-server"
  echo "============================================================================================="

  exit 0
}
if [[ "${BASH_SOURCE[0]}" = "$0" ]]; then

  if [ "$1" == "-h" ] ; then
        usage
  fi

  providerName=${1:-}
  [[ -z $providerName ]] && echo "Please specify the provider name (aws,gcp,azure,..etc.)!" && exit 1

  namespace=${2:-}
  [[ -z $namespace ]] && echo "Please specify the extension namespace!" && exit 1

  webhookServerPort=${3:-}
  [[ -z $webhookServerPort ]] && echo "Please specify webhook server port" && exit 1


  trap 'cleanUP $namespace' SIGINT SIGTERM

  while true; do
    read -p "[STEP 0] Have you already set the \`ignoreResources\` chart value to \`true\` for your extension controller-registration?" yn
    case $yn in
        [Yy]* )
            echo "[STEP 1] Checking Pre-reqs!"
            checkPrereqs

            echo "[STEP 2] Creating Inlets LB Service..!"
            createInletsLB $namespace && sleep 2s

            echo "[STEP 3] Waiting for Inlets LB Service to be created..!";
            loadbalancerIPOrHostName=$(waitForInletsLBToBeReady $namespace $providerName)
            echo "[Info] LB IP is $loadbalancerIPOrHostName"

            echo "[STEP 4] Creating the server Pod for TLS Termination and Tunneling connection..!";
            createServerPod $namespace $providerName

            echo "[STEP 5] Waiting for Inlets Pod to be ready..!";
            waitForInletsPodToBeReady $namespace

            echo "[STEP 6] Creating WebhookSVC LB..!"
            createOrUpdateWebhookSVC $namespace $providerName

            echo "[STEP 7] Initializing the inlets client";
            echo "[Info] Inlets initialized, you are ready to go ahead and run \"make EXTENSION_NAMESPACE=$namespace start-provider-$providerName-local\""
            echo "[Info] It will take about 5 seconds for the connection to succeeed!"

            inlets client --remote ws://$loadbalancerIPOrHostName:8000 --upstream https://localhost:$webhookServerPort --token=21d809ed61915c9177fbceeaa87e307e766be5f2
        ;;
        [Nn]* ) echo "You need to set  \`ignoreResources\` to true and generate the controller installlation first in your extension chart before proceeding!"; exit;;
        * ) echo "Please answer yes or no.";;
    esac
done
fi
