#!/usr/bin/env bash
#
# SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

set -o errexit
set -o nounset
set -o pipefail

QUIC_CLIENT_IMAGE=ghcr.io/gardener/quic-reverse-http-tunnel/quic-client-tcp:v0.1.4
QUIC_SERVER_IMAGE=ghcr.io/gardener/quic-reverse-http-tunnel/quic-server:v0.1.4

QUIC_SECRET_NAME=quic-tunnel-certs
QUIC_CLIENT_CONTAINER=gardener-quic-client

CERTS_DIR=$(pwd)/tmp/certs

checkPrereqs() {
    command -v host > /dev/null || echo "please install host command for lookup"
    command -v docker > /dev/null || echo "please install docker https://www.docker.com"
    command -v yq > /dev/null || echo "please install yq https://github.com/mikefarah/yq"
}

createOrUpdateWebhookSVC(){
    namespace=${1:-}
    [[ -z $namespace ]] && echo "Please specify extension namespace!" && exit 1

    serviceName=${2:-}
    [[ -z $serviceName ]] && echo "Please specify the service name (gardener-extension-provider-{aws,gcp,azure},..etc.)!" && exit 1

    local quicServerPort=${3:-}
    [[ -z $quicServerPort ]] && echo "Please specify the quic pod server port!" && exit 1

    tmpService=$(mktemp)
    echo ">> A backup copy of the service $namespace/$serviceName is stored in the file $tmpService"
    kubectl -n $namespace get svc $serviceName -o yaml | yq 'del(.metadata.resourceVersion)' > $tmpService

    cat <<EOF | kubectl apply -f -
---
apiVersion: v1
kind: Service
metadata:
  labels:
    app: $serviceName
    app.kubernetes.io/instance: $serviceName
    app.kubernetes.io/name: $serviceName
  annotations:
    networking.resources.gardener.cloud/from-world-to-ports: '[{"protocol":"TCP","port":${quicServerPort}}]'
  name: $serviceName
  namespace: $namespace
spec:
  ports:
  - port: 443
    protocol: TCP
    targetPort: $quicServerPort
  selector:
    app: quic-server
    app.kubernetes.io/instance: $serviceName
    app.kubernetes.io/name: $serviceName
  type: ClusterIP
EOF
}


createQuicLB(){
    namespace=${1:-}
    [[ -z $namespace ]] && echo "Please specify extension namespace!" && exit 1

    local quicTunnelPort=${2:-}
    [[ -z $quicTunnelPort ]] && echo "Please specify the quic pod tunnel port!" && exit 1

    cat <<EOF | kubectl apply -f -
---
apiVersion: v1
kind: Service
metadata:
  labels:
    app: quic-lb
  annotations:
    networking.resources.gardener.cloud/from-world-to-ports: '[{"protocol":"UDP","port":${quicTunnelPort}}]'
    service.beta.kubernetes.io/aws-load-balancer-type: "nlb"
  name: quic-lb
  namespace: $namespace
spec:
  externalTrafficPolicy: Cluster
  ports:
  - name: quic-tunnel-port
    port: $quicTunnelPort
    protocol: UDP
    targetPort: $quicTunnelPort
  selector:
    app: quic-server
  type: LoadBalancer
EOF
}

waitForQuicLBToBeReady(){
    namespace=${1:-}
    [[ -z $namespace ]] && echo "Please specify extension namespace!" && exit 1

    until host $(kubectl -n $namespace get svc quic-lb -o yaml | yq '.status.loadBalancer.ingress[0] | .hostname // .ip') 2>&1 > /dev/null
    do
        sleep 2s
    done
    echo $(kubectl -n $namespace get svc quic-lb -o yaml | yq '.status.loadBalancer.ingress[0] | .hostname // .ip')
}

