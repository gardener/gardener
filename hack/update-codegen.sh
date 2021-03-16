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
export PROJECT_ROOT

# core.gardener.cloud APIs

core_groups() {
  echo "Generating API groups for pkg/apis/core"

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
}
export -f core_groups

# extensions.gardener.cloud APIs

extensions_groups() {
  echo "Generating API groups for pkg/apis/extensions"

  bash "${PROJECT_ROOT}"/vendor/k8s.io/code-generator/generate-groups.sh \
    "deepcopy,client,informer,lister" \
    github.com/gardener/gardener/pkg/client/extensions \
    github.com/gardener/gardener/pkg/apis \
    "extensions:v1alpha1" \
    -h "${PROJECT_ROOT}/hack/LICENSE_BOILERPLATE.txt"
}
export -f extensions_groups

# seedmanagement.gardener.cloud APIs

seedmanagement_groups() {
  echo "Generating API groups for pkg/apis/seedmanagement"

  bash "${PROJECT_ROOT}"/vendor/k8s.io/code-generator/generate-groups.sh \
    "all" \
    github.com/gardener/gardener/pkg/client/seedmanagement \
    github.com/gardener/gardener/pkg/apis \
    "seedmanagement:v1alpha1" \
    -h "${PROJECT_ROOT}/hack/LICENSE_BOILERPLATE.txt"

  bash "${PROJECT_ROOT}"/vendor/k8s.io/code-generator/generate-internal-groups.sh \
    "deepcopy,defaulter,conversion" \
    github.com/gardener/gardener/pkg/client/seedmanagement \
    github.com/gardener/gardener/pkg/apis \
    github.com/gardener/gardener/pkg/apis \
    "seedmanagement:v1alpha1" \
    -h "${PROJECT_ROOT}/hack/LICENSE_BOILERPLATE.txt"
}
export -f seedmanagement_groups

# settings.gardener.cloud APIs

settings_groups() {
  echo "Generating API groups for pkg/apis/settings"

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
}
export -f settings_groups

# operations.gardener.cloud APIs

operations_groups() {
  echo "Generating API groups for pkg/apis/operations"

  bash "${PROJECT_ROOT}"/vendor/k8s.io/code-generator/generate-internal-groups.sh \
    deepcopy,defaulter,client,lister,informer \
    github.com/gardener/gardener/pkg/client/operations \
    github.com/gardener/gardener/pkg/apis \
    github.com/gardener/gardener/pkg/apis \
    "operations:v1alpha1" \
    -h "${PROJECT_ROOT}/hack/LICENSE_BOILERPLATE.txt"

  bash "${PROJECT_ROOT}"/vendor/k8s.io/code-generator/generate-internal-groups.sh \
    conversion \
    github.com/gardener/gardener/pkg/client/operations \
    github.com/gardener/gardener/pkg/apis \
    github.com/gardener/gardener/pkg/apis \
    "operations:v1alpha1" \
    -h "${PROJECT_ROOT}/hack/LICENSE_BOILERPLATE.txt"
}
export -f operations_groups

# authentication.gardener.cloud APIs

authentication_groups() {
  echo "Generating API groups for pkg/apis/authentication"

  bash "${PROJECT_ROOT}"/vendor/k8s.io/code-generator/generate-groups.sh \
    deepcopy,defaulter \
    github.com/gardener/gardener/pkg/client/authentication \
    github.com/gardener/gardener/pkg/apis \
    "authentication:v1alpha1" \
    -h "${PROJECT_ROOT}/hack/LICENSE_BOILERPLATE.txt"

  bash "${PROJECT_ROOT}"/vendor/k8s.io/code-generator/generate-internal-groups.sh \
    deepcopy,defaulter,conversion \
    github.com/gardener/gardener/pkg/client/authentication \
    github.com/gardener/gardener/pkg/apis \
    github.com/gardener/gardener/pkg/apis \
    "authentication:v1alpha1" \
    -h "${PROJECT_ROOT}/hack/LICENSE_BOILERPLATE.txt"
}
export -f authentication_groups

# Componentconfig for controller-manager

controllermanager_groups() {
  echo "Generating API groups for pkg/controllermanager/apis/config"

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
}
export -f controllermanager_groups

# Componentconfig for admission controller

admissioncontroller_groups() {
  echo "Generating API groups for pkg/admissioncontroller/apis/config"

  bash "${PROJECT_ROOT}"/vendor/k8s.io/code-generator/generate-internal-groups.sh \
    deepcopy,defaulter \
    github.com/gardener/gardener/pkg/client/admissioncontrollerconfig \
    github.com/gardener/gardener/pkg/admissioncontroller/apis \
    github.com/gardener/gardener/pkg/admissioncontroller/apis \
    "config:v1alpha1" \
    -h "${PROJECT_ROOT}/hack/LICENSE_BOILERPLATE.txt"

  bash "${PROJECT_ROOT}"/vendor/k8s.io/code-generator/generate-internal-groups.sh \
    conversion \
    github.com/gardener/gardener/pkg/client/admissioncontrollerconfig \
    github.com/gardener/gardener/pkg/admissioncontroller/apis \
    github.com/gardener/gardener/pkg/admissioncontroller/apis \
    "config:v1alpha1" \
    --extra-peer-dirs=github.com/gardener/gardener/pkg/admissioncontroller/apis/config,github.com/gardener/gardener/pkg/admissioncontroller/apis/config/v1alpha1,k8s.io/apimachinery/pkg/apis/meta/v1,k8s.io/apimachinery/pkg/conversion,k8s.io/apimachinery/pkg/runtime,k8s.io/component-base/config,k8s.io/component-base/config/v1alpha1 \
    -h "${PROJECT_ROOT}/hack/LICENSE_BOILERPLATE.txt"
}
export -f admissioncontroller_groups

