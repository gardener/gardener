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
# - a target with the podCIDR "runtimeCluster" has to be created in the same namespace
# - the virtual-garden component needs to be run before so that the data imports are satisfied
# - a Kubernetes cluster that can be registered as Seed cluster

set -o errexit
set -o nounset
set -o pipefail

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

read -p "[Landscaper Gardenlet]: Enter Seed's ingress domain: " ingressDomain
if [ -z "$ingressDomain" ]
  then
     echo "the Seed's ingress domain must be provided"
     exit 1
  fi

kubeconfigPath=$(printenv KUBECONFIG)
if [ -z "$kubeconfigPath" ]
  then
     echo "please set the KUBECONFIG environment variable in the current shell to the cluster to be registered as Seed"
     exit 1
  fi

echo "[Landscaper Gardenlet]: Using cluster with kubeconfig at $kubeconfigPath to be registered as Seed"
seedKubeconfig=$(cat $kubeconfigPath | yaml2json)

defaultPodCIDR="100.96.0.0/11"
read -p "[Landscaper Gardenlet]: Enter Seed's pod CIDR [$defaultPodCIDR]: " podCIDR
podCIDR=${podCIDR:-"$defaultPodCIDR"}

defaultServiceCIDR="100.64.0.0/13"
read -p "[Landscaper Gardenlet]: Enter Seed's pod CIDR [$defaultServiceCIDR]: " serviceCIDR
serviceCIDR=${serviceCIDR:-"$defaultServiceCIDR"}

defaultNodeCIDR="10.250.0.0/16"
read -p "[Landscaper Gardenlet]: Enter Seed's node CIDR [$defaultNodeCIDR]: " nodeCIDR
nodeCIDR=${nodeCIDR:-"$defaultNodeCIDR"}

defaultProvider="aws"
read -p "[Landscaper Gardenlet]: Enter Seed's cloud provider type [$defaultProvider]: " provider
provider=${provider:-"$defaultProvider"}

defaultRegion="eu-west-1"
read -p "[Landscaper Gardenlet]: Enter the Seed's region [$defaultRegion]: " region
region=${region:-"$defaultRegion"}

INSTALLATION_PATH="${EXPORT_DIR}/gardenlet-installation.yaml"
TARGET_PATH="${EXPORT_DIR}/gardenlet-seed-target.yaml"

cat << EOF > ${TARGET_PATH}
apiVersion: landscaper.gardener.cloud/v1alpha1
kind: Target
metadata:
  name: gardenlet-seed-soil
spec:
  type: landscaper.gardener.cloud/kubernetes-cluster
  config:
    kubeconfig: |
      $seedKubeconfig
EOF

cat << EOF > ${INSTALLATION_PATH}
apiVersion: landscaper.gardener.cloud/v1alpha1
kind: Installation
metadata:
  name: gardener-gardenlet
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
      resourceName: gardenlet-blueprint

  imports:
    targets:
      - name: seedCluster
        target: '#gardenlet-seed-soil'
      - name: gardenCluster
        # match export of virtual garden component
        # prerequisite
        target: 'virtual-garden-cluster'

  # static data to not require to import config map
  importDataMappings:
    deploymentConfiguration:
      replicaCount: 1
      resources:
        requests:
          cpu: 70m
          memory: 100Mi

    componentConfiguration:
      apiVersion: gardenlet.config.gardener.cloud/v1alpha1
      kind: GardenletConfiguration
      server:
        https:
          bindAddress: 0.0.0.0
          port: 2720
      seedConfig:
        metadata:
          name: soil
        spec:
          dns:
            ingressDomain: $ingressDomain
          networks:
            pods: $podCIDR
            nodes: $nodeCIDR
            services: $serviceCIDR
          provider:
            region: $region
            type: $provider

EOF

echo "[Landscaper Gardenlet]: Target for Seed cluster exported to ${TARGET_PATH}"
echo "[Landscaper Gardenlet]: Installation exported to ${INSTALLATION_PATH}"
