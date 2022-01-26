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

set -e

DEFAULT_REPO_CTX="eu.gcr.io/gardener-project/development"
DEFAULT_BLUEPRINT_RESOURCES_FILE_PATH='landscaper/resources.yaml'
DEFAULT_OCI_IMAGE_RESOURCES_FILE_PATH='landscaper/hack/resources.yaml'

COMPONENT_NAME=''
EFFECTIVE_VERSION=''
REPO_CTX=''
# resource file contains the resources (OCI images, blueprint) to enhance the component descriptor with
BLUEPRINT_RESOURCES_FILE_PATH=''
OCI_IMAGE_RESOURCES_FILE_PATH=''

while test $# -gt 0; do
             case "$1" in
                  --component-name)
                      shift
                      COMPONENT_NAME=$1
                      shift
                      ;;
                  --version)
                      shift
                      EFFECTIVE_VERSION=$1
                      shift
                      ;;
                  --repo-context)
                      shift
                      REPO_CTX=$1
                      shift
                      ;;
                  --blueprint-resources-file-path)
                      shift
                      BLUEPRINT_RESOURCES_FILE_PATH=$1
                      shift
                      ;;
                  --oci-image-resources-file-path)
                      shift
                      OCI_IMAGE_RESOURCES_FILE_PATH=$1
                      shift
                      ;;
            esac
    done


if [ -z "$COMPONENT_NAME" ]
  then
     echo "a component name has to be provided. Example: github.com/gardener/gardener"
     exit 1
  fi

if [ -z "$EFFECTIVE_VERSION" ]
  then
     echo "an effective version has to be provided"
     exit 1
  fi

if [ -z "$REPO_CTX" ]
  then
     REPO_CTX=$DEFAULT_REPO_CTX
  fi

if [ -z "$BLUEPRINT_RESOURCES_FILE_PATH" ]
  then
     BLUEPRINT_RESOURCES_FILE_PATH=$DEFAULT_BLUEPRINT_RESOURCES_FILE_PATH
  fi

if [ -z "$OCI_IMAGE_RESOURCES_FILE_PATH" ]
  then
     OCI_IMAGE_RESOURCES_FILE_PATH=$DEFAULT_OCI_IMAGE_RESOURCES_FILE_PATH
  fi

if ! which component-cli 1>/dev/null; then
  echo -n "component-cli is required to generate the component descriptors"
  exit 1
fi

echo "> Generate Component Descriptor for component ${COMPONENT_NAME} with version ${EFFECTIVE_VERSION}"

# temporary storage for the component descriptor
COMPONENT_ARCHIVE_PATH="$(mktemp -d)"

# path of the generate component descriptor
COMPONENT_DESCRIPTOR_BASE_DEFINITION_PATH="${COMPONENT_ARCHIVE_PATH}/component-descriptor.yaml"

# Create empty component descriptor base definition in the tmp. directory
component-cli ca create "${COMPONENT_ARCHIVE_PATH}" \
    --component-name=${COMPONENT_NAME} \
    --component-version=${EFFECTIVE_VERSION} \
    --repo-ctx=${REPO_CTX}

# enhance the component descriptor with blueprint resource
component-cli component-archive resources add \
"${COMPONENT_ARCHIVE_PATH}" \
"$BLUEPRINT_RESOURCES_FILE_PATH"

# temporary storage for oci images resources
RESOURCES_FIlE_PATH="$COMPONENT_ARCHIVE_PATH/resources.yaml"

# Add images of components. This has to be done manually as in the CI
# script at .ci/component_descriptor, this is done by the CI pipeline that builds the image.
# NOTE: if you also want to test the Gardener control plane images at their current version (GAPI etc.),
# then replace the `latest` tag with ${EFFECTIVE_VERSION}
cat << EOF >> ${RESOURCES_FIlE_PATH}
---
type: ociImage
name: landscaper-controlplane
relation: local
access:
  type: ociRegistry
  imageReference: eu.gcr.io/gardener-project/gardener/landscaper-controlplane:${EFFECTIVE_VERSION}
---
type: ociImage
name: landscaper-gardenlet
relation: local
access:
  type: ociRegistry
  imageReference: eu.gcr.io/gardener-project/gardener/landscaper-controlplane:${EFFECTIVE_VERSION}
---
type: ociImage
name: apiserver
relation: local
access:
  type: ociRegistry
  imageReference: eu.gcr.io/gardener-project/gardener/apiserver:latest
---
type: ociImage
name: controller-manager
relation: local
access:
  type: ociRegistry
  imageReference: eu.gcr.io/gardener-project/gardener/controller-manager:latest
---
type: ociImage
name: admission-controller
relation: local
access:
  type: ociRegistry
  imageReference: eu.gcr.io/gardener-project/gardener/admission-controller:latest
---
type: ociImage
name: scheduler
relation: local
access:
  type: ociRegistry
  imageReference: eu.gcr.io/gardener-project/gardener/scheduler:latest
...
EOF

# enhance the component descriptor with OCI image resources
# to log the full component descriptor use "cat $COMPONENT_DESCRIPTOR_BASE_DEFINITION_PATH"
component-cli component-archive resources add \
"${COMPONENT_ARCHIVE_PATH}" \
"$RESOURCES_FIlE_PATH"

echo "> Creating ctf"

# temporary storage for the ctf archive
CTF_ARCHIVE_PATH="$(mktemp -d)"/ctf.tar

# Create CTF tar archive at CTF_PATH based on directory in component archive layout (packed automatically)
# Pushed by CI to private registry if CTF is found at CTF_PATH.
component-cli ctf add "${CTF_ARCHIVE_PATH}" -f "${COMPONENT_ARCHIVE_PATH}"

echo "> Pushing ctf to registry"
component-cli ctf push --repo-ctx=${REPO_CTX} "${CTF_ARCHIVE_PATH}"

echo "To view the component descriptor: \"component-cli component-archive remote get ${REPO_CTX} ${COMPONENT_NAME} ${EFFECTIVE_VERSION}\""
