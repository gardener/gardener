#!/usr/bin/env bash
#
# SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

set -o errexit
set -o nounset
set -o pipefail

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

# For go-to-protobuf, a path ending with `github.com/gardener/gardener` is expected, similar to the former GOPATH structure.
TMP_DIR=$(mktemp -d)
trap "rm -rf ${TMP_DIR}" EXIT
mkdir -p "${TMP_DIR}/github.com/gardener/"
ln -s "${PROJECT_ROOT}" "${TMP_DIR}/github.com/gardener/gardener"

# requires the 'proto' tag to build (will remove when ready)
# searches for the protoc-gen-gogo extension in the output directory
# satisfies import of github.com/gogo/protobuf/gogoproto/gogo.proto and the
# core Google protobuf types
go-to-protobuf \
  --go-header-file=${PROJECT_ROOT}/hack/LICENSE_BOILERPLATE.txt \
  --output-dir="${TMP_DIR}" \
  --proto-import="github.com/gogo/protobuf/gogoproto=$(go list -f '{{ .Dir }}' github.com/gogo/protobuf/gogoproto)" \
  --proto-import="k8s.io/api=$(go list -f '{{ .Dir }}' k8s.io/api)" \
  --proto-import="k8s.io/apimachinery=$(go list -f '{{ .Dir }}' k8s.io/apimachinery)" \
  --proto-import="k8s.io/apiextensions-apiserver=$(go list -f '{{ .Dir }}' k8s.io/apiextensions-apiserver)" \
  --packages="$(IFS=, ; echo "${PACKAGES[*]}")" \
  --apimachinery-packages=$(IFS=, ; echo "${APIMACHINERY_PKGS[*]}")
