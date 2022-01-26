#!/bin/bash
#
# Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

if [ -z "$CURRENT_DIR" ]; then
    CURRENT_DIR=$(readlink -f $(dirname $0))
fi

if [ -z "$PROJECT_ROOT" ]; then
    PROJECT_ROOT="$(realpath ${CURRENT_DIR}/../../../..)"
else
    # if sourced from another script that sets the PROJECT_ROOT variable, makes sure it is a relative path
    PROJECT_ROOT=./${PROJECT_ROOT}
fi

rm -Rf ${PROJECT_ROOT}/landscaper/pkg/controlplane/generate/openapi/openapi_generated.go

# For the given package and its transitive dependencies, generate OpenAPI producing go-code for types annotated
# with +k8s:openapi-gen=true
# This tag however does not exist for all dependencies.
# In such cases, no OpenAPI is generated & the blueprint
# generation (./generate.go) uses a placeholder for the missing JSONSchema.
go run ${PROJECT_ROOT}/landscaper/common/generate/openapi \
  --root-directory ${PROJECT_ROOT} \
  --input-directories ${PROJECT_ROOT}/landscaper/pkg/controlplane/apis/imports/v1alpha1,${PROJECT_ROOT}/landscaper/pkg/controlplane/apis/exports \
  --output-path ${PROJECT_ROOT}/landscaper/pkg/controlplane/generate \
  --packages github.com/gardener/gardener/landscaper/pkg/controlplane/apis/imports/v1alpha1,github.com/gardener/gardener/landscaper/pkg/controlplane/apis/exports \
  --verbosity 1
