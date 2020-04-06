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

CURRENT_DIR="$(dirname $0)"
PROJECT_ROOT="${CURRENT_DIR}"/..

pushd "$PROJECT_ROOT" > /dev/null
APIROOTS=${APIROOTS:-$(git grep --files-with-matches -e '// +k8s:protobuf-gen=package' cmd pkg | \
	xargs -n 1 dirname | \
	sed 's,^,github.com/gardener/gardener/,;' | \
	sort | uniq
)}
popd > /dev/null

rm -f ${GOPATH}/bin/go-to-protobuf
rm -f ${GOPATH}/bin/protoc-gen-gogo

GOFLAGS="" go build -o ${GOPATH}/bin "$PROJECT_ROOT/vendor/k8s.io/code-generator/cmd/go-to-protobuf"
GOFLAGS="" go build -o ${GOPATH}/bin "$PROJECT_ROOT/vendor/k8s.io/code-generator/cmd/go-to-protobuf/protoc-gen-gogo"

if [[ -z "$(which protoc)" || "$(protoc --version)" != "libprotoc 3."* ]]; then
  if [[ "$(uname -s)" == *"Darwin"* ]]; then
    brew install protobuf
  else
    PROTOC_ZIP=protoc-3.7.1-linux-x86_64.zip
    curl -OL https://github.com/protocolbuffers/protobuf/releases/download/v3.7.1/$PROTOC_ZIP
    unzip -o $PROTOC_ZIP -d /usr/local bin/protoc
    unzip -o $PROTOC_ZIP -d /usr/local 'include/*'
    rm -f $PROTOC_ZIP
  fi

  echo "WARNING: Protobuf changes are not being validated"
fi

read -ra PACKAGES <<< $(echo ${APIROOTS})

# requires the 'proto' tag to build (will remove when ready)
# searches for the protoc-gen-gogo extension in the output directory
# satisfies import of github.com/gogo/protobuf/gogoproto/gogo.proto and the
# core Google protobuf types
go-to-protobuf \
  --packages="$(IFS=, ; echo "${PACKAGES[*]}")" \
  --apimachinery-packages='-k8s.io/apimachinery/pkg/util/intstr,-k8s.io/apimachinery/pkg/api/resource,-k8s.io/apimachinery/pkg/runtime/schema,-k8s.io/apimachinery/pkg/runtime,-k8s.io/apimachinery/pkg/apis/meta/v1,-k8s.io/apimachinery/pkg/apis/meta/v1beta1,-k8s.io/api/core/v1,-k8s.io/api/rbac/v1' \
  --go-header-file=${PROJECT_ROOT}/hack/LICENSE_BOILERPLATE.txt \
  --proto-import=${PROJECT_ROOT}/vendor