# Configuration for gardener scheduler

scheduler_groups() {
  echo "Generating API groups for pkg/scheduler/apis/config"

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
}
export -f scheduler_groups

# Componentconfig for gardenlet

gardenlet_groups() {
  echo "Generating API groups for pkg/gardenlet/apis/config"

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
}
export -f gardenlet_groups

# Componentconfig for landscaper-gardenlet

landscapergardenlet_groups() {
  echo "Generating API groups for landscaper/gardenlet/pkg/apis/imports"

  bash "${PROJECT_ROOT}"/vendor/k8s.io/code-generator/generate-internal-groups.sh \
    deepcopy,defaulter \
    github.com/gardener/gardener/pkg/client/componentconfig \
    github.com/gardener/gardener/landscaper/pkg/gardenlet/apis \
    github.com/gardener/gardener/landscaper/pkg/gardenlet/apis \
    "imports:v1alpha1" \
    -h "${PROJECT_ROOT}/hack/LICENSE_BOILERPLATE.txt"

  bash "${PROJECT_ROOT}"/vendor/k8s.io/code-generator/generate-internal-groups.sh \
    conversion \
    github.com/gardener/gardener/pkg/client/componentconfig \
    github.com/gardener/gardener/landscaper/pkg/gardenlet/apis \
    github.com/gardener/gardener/landscaper/pkg/gardenlet/apis \
    "imports:v1alpha1" \
    --extra-peer-dirs=github.com/gardener/gardener/pkg/gardenlet/apis/config,github.com/gardener/landscaper/apis/core/v1alpha1,k8s.io/apimachinery/pkg/apis/meta/v1,k8s.io/apimachinery/pkg/conversion,k8s.io/apimachinery/pkg/runtime \
    -h "${PROJECT_ROOT}/hack/LICENSE_BOILERPLATE.txt"
}
export -f landscapergardenlet_groups

# Componentconfig for admission plugins

shoottolerationrestriction_groups() {
  echo "Generating API groups for plugin/pkg/shoot/tolerationrestriction/apis/shoottolerationrestriction"

  bash "${PROJECT_ROOT}"/vendor/k8s.io/code-generator/generate-internal-groups.sh \
    deepcopy,defaulter \
    github.com/gardener/gardener/pkg/client/componentconfig \
    github.com/gardener/gardener/plugin/pkg/shoot/tolerationrestriction/apis \
    github.com/gardener/gardener/plugin/pkg/shoot/tolerationrestriction/apis \
    "shoottolerationrestriction:v1alpha1" \
    -h "${PROJECT_ROOT}/hack/LICENSE_BOILERPLATE.txt"

  bash "${PROJECT_ROOT}"/vendor/k8s.io/code-generator/generate-internal-groups.sh \
    conversion \
    github.com/gardener/gardener/pkg/client/componentconfig \
    github.com/gardener/gardener/plugin/pkg/shoot/tolerationrestriction/apis \
    github.com/gardener/gardener/plugin/pkg/shoot/tolerationrestriction/apis \
    "shoottolerationrestriction:v1alpha1" \
    --extra-peer-dirs=github.com/gardener/gardener/plugin/pkg/shoot/tolerationrestriction/apis/shoottolerationrestriction,github.com/gardener/gardener/plugin/pkg/shoot/tolerationrestriction/apis/shoottolerationrestriction/v1alpha1,k8s.io/apimachinery/pkg/apis/meta/v1,k8s.io/apimachinery/pkg/conversion,k8s.io/apimachinery/pkg/runtime,k8s.io/component-base/config,k8s.io/component-base/config/v1alpha1 \
    -h "${PROJECT_ROOT}/hack/LICENSE_BOILERPLATE.txt"
}
export -f shoottolerationrestriction_groups

# OpenAPI definitions

openapi_definitions() {
  echo "Generating openapi definitions"
  rm -Rf ./${PROJECT_ROOT}/openapi/openapi_generated.go
  go install ./${PROJECT_ROOT}/vendor/k8s.io/kube-openapi/cmd/openapi-gen
  ${GOPATH}/bin/openapi-gen "$@" \
    --v 1 \
    --logtostderr \
    --input-dirs=github.com/gardener/gardener/pkg/apis/authentication/v1alpha1 \
    --input-dirs=github.com/gardener/gardener/pkg/apis/core/v1alpha1 \
    --input-dirs=github.com/gardener/gardener/pkg/apis/core/v1beta1 \
    --input-dirs=github.com/gardener/gardener/pkg/apis/settings/v1alpha1 \
    --input-dirs=github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1 \
    --input-dirs=github.com/gardener/gardener/pkg/apis/operations/v1alpha1 \
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
    --report-filename=${PROJECT_ROOT}/pkg/openapi/api_violations.report \
    --output-package=github.com/gardener/gardener/pkg/openapi \
    -h "${PROJECT_ROOT}/hack/LICENSE_BOILERPLATE.txt"
}
export -f openapi_definitions

if [[ $# -gt 0 && "$1" == "--parallel" ]]; then
  shift 1
  parallel --will-cite ::: \
    authentication_groups \
    core_groups \
    extensions_groups \
    seedmanagement_groups \
    operations_groups \
    settings_groups \
    controllermanager_groups \
    admissioncontroller_groups \
    scheduler_groups \
    gardenlet_groups \
    shoottolerationrestriction_groups \
    landscapergardenlet_groups
else
  authentication_groups
  core_groups
  extensions_groups
  seedmanagement_groups
  operations_groups
  settings_groups
  controllermanager_groups
  admissioncontroller_groups
  scheduler_groups
  gardenlet_groups
  shoottolerationrestriction_groups
  landscapergardenlet_groups
fi

openapi_definitions "$@"