createServerDeploy(){
    namespace=${1:-}
    [[ -z $namespace ]] && echo "Please specify extension namespace!" && exit 1

    serviceName=${2:-}
    [[ -z $serviceName ]] && echo "Please specify the service name (gardener-extension-provider-{aws,gcp,azure},..etc.)!" && exit 1

    local quicServerPort=${3:-}
    [[ -z $quicServerPort ]] && echo "Please specify the quic pod server port!" && exit 1

    local quicTunnelPort=${4:-}
    [[ -z $quicTunnelPort ]] && echo "Please specify the quic pod tunnel port!" && exit 1

    cat <<EOF | kubectl apply -f -
---
apiVersion: apps/v1
kind: Deployment
metadata:
  labels:
    app: quic-server
    app.kubernetes.io/instance: $serviceName
    app.kubernetes.io/name: $serviceName
    networking.gardener.cloud/to-dns: allowed
    networking.gardener.cloud/to-public-networks: allowed
  name: quic-server
  namespace: $namespace
spec:
  replicas: 1
  selector:
    matchLabels:
      app: quic-server
  template:
    metadata:
      labels:
        app: quic-server
        app.kubernetes.io/instance: $serviceName
        app.kubernetes.io/name: $serviceName
    spec:
      containers:
      - args:
        - --listen-tcp=0.0.0.0:$quicServerPort
        - --listen-quic=0.0.0.0:$quicTunnelPort
        - --cert-file=/certs/tls.crt
        - --cert-key=/certs/tls.key
        - --client-ca-file=/certs/ca.crt
        image: "${QUIC_SERVER_IMAGE}"
        imagePullPolicy: IfNotPresent
        name: quic-server
        volumeMounts:
        - name: quic-tls
          mountPath: "/certs"
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
        image: alpine
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
      - name: quic-tls
        secret:
          secretName: ${QUIC_SECRET_NAME}
      dnsPolicy: ClusterFirst
      enableServiceLinks: true
      restartPolicy: Always
EOF
}

waitForQuicDeployToBeReady(){
    namespace=${1:-}
    [[ -z $namespace ]] && echo "Please specify extension namespace!" && exit 1

    until test "$(kubectl -n $namespace get deploy quic-server --no-headers | awk '{print $2}')" = "1/1"
    do
        sleep 2s
    done
}

