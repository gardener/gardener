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
fi

echo $PROJECT_ROOT

rm -Rf ${PROJECT_ROOT}/landscaper/pkg/controlplane/generate/openapi/openapi_generated.go

# Packages have to include the tag +k8s:openapi-gen=true for the types to be included in the generation
# However, this is not done by all dependencies. In such cases, no OpenAPI is generated & the blueprint
# generation (./generate.go) uses a placeholder for the missing JSONSchema.
openapi-gen \
  --v 1 \
  --logtostderr \
  --input-dirs=github.com/gardener/gardener/landscaper/pkg/controlplane/apis/imports/v1alpha1 \
  --input-dirs=github.com/gardener/landscaper/apis/core/v1alpha1 \
  --input-dirs=github.com/gardener/hvpa-controller/api/v1alpha1 \
  --input-dirs=k8s.io/api/core/v1 \
  --input-dirs=k8s.io/api/rbac/v1 \
  --input-dirs=k8s.io/api/autoscaling/v1 \
  --input-dirs=k8s.io/api/networking/v1 \
  --input-dirs=k8s.io/apimachinery/pkg/apis/meta/v1 \
  --input-dirs=k8s.io/apimachinery/pkg/api/resource \
  --input-dirs=k8s.io/apimachinery/pkg/types \
  --input-dirs=k8s.io/apimachinery/pkg/version \
  --input-dirs=k8s.io/apimachinery/pkg/runtime \
  --input-dirs=k8s.io/apimachinery/pkg/util/intstr \
  --input-dirs=k8s.io/apiserver/pkg/apis/audit/v1 \
  --report-filename=${PROJECT_ROOT}/landscaper/pkg/controlplane/generate/openapi/api_violations.report \
  --output-package=openapi \
  --output-base=${PROJECT_ROOT}/landscaper/pkg/controlplane/generate \
  -h "${PROJECT_ROOT}/hack/LICENSE_BOILERPLATE.txt"
