#!/bin/bash
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

set -o errexit
set -o nounset
set -o pipefail

rm -f ${GOPATH}/bin/*-gen

CURRENT_DIR=$(dirname $0)
PROJECT_ROOT="${CURRENT_DIR}"/..

# core.gardener.cloud APIs

bash "${PROJECT_ROOT}"/vendor/k8s.io/code-generator/generate-internal-groups.sh \
  deepcopy,defaulter,client,lister,informer \
  github.com/gardener/gardener/pkg/client/core \
  github.com/gardener/gardener/pkg/apis \
  github.com/gardener/gardener/pkg/apis \
  "core:v1alpha1,v1beta1" \
  -h "${PROJECT_ROOT}/hack/LICENSE_BOILERPLATE.txt"

bash "${PROJECT_ROOT}"/vendor/k8s.io/code-generator/generate-internal-groups.sh \
  conversion \
  github.com/gardener/gardener/pkg/client/core \
  github.com/gardener/gardener/pkg/apis \
  github.com/gardener/gardener/pkg/apis \
  "core:v1alpha1,v1beta1" \
  -h "${PROJECT_ROOT}/hack/LICENSE_BOILERPLATE.txt"

# extensions.gardener.cloud APIs

bash "${PROJECT_ROOT}"/vendor/k8s.io/code-generator/generate-groups.sh \
  "deepcopy,client,informer,lister" \
  github.com/gardener/gardener/pkg/client/extensions \
  github.com/gardener/gardener/pkg/apis \
  "extensions:v1alpha1" \
  -h "${PROJECT_ROOT}/hack/LICENSE_BOILERPLATE.txt"

# settings.gardener.cloud APIs

bash "${PROJECT_ROOT}"/vendor/k8s.io/code-generator/generate-groups.sh \
  "all" \
  github.com/gardener/gardener/pkg/client/settings \
  github.com/gardener/gardener/pkg/apis \
  "settings:v1alpha1" \
  -h "${PROJECT_ROOT}/hack/LICENSE_BOILERPLATE.txt"

bash "${PROJECT_ROOT}"/vendor/k8s.io/code-generator/generate-internal-groups.sh \
  "deepcopy,defaulter,conversion" \
  github.com/gardener/gardener/pkg/client/settings \
  github.com/gardener/gardener/pkg/apis \
  github.com/gardener/gardener/pkg/apis \
  "settings:v1alpha1" \
  -h "${PROJECT_ROOT}/hack/LICENSE_BOILERPLATE.txt"

# Componentconfig for controller-manager

bash "${PROJECT_ROOT}"/vendor/k8s.io/code-generator/generate-internal-groups.sh \
  deepcopy,defaulter \
  github.com/gardener/gardener/pkg/client/componentconfig \
  github.com/gardener/gardener/pkg/controllermanager/apis \
  github.com/gardener/gardener/pkg/controllermanager/apis \
  "config:v1alpha1" \
  -h "${PROJECT_ROOT}/hack/LICENSE_BOILERPLATE.txt"

bash "${PROJECT_ROOT}"/vendor/k8s.io/code-generator/generate-internal-groups.sh \
  conversion \
  github.com/gardener/gardener/pkg/client/componentconfig \
  github.com/gardener/gardener/pkg/controllermanager/apis \
  github.com/gardener/gardener/pkg/controllermanager/apis \
  "config:v1alpha1" \
  --extra-peer-dirs=github.com/gardener/gardener/pkg/controllermanager/apis/config,github.com/gardener/gardener/pkg/controllermanager/apis/config/v1alpha1,k8s.io/apimachinery/pkg/apis/meta/v1,k8s.io/apimachinery/pkg/conversion,k8s.io/apimachinery/pkg/runtime,k8s.io/component-base/config,k8s.io/component-base/config/v1alpha1 \
  -h "${PROJECT_ROOT}/hack/LICENSE_BOILERPLATE.txt"

# Configuration for gardener scheduler

bash "${PROJECT_ROOT}"/vendor/k8s.io/code-generator/generate-internal-groups.sh \
  deepcopy,defaulter \
  github.com/gardener/gardener/pkg/scheduler/client \
  github.com/gardener/gardener/pkg/scheduler/apis \
  github.com/gardener/gardener/pkg/scheduler/apis \
  "config:v1alpha1" \
  -h "${PROJECT_ROOT}/hack/LICENSE_BOILERPLATE.txt"

bash "${PROJECT_ROOT}"/vendor/k8s.io/code-generator/generate-internal-groups.sh \
  conversion \
  github.com/gardener/gardener/pkg/scheduler/client \
  github.com/gardener/gardener/pkg/scheduler/apis \
  github.com/gardener/gardener/pkg/scheduler/apis \
  "config:v1alpha1" \
  --extra-peer-dirs=github.com/gardener/gardener/pkg/scheduler/apis/config,github.com/gardener/gardener/pkg/scheduler/apis/config/v1alpha1,k8s.io/apimachinery/pkg/apis/meta/v1,k8s.io/apimachinery/pkg/conversion,k8s.io/apimachinery/pkg/runtime,k8s.io/component-base/config,k8s.io/component-base/config/v1alpha1 \
  -h "${PROJECT_ROOT}/hack/LICENSE_BOILERPLATE.txt"

# Componentconfig for gardenlet

bash "${PROJECT_ROOT}"/vendor/k8s.io/code-generator/generate-internal-groups.sh \
  deepcopy,defaulter \
  github.com/gardener/gardener/pkg/client/componentconfig \
  github.com/gardener/gardener/pkg/gardenlet/apis \
  github.com/gardener/gardener/pkg/gardenlet/apis \
  "config:v1alpha1" \
  -h "${PROJECT_ROOT}/hack/LICENSE_BOILERPLATE.txt"

bash "${PROJECT_ROOT}"/vendor/k8s.io/code-generator/generate-internal-groups.sh \
  conversion \
  github.com/gardener/gardener/pkg/client/componentconfig \
  github.com/gardener/gardener/pkg/gardenlet/apis \
  github.com/gardener/gardener/pkg/gardenlet/apis \
  "config:v1alpha1" \
  --extra-peer-dirs=github.com/gardener/gardener/pkg/gardenlet/apis/config,github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1,k8s.io/apimachinery/pkg/apis/meta/v1,k8s.io/apimachinery/pkg/conversion,k8s.io/apimachinery/pkg/runtime,k8s.io/component-base/config,k8s.io/component-base/config/v1alpha1 \
  -h "${PROJECT_ROOT}/hack/LICENSE_BOILERPLATE.txt"

rm -Rf ./${PROJECT_ROOT}/openapi/openapi_generated.go
go install ./${PROJECT_ROOT}/vendor/k8s.io/kube-openapi/cmd/openapi-gen

echo "Generating openapi definitions"
${GOPATH}/bin/openapi-gen "$@" \
  --v 1 \
  --logtostderr \
  --input-dirs=github.com/gardener/gardener/pkg/apis/core/v1alpha1 \
  --input-dirs=github.com/gardener/gardener/pkg/apis/core/v1beta1 \
  --input-dirs=github.com/gardener/gardener/pkg/apis/settings/v1alpha1 \
  --input-dirs=k8s.io/api/core/v1 \
  --input-dirs=k8s.io/api/rbac/v1 \
  --input-dirs=k8s.io/apimachinery/pkg/apis/meta/v1 \
  --input-dirs=k8s.io/apimachinery/pkg/api/resource \
  --input-dirs=k8s.io/apimachinery/pkg/types \
  --input-dirs=k8s.io/apimachinery/pkg/version \
  --input-dirs=k8s.io/apimachinery/pkg/runtime \
  --input-dirs=k8s.io/apimachinery/pkg/util/intstr \
  --report-filename=${PROJECT_ROOT}/pkg/openapi/api_violations.report \
  --output-package=github.com/gardener/gardener/pkg/openapi \
  -h "${PROJECT_ROOT}/hack/LICENSE_BOILERPLATE.txt"

echo
echo "NOTE: If you changed the API then consider updating the example manifests.".