createCerts() {
    local dir=${1:-}
    [[ -z $dir ]] && echo "Please specify certs directory!" && exit 1

    mkdir -p $dir

    local ipOrHostname=${2:-}
    local template=""

    # This will not validate the quads but it is enough to determine if the value is an ip or a hostname
    if [[ $ipOrHostname =~ ^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    template="IP.1 = ${ipOrHostname}"
    else
    template="DNS.3 = ${ipOrHostname}"
    fi
(
    cd $dir

    cat > server.conf << EOF
[req]
req_extensions = v3_req
distinguished_name = req_distinguished_name
[req_distinguished_name]
[ v3_req ]
basicConstraints = CA:FALSE
keyUsage = nonRepudiation, digitalSignature, keyEncipherment
extendedKeyUsage = serverAuth
subjectAltName = @alt_names
[alt_names]
DNS.1 = localhost
DNS.2 = quic-tunnel-server
${template}
IP.2 = 127.0.0.1
EOF

    cat > client.conf << EOF
[req]
req_extensions = v3_req
distinguished_name = req_distinguished_name
[req_distinguished_name]
[ v3_req ]
basicConstraints = CA:FALSE
keyUsage = nonRepudiation, digitalSignature, keyEncipherment
extendedKeyUsage = clientAuth
EOF

    # Create a certificate authority
    openssl genrsa -out ca.key 3072
    openssl req -x509 -new -nodes -key ca.key -days 1 -out ca.crt -subj "/CN=quic-tunnel-ca"

    # Create a server certificate
    openssl genrsa -out tls.key 3072
    openssl req -new -key tls.key -out server.csr -subj "/CN=quic-tunnel-server" -config server.conf
    openssl x509 -req -in server.csr -CA ca.crt -CAkey ca.key -CAcreateserial -out tls.crt -days 1 -extensions v3_req -extfile server.conf

    # Create a client certificate
    openssl genrsa -out client.key 3072
    openssl req -new -key client.key -out client.csr -subj "/CN=quic-tunnel-client" -config client.conf
    openssl x509 -req -in client.csr -CA ca.crt -CAkey ca.key -CAcreateserial -out client.crt -days 1 -extensions v3_req -extfile client.conf

    # Clean up after we're done.
    rm ./*.csr
    rm ./*.srl
    rm ./*.conf
)
}

loadCerts() {
    local certsDir=${1:-}
    local namespace=${2:-}
    local secret=${3:-}
    [[ -z $certsDir ]] && echo "Please specify local certs Dir!" && exit 1
    [[ -z $namespace ]] && echo "Please specify extension namespace!" && exit 1
    [[ -z $secret ]] && echo "Please specify webhook secret name!" && exit 1

    # if it already exists, we get rid of it
    kubectl -n $namespace delete --ignore-not-found secret $secret 2>/dev/null || true

    # now create it anew
    (
    cd $certsDir
    kubectl -n $namespace create secret generic $secret --from-file=ca.crt --from-file=tls.key --from-file=tls.crt
    )
}

tunnelConsent() {
    local red='\033[0;31m'
    local no_color='\033[0m'
    echo -e "${red}> WARNING: A network tunnel from the seed cluster toward this host is about to be opened via https://github.com/gardener/quic-reverse-http-tunnel.${no_color}"

    read -p "Do you agree the tunnel to be opened? [Yes|No]: " yn
    case $yn in
        [Yy]* )
            echo "Tunnel will be opened!"
            ;;
        [Nn]* )
            echo "Tunnel will not be opened, exiting ..."
            exit
            ;;
        * )
            echo "Invalid answer, please answer with 'Yes' or 'No'."
            exit 1
            ;;
    esac
}

cleanUP() {
    namespace=${1:-}
    [[ -z $namespace ]] && echo "Please specify the extension namespace!" && exit 1

    echo "cleaning up local-dev setup.."

    echo "Deleting quic service..."
    kubectl -n $namespace delete --ignore-not-found svc/quic-lb

    echo "Deleting the quic deploy..."
    kubectl -n $namespace delete --ignore-not-found deploy/quic-server

    echo "Deleting the quic certs..."
    kubectl -n $namespace delete --ignore-not-found secret/quic-tunnel-certs

    echo "Re-applying old service values..."
    kubectl apply -f $tmpService

    if [[ "$(docker container ls -f name=$QUIC_CLIENT_CONTAINER -q)" != "" ]]; then
        docker kill $QUIC_CLIENT_CONTAINER
    fi
}

usage(){
    echo "==================================================================DISCLAIMER============================================================================"
    echo "This scripts needs to be run against the KUBECONFIG of a seed cluster, please set your KUBECONFIG accordingly"
    echo "You also need to set the \`ignoreResources\` variable in your extension chart to \`true\`, generate and apply the corresponding controller-installation"
    echo "========================================================================================================================================================"

    echo ""

    echo "===================================PRE-REQs========================================="
    echo "\`host\` commands for DNS"
    echo "\`docker\` https://www.docker.com"
    echo "\`yq\` https://github.com/mikefarah/yq"
    echo "===================================================================================="

    echo ""

    echo "========================================================USAGE======================================================================"
    echo "> ./hack/hook-me.sh <service e.g., gardener-extension-provider-aws> <extension namespace e.g. extension-provider-aws-fec6w> <webhookserver port e.g., 8443> [<quic-server port, e.g. 9443>]"
    echo "> \`make start [EXTENSION_NAMESPACE=<extension namespace e.g. extension-provider-aws-fec6w> WEBHOOK_CONFIG_MODE=service GARDEN_KUBECONFIG=<path to kubeconfig for garden cluster>]\`"
    echo "=================================================================================================================================="

    echo ""

    echo "===================================CLEAN UP COMMANDS========================================="
    echo "> kubectl -n <extension namespace e.g. extension-provider-aws-fec6w> delete --ignore-not-found svc/quic-lb"
    echo "> kubectl -n <extension namespace e.g. extension-provider-aws-fec6w> delete --ignore-not-found deploy/quic-server"
    echo "============================================================================================="

    exit 0
}

if [[ "${BASH_SOURCE[0]}" = "$0" ]]; then
    if [ "$#" -lt 2 ] || [ "$1" == "-h" ]; then
        usage
    fi

    serviceName=${1:-}
    [[ -z $serviceName ]] && echo "Please specify the service name (gardener-extension-provider-{aws,gcp,azure},..etc.)!" && exit 1

    namespace=${2:-}
    [[ -z $namespace ]] && echo "Please specify the extension namespace!" && exit 1

    webhookServerPort=${3:-}
    [[ -z $webhookServerPort ]] && echo "Please specify webhook server port" && exit 1

    quicServerPort=${4:-}
    [[ -z $quicServerPort ]] && echo "quic-server port not specified, using default port of 9443" && quicServerPort=9443

    quicTunnelPort=${5:-}
    [[ -z $quicTunnelPort ]] && echo "quic-tunnel port not specified, using default port of 9444" && quicTunnelPort=9444

    tunnelConsent
    trap 'cleanUP $namespace' EXIT

    while true; do
    read -p "[STEP 0] Have you already set the \`ignoreResources\` chart value to \`true\` for your extension controller-registration?" yn
    case $yn in
        [Yy]* )
            echo "[STEP 1] Checking Pre-reqs!"
            checkPrereqs

            echo "[STEP 2] Creating Quic LB Service..!"
            createQuicLB $namespace $quicTunnelPort && sleep 2s

            echo "[STEP 3] Waiting for Quic LB Service to be created..!";
            output=$(waitForQuicLBToBeReady $namespace $serviceName)
            loadbalancerIPOrHostName=$(echo "$output" | tail -n1)
            echo "[Info] LB IP is $loadbalancerIPOrHostName"

            echo "[STEP 4] Creating the CA, client and server keys and certs..!";
            createCerts $CERTS_DIR $loadbalancerIPOrHostName

            echo "[STEP 5] Loading quic tunnel certs into cluster..!";
            loadCerts $CERTS_DIR $namespace $QUIC_SECRET_NAME

            echo "[STEP 6] Creating the server Deploy for TLS Termination and Tunneling connection..!";
            createServerDeploy $namespace $serviceName $quicServerPort $quicTunnelPort

            echo "[STEP 7] Waiting for Quic Deploy to be ready..!";
            waitForQuicDeployToBeReady $namespace

            echo "[STEP 8] Creating WebhookSVC LB..!"
            createOrUpdateWebhookSVC $namespace $serviceName $quicServerPort

            echo "[STEP 9] Initializing the quic client";
            echo "[Info] Quic initialized, you are ready to go ahead and run \"make EXTENSION_NAMESPACE=$namespace WEBHOOK_CONFIG_MODE=service start\""
            echo "[Info] It will take about 5 seconds for the connection to succeed!"

            echo "[Step 10] Running quic client"
            docker run \
                --name ${QUIC_CLIENT_CONTAINER} \
                --rm \
                -v "$CERTS_DIR":/certs \
                $QUIC_CLIENT_IMAGE \
                --server="$loadbalancerIPOrHostName:$quicTunnelPort" \
                --upstream="host.docker.internal:$webhookServerPort" \
                --ca-file=/certs/ca.crt \
                --cert-file=/certs/client.crt \
                --cert-key=/certs/client.key \
                --v=3
        ;;
        [Nn]* ) echo "You need to set \`ignoreResources\` to true and generate the controller installlation first in your extension chart before proceeding!"; exit;;
        * ) echo "Please answer yes or no.";;
    esac
done
fi
