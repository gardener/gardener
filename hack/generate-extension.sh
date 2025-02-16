#!/usr/bin/env bash
# SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

set -e
set -o pipefail

function usage {
    cat <<EOM
Usage:
generate-extension [options]

This script generates a Extension (operator.gardener.cloud) manifest. Kustomize and YQ are required to run this script.

    -h, --help                              Display this help and exit.
    --name                                  Name is the name of the extension.
    --provider-type                         Type of the provider.
    --component-name                        Name of the Kustomize component, one of {provider-extension, network, containerruntime, extension}
    --destination                           The path the extension manifest is written to.
    --extension-oci-repository              URL to OCI image containing the extension chart.
    --admission-runtime-oci-repository      OPTIONAL: URL to OCI image containing the admission runtime chart.
    --admission-application-oci-repository  OPTIONAL: URL to OCI image containing the admission application chart.
    --keep-temp-dir                         OPTIONAL: Set to keep temporary kustomize.yaml file used to generate example.
EOM
    exit 0
}

function cleanup {
  if ! ${KEEP_TEMP_DIR-false}; then
    rm -rf "$TEMP_DIR"
  elif [[ -d $TEMP_DIR ]]; then
    echo "Temp dir: ${TEMP_DIR}"
  fi
}

# Parse the arguments
for i in "$@"; do
    case $i in
    -h|--help)
        usage
        ;;
    --name=*)
        NAME="${i#*=}"
        shift
        ;;
    --provider-type=*)
        PROVIDER_TYPE="${i#*=}"
        shift
        ;;
    --extension-oci-repository=*)
        OCI_REPO="${i#*=}"
        shift
        ;;
    --destination=*)
        DESTINATION="${i#*=}"
        shift
        ;;
    --component-name=*)
        COMPONENT_NAME="${i#*=}"
        shift
        ;;
    --admission-runtime-oci-repository=*)
        ADMISSION_RUNTIME_OCI_REPO="${i#*=}"
        shift
        ;;
    --admission-application-oci-repository=*)
        ADMISSION_APP_OCI_REPO="${i#*=}"
        shift
        ;;
    --keep-temp-dir)
        KEEP_TEMP_DIR=true
        shift
        ;;
    esac
done

( [[ -z "${NAME:-}" ]] || [[ -z "${PROVIDER_TYPE:-}" ]] || [[ -z "${OCI_REPO:-}" ]] || [[ -z "${DESTINATION:-}" ]] || [[ -z "${COMPONENT_NAME:-}" ]] ) && usage

# Register trap
trap cleanup EXIT

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"
TEMP_DIR=$(mktemp -d)
KUSTOMIZE_REAL_PATH="$(realpath --relative-to=$TEMP_DIR $SCRIPT_DIR/..)/example/operator/extensions"
COMPONENT_PATH="${SCRIPT_DIR}/../example/operator/extensions/components/${COMPONENT_NAME}"

if [ ! -d "${COMPONENT_PATH}" ]; then
  echo "Unknown component name"
  exit 1
fi

if [[ -n $VERSION ]]; then
  VERSION_REF="?ref=${VERSION}"
fi

# Kustomization file
KUSTOMIZATION_YAML="apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

resources:
- ${KUSTOMIZE_REAL_PATH}/base

components:
- ${KUSTOMIZE_REAL_PATH}/components/${COMPONENT_NAME}

patches:
- target:
    version: v1alpha1
    group: operator.gardener.cloud
    kind: Extension
    name: example-extension
  patch: |-
    - op: replace
      path: /metadata/name
      value: $NAME
    - op: replace
      path: /spec/deployment/extension/helm/ociRepository/ref
      value: $OCI_REPO
"

# Add admission component
if [[ -n $ADMISSION_RUNTIME_OCI_REPO ]] && [[ -n $ADMISSION_APP_OCI_REPO ]]; then
  KUSTOMIZATION_YAML=$(printf "$KUSTOMIZATION_YAML" | yq ".components += \"${KUSTOMIZE_REAL_PATH}/components/admission\"")

  KUSTOMIZATION_YAML=$(printf "$KUSTOMIZATION_YAML" | yq ".patches[0].patch +=\"
- op: replace
  path: /spec/deployment/admission/runtimeCluster/helm/ociRepository/ref
  value: ${ADMISSION_RUNTIME_OCI_REPO}
- op: replace
  path: /spec/deployment/admission/virtualCluster/helm/ociRepository/ref
  value: ${ADMISSION_APP_OCI_REPO}
\"")
fi

# Create extension type patches.
NUMBER_OF_RESOURCES=$(yq -r '.spec.resources | length' "${COMPONENT_PATH}/extension.yaml")

for ((i = 0; i < $NUMBER_OF_RESOURCES; i++)); do
    KUSTOMIZATION_YAML=$(printf "$KUSTOMIZATION_YAML" | yq ".patches[0].patch +=\"
- op: replace
  path: /spec/resources/${i}/type
  value: ${PROVIDER_TYPE}
\"")
done

printf "$KUSTOMIZATION_YAML" > $TEMP_DIR/kustomization.yaml

# Write result to destination
kustomize build $TEMP_DIR -o $DESTINATION

echo "Successfully generated extension at $DESTINATION"
