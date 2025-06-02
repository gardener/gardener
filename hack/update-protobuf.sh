#!/usr/bin/env bash
#
# SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

set -o errexit
set -o nounset
set -o pipefail

# setup virtual GOPATH
source $(dirname $0)/vgopath-setup.sh

# We need to explicitly pass GO111MODULE=off to k8s.io/code-generator as it is significantly slower otherwise,
# see https://github.com/kubernetes/code-generator/issues/100.
export GO111MODULE=off

CURRENT_DIR="$(dirname $0)"
PROJECT_ROOT="${CURRENT_DIR}"/..
if [ "${PROJECT_ROOT#/}" == "${PROJECT_ROOT}" ]; then
  PROJECT_ROOT="./$PROJECT_ROOT"
fi

pushd "$PROJECT_ROOT" > /dev/null
APIROOTS=${APIROOTS:-$(git grep --untracked --files-with-matches -e '// +k8s:protobuf-gen=package' cmd pkg | \
	xargs -n 1 dirname | \
	sed 's,^,github.com/gardener/gardener/,;' | \
	sort | uniq
)}
popd > /dev/null

read -ra PACKAGES <<< $(echo ${APIROOTS})

APIMACHINERY_PKGS=(
  -k8s.io/apimachinery/pkg/util/intstr
  -k8s.io/apimachinery/pkg/api/resource
  -k8s.io/apimachinery/pkg/runtime/schema
  -k8s.io/apimachinery/pkg/runtime
  -k8s.io/apimachinery/pkg/apis/meta/v1
  -k8s.io/apimachinery/pkg/apis/meta/v1beta1
  -k8s.io/api/core/v1,-k8s.io/api/rbac/v1
  -k8s.io/api/autoscaling/v1
  -k8s.io/api/networking/v1
  -k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1
)

# requires the 'proto' tag to build (will remove when ready)
# searches for the protoc-gen-gogo extension in the output directory
# satisfies import of github.com/gogo/protobuf/gogoproto/gogo.proto and the
# core Google protobuf types
go-to-protobuf \
  --go-header-file=${PROJECT_ROOT}/hack/LICENSE_BOILERPLATE.txt \
  --output-dir="${GOPATH}/src" \
  --proto-import="${GOPATH}/src/k8s.io/kubernetes/staging/src" \
  --proto-import="${GOPATH}/src/k8s.io/kubernetes/vendor" \
  --packages="$(IFS=, ; echo "${PACKAGES[*]}")" \
  --apimachinery-packages=$(IFS=, ; echo "${APIMACHINERY_PKGS[*]}")
