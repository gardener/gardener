#!/bin/bash
#
# Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

# Generates a minimal standalone installation that can be used in development/testing scenarios
# Prerequisites:
# - a target with the name "runtimeCluster" has to be created in the same namespace
# - the virtual-garden component needs to be run before so that the data imports are satisfied

set -e

EXPORT_DIR=''
EFFECTIVE_VERSION=''

while test $# -gt 0; do
             case "$1" in
                  --export-dir)
                      shift
                      EXPORT_DIR=$1
                      shift
                      ;;
                  --version)
                      shift
                      EFFECTIVE_VERSION=$1
                      shift
                      ;;
            esac
    done

if [ -z "$EXPORT_DIR" ]
  then
     echo "an export directory must be provided"
     exit 1
  fi

if [ -z "$EFFECTIVE_VERSION" ]
  then
     echo "an effective version must be provided"
     exit 1
  fi

INSTALLATION_PATH="${EXPORT_DIR}/controlplane-installation.yaml"

cat << EOF > ${INSTALLATION_PATH}
apiVersion: landscaper.gardener.cloud/v1alpha1
kind: Installation
metadata:
  name: gardener-controlplane
spec:
  componentDescriptor:
    ref:
      repositoryContext:
        type: ociRegistry
        baseUrl: eu.gcr.io/gardener-project/development
      componentName: github.com/gardener/gardener
      version: ${EFFECTIVE_VERSION}

  blueprint:
    ref:
      resourceName: controlplane-blueprint

  imports:
    targets:
      # Important: a target with the name "runtimeCluster" has to be created in the same namespace
      # before using this installation
      - name: runtimeCluster
        target: '#runtime-cluster'
      - name: virtualGardenCluster
        # match export of virtual garden component
        target: 'virtual-garden-cluster'
    data:
      #  ----- Data refs: importing from virtual garden component -----
      - name: etcdUrl
        dataRef: "etcd-url"

      - name: etcdCaBundle
        dataRef: "etcd-ca-pem"

      - name: etcdClientCert
        dataRef: "etcd-client-tls-pem"

      - name: etcdClientKey
        dataRef: "etcd-client-tls-key-pem"

  # static data to not require to import config map
  importDataMappings:
    virtualGarden:
      enabled: true
      clusterIP: "100.64.10.10"

    internalDomain:
      provider: aws-route53
      domain: internal.test.domain
      credentials:
        # fake credentials just for testing
        AWS_ACCESS_KEY_ID: ZHVtbXk=
        AWS_SECRET_ACCESS_KEY: ZHVtbXk=

    gardenerAdmissionController:
      enabled: true

  exports:
    data:
    - name: gardenerAPIServerCA
      dataRef: "gardener-apiserver-ca"
    - name: gardenerAPIServerTLSServing
      dataRef: "gardener-apiserver-tls-serving"
    - name: gardenerAdmissionControllerCA
      dataRef: "gardener-admission-controller-ca"
    - name: gardenerAdmissionControllerTLSServing
      dataRef: "gardener-admission-controller-tls-serving"
    - name: gardenerControllerManagerTLSServing
      dataRef: "gardener-controller-manager-tls-serving"
    - name: gardenerIdentity
      dataRef: "gardener-identity"
    - name: openVPNDiffieHellmanKey
      dataRef: "openvpn-diffie-hellman-key"
EOF

echo "[Landscaper Controlplane]: Installation exported to ${INSTALLATION_PATH}"

