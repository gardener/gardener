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

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/')
DOWNLOAD_DIR="$(dirname $0)"/tools/bin
PROTOC_VERSION=23.3
PROTOC_ZIP=protoc-v$PROTOC_VERSION-$OS-$ARCH.zip

if [[ $OS == "darwin" ]]; then
  url="https://github.com/protocolbuffers/protobuf/releases/download/v${PROTOC_VERSION}/protoc-${PROTOC_VERSION}-osx-universal_binary.zip"
elif [[ $OS == "linux" && $ARCH == "amd64" ]]; then
  url="https://github.com/protocolbuffers/protobuf/releases/download/v${PROTOC_VERSION}/protoc-${PROTOC_VERSION}-linux-x86_64.zip"
elif [[ $OS == "linux" && $ARCH == "arm64" ]]; then
  url="https://github.com/protocolbuffers/protobuf/releases/download/v${PROTOC_VERSION}/protoc-${PROTOC_VERSION}-linux-aarch_64.zip"
else
  echo "${os}/${arch} is not supported."
fi

curl -L -o $DOWNLOAD_DIR/$PROTOC_ZIP "$url"
unzip -o $DOWNLOAD_DIR/$PROTOC_ZIP -d $DOWNLOAD_DIR >/dev/null
mv -f $DOWNLOAD_DIR/bin/protoc $DOWNLOAD_DIR/protoc
chmod -R +rX $DOWNLOAD_DIR/protoc
rm -fr $DOWNLOAD_DIR/bin $DOWNLOAD_DIR/$PROTOC_ZIP

echo "WARNING: Protobuf changes are not being validated"
