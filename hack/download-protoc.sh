#!/usr/bin/env bash
#
# Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

TOOLS_BIN_DIR=${TOOLS_BIN_DIR:-$(dirname "$0")/tools/bin}

os=$(uname -s | tr '[:upper:]' '[:lower:]')
arch=$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/')
protoc_zip=protoc-v"$PROTOC_VERSION"-"$os"-"$arch".zip

if [[ $os == "darwin" ]]; then
  url="https://github.com/protocolbuffers/protobuf/releases/download/v${PROTOC_VERSION}/protoc-${PROTOC_VERSION}-osx-universal_binary.zip"
elif [[ $os == "linux" && $arch == "amd64" ]]; then
  url="https://github.com/protocolbuffers/protobuf/releases/download/v${PROTOC_VERSION}/protoc-${PROTOC_VERSION}-linux-x86_64.zip"
elif [[ $os == "linux" && $arch == "arm64" ]]; then
  url="https://github.com/protocolbuffers/protobuf/releases/download/v${PROTOC_VERSION}/protoc-${PROTOC_VERSION}-linux-aarch_64.zip"
else
  if ! command -v protoc &>/dev/null; then
    echo "Unable to automatically install protoc for ${os}/${arch}. Please install it yourself and retry."
    exit 1
  fi
fi

out_dir=$(mktemp -d)
function cleanup_output {
    rm -rf "$out_dir"
}
trap cleanup_output EXIT

curl -L -o "$out_dir"/"$protoc_zip" "$url"
unzip -o "$out_dir"/"$protoc_zip" -d "$out_dir" >/dev/null
rm -rf "$TOOLS_BIN_DIR"/include
cp -f "$out_dir"/bin/protoc "$TOOLS_BIN_DIR"/protoc
cp -r "$out_dir"/include "$TOOLS_BIN_DIR"
