#!/usr/bin/env bash
#
# SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

set -o errexit
set -o nounset
set -o pipefail

CODEGEN_GROUPS=""
MODE="sequential"
AVAILABLE_CODEGEN_OPTIONS=(
  "authentication_groups"
  "core_groups"
  "extensions_groups"
  "resources_groups"
  "operator_groups"
  "seedmanagement_groups"
  "operations_groups"
  "settings_groups"
  "security_groups"
  "operatorconfig_groups"
  "controllermanager_groups"
  "admissioncontroller_groups"
  "scheduler_groups"
  "gardenlet_groups"
  "resourcemanager_groups"
  "shootresourcereservation_groups"
  "shoottolerationrestriction_groups"
  "shootdnsrewriting_groups"
  "provider_local_groups"
  "extensions_config_groups"
  "nodeagent_groups"
)

# setup virtual GOPATH
source $(dirname $0)/vgopath-setup.sh

CODE_GEN_DIR=$(go list -m -f '{{.Dir}}' k8s.io/code-generator)
source "${CODE_GEN_DIR}/kube_codegen.sh"

rm -f ${GOPATH}/bin/*-gen

CURRENT_DIR=$(dirname $0)
PROJECT_ROOT="${CURRENT_DIR}"/..
export PROJECT_ROOT

parse_flags() {
  while test $# -gt 0; do
    case "$1" in
      --mode)
        shift
        if [[ -n "$1" ]]; then
        MODE="$1"
        fi
        ;;
      --groups)
        shift
        CODEGEN_GROUPS="${1:-$CODEGEN_GROUPS}"
        ;;
      *)
        echo "Unknown argument: $1"
        exit 1
        ;;
    esac
    shift
  done
}

# core.gardener.cloud APIs

core_groups() {
  echo "Generating API groups for pkg/apis/core"
  
  kube::codegen::gen_helpers \
    --boilerplate "${PROJECT_ROOT}/hack/LICENSE_BOILERPLATE.txt" \
    "${PROJECT_ROOT}/pkg/apis/core"
  
  kube::codegen::gen_client \
    --with-watch \
    --one-input-api "core" \
    --output-dir "${PROJECT_ROOT}/pkg/client/core" \
    --output-pkg "github.com/gardener/gardener/pkg/client/core" \
    --boilerplate "${PROJECT_ROOT}/hack/LICENSE_BOILERPLATE.txt" \
    "${PROJECT_ROOT}/pkg/apis"
}
export -f core_groups

# extensions.gardener.cloud APIs

extensions_groups() {
  echo "Generating API groups for pkg/apis/extensions"
  
  kube::codegen::gen_helpers \
    --boilerplate "${PROJECT_ROOT}/hack/LICENSE_BOILERPLATE.txt" \
    "${PROJECT_ROOT}/pkg/apis/extensions"
}
export -f extensions_groups

# resources.gardener.cloud APIs

resources_groups() {
  echo "Generating API groups for pkg/apis/resources"
  
  kube::codegen::gen_helpers \
    --boilerplate "${PROJECT_ROOT}/hack/LICENSE_BOILERPLATE.txt" \
    "${PROJECT_ROOT}/pkg/apis/resources"
}
export -f resources_groups

# operator.gardener.cloud APIs

operator_groups() {
  echo "Generating API groups for pkg/apis/operator"
  
  kube::codegen::gen_helpers \
    --boilerplate "${PROJECT_ROOT}/hack/LICENSE_BOILERPLATE.txt" \
    "${PROJECT_ROOT}/pkg/apis/operator"
}
export -f operator_groups

# seedmanagement.gardener.cloud APIs

seedmanagement_groups() {
  echo "Generating API groups for pkg/apis/seedmanagement"
  
  kube::codegen::gen_helpers \
    --boilerplate "${PROJECT_ROOT}/hack/LICENSE_BOILERPLATE.txt" \
    "${PROJECT_ROOT}/pkg/apis/seedmanagement"
  
  kube::codegen::gen_client \
    --with-watch \
    --one-input-api "seedmanagement" \
    --output-dir "${PROJECT_ROOT}/pkg/client/seedmanagement" \
    --output-pkg "github.com/gardener/gardener/pkg/client/seedmanagement" \
    --boilerplate "${PROJECT_ROOT}/hack/LICENSE_BOILERPLATE.txt" \
    "${PROJECT_ROOT}/pkg/apis"
}
export -f seedmanagement_groups

# settings.gardener.cloud APIs

settings_groups() {
  echo "Generating API groups for pkg/apis/settings"
  
  kube::codegen::gen_helpers \
    --boilerplate "${PROJECT_ROOT}/hack/LICENSE_BOILERPLATE.txt" \
    "${PROJECT_ROOT}/pkg/apis/settings"
  
  kube::codegen::gen_client \
    --with-watch \
    --one-input-api "settings" \
    --output-dir "${PROJECT_ROOT}/pkg/client/settings" \
    --output-pkg "github.com/gardener/gardener/pkg/client/settings" \
    --boilerplate "${PROJECT_ROOT}/hack/LICENSE_BOILERPLATE.txt" \
    "${PROJECT_ROOT}/pkg/apis"
}
export -f settings_groups

# security.gardener.cloud APIs

security_groups() {
  echo "Generating API groups for pkg/apis/security"
  
  kube::codegen::gen_helpers \
    --boilerplate "${PROJECT_ROOT}/hack/LICENSE_BOILERPLATE.txt" \
    "${PROJECT_ROOT}/pkg/apis/security"
  
  kube::codegen::gen_client \
    --with-watch \
    --one-input-api "security" \
    --output-dir "${PROJECT_ROOT}/pkg/client/security" \
    --output-pkg "github.com/gardener/gardener/pkg/client/security" \
    --boilerplate "${PROJECT_ROOT}/hack/LICENSE_BOILERPLATE.txt" \
    "${PROJECT_ROOT}/pkg/apis"
}
export -f security_groups

# operations.gardener.cloud APIs

operations_groups() {
  echo "Generating API groups for pkg/apis/operations"
  
  kube::codegen::gen_helpers \
    --boilerplate "${PROJECT_ROOT}/hack/LICENSE_BOILERPLATE.txt" \
    "${PROJECT_ROOT}/pkg/apis/operations"
}
export -f operations_groups

# authentication.gardener.cloud APIs

authentication_groups() {
  echo "Generating API groups for pkg/apis/authentication"
  
  kube::codegen::gen_helpers \
    --boilerplate "${PROJECT_ROOT}/hack/LICENSE_BOILERPLATE.txt" \
    "${PROJECT_ROOT}/pkg/apis/authentication"
}
export -f authentication_groups

# Componentconfig for operator

operatorconfig_groups() {
  echo "Generating API groups for pkg/operator/apis/config"
  
  kube::codegen::gen_helpers \
    --boilerplate "${PROJECT_ROOT}/hack/LICENSE_BOILERPLATE.txt" \
    --extra-peer-dir github.com/gardener/gardener/pkg/operator/apis/config \
    --extra-peer-dir github.com/gardener/gardener/pkg/operator/apis/config/v1alpha1 \
    --extra-peer-dir k8s.io/apimachinery/pkg/apis/meta/v1 \
    --extra-peer-dir k8s.io/apimachinery/pkg/conversion \
    --extra-peer-dir k8s.io/apimachinery/pkg/runtime \
    --extra-peer-dir k8s.io/component-base/config \
    --extra-peer-dir k8s.io/component-base/config/v1alpha1 \
    "${PROJECT_ROOT}/pkg/operator/apis/config"
}
export -f operatorconfig_groups

# Componentconfig for controller-manager

controllermanager_groups() {
  echo "Generating API groups for pkg/controllermanager/apis/config"
  
  kube::codegen::gen_helpers \
    --boilerplate "${PROJECT_ROOT}/hack/LICENSE_BOILERPLATE.txt" \
    --extra-peer-dir github.com/gardener/gardener/pkg/controllermanager/apis/config \
    --extra-peer-dir github.com/gardener/gardener/pkg/controllermanager/apis/config/v1alpha1 \
    --extra-peer-dir k8s.io/apimachinery/pkg/apis/meta/v1 \
    --extra-peer-dir k8s.io/apimachinery/pkg/conversion \
    --extra-peer-dir k8s.io/apimachinery/pkg/runtime \
    --extra-peer-dir k8s.io/component-base/config \
    --extra-peer-dir k8s.io/component-base/config/v1alpha1 \
    "${PROJECT_ROOT}/pkg/controllermanager/apis/config"
}
export -f controllermanager_groups

# Componentconfig for admission controller

admissioncontroller_groups() {
  echo "Generating API groups for pkg/admissioncontroller/apis/config"
  
  kube::codegen::gen_helpers \
    --boilerplate "${PROJECT_ROOT}/hack/LICENSE_BOILERPLATE.txt" \
    --extra-peer-dir github.com/gardener/gardener/pkg/admissioncontroller/apis/config/v1alpha1 \
    --extra-peer-dir k8s.io/apimachinery/pkg/apis/meta/v1,k8s.io/apimachinery/pkg/conversion \
    --extra-peer-dir k8s.io/apimachinery/pkg/runtime \
    --extra-peer-dir k8s.io/component-base/config/v1alpha1 \
    "${PROJECT_ROOT}/pkg/admissioncontroller/apis/config"
}
export -f admissioncontroller_groups

# Configuration for gardener scheduler

scheduler_groups() {
  echo "Generating API groups for pkg/scheduler/apis/config"
  
  kube::codegen::gen_helpers \
    --boilerplate "${PROJECT_ROOT}/hack/LICENSE_BOILERPLATE.txt" \
    --extra-peer-dir github.com/gardener/gardener/pkg/scheduler/apis/config/v1alpha1 \
    --extra-peer-dir k8s.io/apimachinery/pkg/apis/meta/v1 \
    --extra-peer-dir k8s.io/apimachinery/pkg/conversion \
    --extra-peer-dir k8s.io/apimachinery/pkg/runtime \
    --extra-peer-dir k8s.io/component-base/config/v1alpha1 \
    "${PROJECT_ROOT}/pkg/scheduler/apis/config"
}
export -f scheduler_groups

# Componentconfig for gardenlet

gardenlet_groups() {
  echo "Generating API groups for pkg/gardenlet/apis/config"
  
  kube::codegen::gen_helpers \
    --boilerplate "${PROJECT_ROOT}/hack/LICENSE_BOILERPLATE.txt" \
    --extra-peer-dir github.com/gardener/gardener/pkg/gardenlet/apis/config \
    --extra-peer-dir github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1 \
    --extra-peer-dir k8s.io/apimachinery/pkg/apis/meta/v1 \
    --extra-peer-dir k8s.io/apimachinery/pkg/conversion \
    --extra-peer-dir k8s.io/apimachinery/pkg/runtime \
    --extra-peer-dir k8s.io/component-base/config \
    --extra-peer-dir k8s.io/component-base/config/v1alpha1 \
    "${PROJECT_ROOT}/pkg/gardenlet/apis/config"
}
export -f gardenlet_groups

# Componentconfig for resource-manager

resourcemanager_groups() {
  echo "Generating API groups for pkg/resourcemanager/apis/config"
  
  kube::codegen::gen_helpers \
    --boilerplate "${PROJECT_ROOT}/hack/LICENSE_BOILERPLATE.txt" \
    --extra-peer-dir github.com/gardener/gardener/pkg/resourcemanager/apis/config/v1alpha1 \
    --extra-peer-dir k8s.io/apimachinery/pkg/apis/meta/v1 \
    --extra-peer-dir k8s.io/apimachinery/pkg/conversion \
    --extra-peer-dir k8s.io/apimachinery/pkg/runtime \
    --extra-peer-dir k8s.io/component-base/config/v1alpha1 \
    "${PROJECT_ROOT}/pkg/resourcemanager/apis/config"
}
export -f resourcemanager_groups

# Componentconfig for node-agent

nodeagent_groups() {
  echo "Generating API groups for pkg/nodeagent/apis/config"
  
  kube::codegen::gen_helpers \
    --boilerplate "${PROJECT_ROOT}/hack/LICENSE_BOILERPLATE.txt" \
    --extra-peer-dir github.com/gardener/gardener/pkg/nodeagent/apis/config/v1alpha1 \
    --extra-peer-dir k8s.io/apimachinery/pkg/apis/meta/v1 \
    --extra-peer-dir k8s.io/apimachinery/pkg/conversion \
    --extra-peer-dir k8s.io/apimachinery/pkg/runtime \
    --extra-peer-dir k8s.io/component-base/config/v1alpha1 \
    "${PROJECT_ROOT}/pkg/nodeagent/apis/config"
}
export -f nodeagent_groups

# Componentconfig for admission plugins

shoottolerationrestriction_groups() {
  echo "Generating API groups for plugin/pkg/shoot/tolerationrestriction/apis/shoottolerationrestriction"
  
  kube::codegen::gen_helpers \
    --boilerplate "${PROJECT_ROOT}/hack/LICENSE_BOILERPLATE.txt" \
    --extra-peer-dir github.com/gardener/gardener/plugin/pkg/shoot/tolerationrestriction/apis/shoottolerationrestriction \
    --extra-peer-dir github.com/gardener/gardener/plugin/pkg/shoot/tolerationrestriction/apis/shoottolerationrestriction/v1alpha1 \
    --extra-peer-dir k8s.io/apimachinery/pkg/apis/meta/v1,k8s.io/apimachinery/pkg/conversion \
    --extra-peer-dir k8s.io/apimachinery/pkg/runtime \
    --extra-peer-dir k8s.io/component-base/config \
    --extra-peer-dir k8s.io/component-base/config/v1alpha1 \
    "${PROJECT_ROOT}/plugin/pkg/shoot/tolerationrestriction/apis/shoottolerationrestriction"
}
export -f shoottolerationrestriction_groups

shootdnsrewriting_groups() {
  echo "Generating API groups for plugin/pkg/shoot/dnsrewriting/apis/shootdnsrewriting"
  
  kube::codegen::gen_helpers \
    --boilerplate "${PROJECT_ROOT}/hack/LICENSE_BOILERPLATE.txt" \
    --extra-peer-dir github.com/gardener/gardener/plugin/pkg/shoot/dnsrewriting/apis/shootdnsrewriting \
    --extra-peer-dir github.com/gardener/gardener/plugin/pkg/shoot/dnsrewriting/apis/shootdnsrewriting/v1alpha1 \
    --extra-peer-dir k8s.io/apimachinery/pkg/apis/meta/v1,k8s.io/apimachinery/pkg/conversion \
    --extra-peer-dir k8s.io/apimachinery/pkg/runtime \
    --extra-peer-dir k8s.io/component-base/config \
    --extra-peer-dir k8s.io/component-base/config/v1alpha1 \
    "${PROJECT_ROOT}/plugin/pkg/shoot/dnsrewriting/apis/shootdnsrewriting"
}
export -f shootdnsrewriting_groups

shootresourcereservation_groups() {
  echo "Generating API groups for plugin/pkg/shoot/resourcereservation/apis/shootresourcereservation"
  
  kube::codegen::gen_helpers \
    --boilerplate "${PROJECT_ROOT}/hack/LICENSE_BOILERPLATE.txt" \
    --extra-peer-dir github.com/gardener/gardener/plugin/pkg/shoot/resourcereservation/apis/shootresourcereservation \
    --extra-peer-dir github.com/gardener/gardener/plugin/pkg/shoot/resourcereservation/apis/shootresourcereservation/v1alpha1 \
    --extra-peer-dir k8s.io/apimachinery/pkg/apis/meta/v1,k8s.io/apimachinery/pkg/conversion \
    --extra-peer-dir k8s.io/apimachinery/pkg/runtime \
    --extra-peer-dir k8s.io/component-base/config \
    --extra-peer-dir k8s.io/component-base/config/v1alpha1 \
    "${PROJECT_ROOT}/plugin/pkg/shoot/resourcereservation/apis/shootresourcereservation"
}
export -f shootresourcereservation_groups

# local.provider.extensions.gardener.cloud APIs

provider_local_groups() {
  echo "Generating API groups for pkg/provider-local/apis/local"
  
  kube::codegen::gen_helpers \
    --boilerplate "${PROJECT_ROOT}/hack/LICENSE_BOILERPLATE.txt" \
    --extra-peer-dir github.com/gardener/gardener/pkg/provider-local/apis/local \
    --extra-peer-dir github.com/gardener/gardener/pkg/provider-local/apis/local/v1alpha1 \
    --extra-peer-dir k8s.io/apimachinery/pkg/apis/meta/v1 \
    --extra-peer-dir k8s.io/apimachinery/pkg/conversion \
    --extra-peer-dir k8s.io/apimachinery/pkg/runtime \
    "${PROJECT_ROOT}/pkg/provider-local/apis/local"
}
export -f provider_local_groups

# extensions/pkg/apis deepcopy methods

extensions_config_groups() {
  echo "Generating API groups for extensions/pkg/apis/config"
  
  kube::codegen::gen_helpers \
    --boilerplate "${PROJECT_ROOT}/hack/LICENSE_BOILERPLATE.txt" \
    "${PROJECT_ROOT}/extensions/pkg/apis/config"
}
export -f extensions_config_groups

# OpenAPI definitions

openapi_definitions() {
  echo "> Generating openapi definitions"
  rm -Rf ./${PROJECT_ROOT}/openapi/openapi_generated.go

  GO111MODULE=on go install k8s.io/kube-openapi/cmd/openapi-gen

  # Go installs in $GOBIN if defined, and $GOPATH/bin otherwise
  gobin="${GOBIN:-$(go env GOPATH)/bin}"

  "${gobin}/openapi-gen" \
    -v 1 \
    --output-file openapi_generated.go \
    --go-header-file "${PROJECT_ROOT}/hack/LICENSE_BOILERPLATE.txt" \
    --output-dir "${PROJECT_ROOT}/pkg/apiserver/openapi" \
    --output-pkg "github.com/gardener/gardener/pkg/apiserver/openapi" \
    --report-filename "${PROJECT_ROOT}/pkg/apiserver/openapi/api_violations.report" \
    "github.com/gardener/gardener/pkg/apis/authentication/v1alpha1" \
    "github.com/gardener/gardener/pkg/apis/core/v1" \
    "github.com/gardener/gardener/pkg/apis/core/v1beta1" \
    "github.com/gardener/gardener/pkg/apis/settings/v1alpha1" \
    "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1" \
    "github.com/gardener/gardener/pkg/apis/security/v1alpha1" \
    "github.com/gardener/gardener/pkg/apis/operations/v1alpha1" \
    "k8s.io/api/core/v1" \
    "k8s.io/api/rbac/v1" \
    "k8s.io/api/autoscaling/v1" \
    "k8s.io/api/networking/v1" \
    "k8s.io/apimachinery/pkg/apis/meta/v1" \
    "k8s.io/apimachinery/pkg/api/resource" \
    "k8s.io/apimachinery/pkg/types" \
    "k8s.io/apimachinery/pkg/version" \
    "k8s.io/apimachinery/pkg/runtime" \
    "k8s.io/apimachinery/pkg/util/intstr" \
    "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
}
export -f openapi_definitions

parse_flags "$@"

valid_options=()
invalid_options=()

if [[ -z "$CODEGEN_GROUPS" ]]; then
  valid_options=("${AVAILABLE_CODEGEN_OPTIONS[@]}")
else
  IFS=' ' read -ra OPTIONS_ARRAY <<< "$CODEGEN_GROUPS"
  for option in "${OPTIONS_ARRAY[@]}"; do
    valid=false
    for valid_option in "${AVAILABLE_CODEGEN_OPTIONS[@]}"; do
        if [[ "$option" == "$valid_option" ]]; then
            valid=true
            break
        fi
    done

    if $valid; then
        valid_options+=("$option")
    else
        invalid_options+=("$option")
    fi
  done

  if [[ ${#invalid_options[@]} -gt 0 ]]; then
    printf "ERROR: Invalid options: %s, Available options are: %s\n\n" "${invalid_options[*]}" "${AVAILABLE_CODEGEN_OPTIONS[*]}"
    exit 1
  fi
fi

printf "\n> Generating codegen for groups: %s\n" "${valid_options[*]}"
if [[ "$MODE" == "sequential" ]]; then
  for target in "${valid_options[@]}"; do
    "$target"
  done
elif [[ "$MODE" == "parallel" ]]; then
  parallel --will-cite ::: "${valid_options[@]}"
else
  printf "ERROR: Invalid mode ('%s'). Specify either 'parallel' or 'sequential'\n\n" "$MODE"
  exit 1
fi

openapi_definitions
