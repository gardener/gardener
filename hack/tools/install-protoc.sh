#!/usr/bin/env bash
#
# SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

set -o errexit
set -o nounset
set -o pipefail

TOOLS_BIN_DIR=${TOOLS_BIN_DIR:-$(dirname "$0")/bin}

os=$(uname -s | tr '[:upper:]' '[:lower:]')
arch=$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/')
protoc_zip=protoc-"$PROTOC_VERSION"-"$os"-"$arch".zip

if [[ $os == "darwin" ]]; then
  url="https://github.com/protocolbuffers/protobuf/releases/download/${PROTOC_VERSION}/protoc-${PROTOC_VERSION#v}-osx-universal_binary.zip"
elif [[ $os == "linux" && $arch == "amd64" ]]; then
  url="https://github.com/protocolbuffers/protobuf/releases/download/${PROTOC_VERSION}/protoc-${PROTOC_VERSION#v}-linux-x86_64.zip"
elif [[ $os == "linux" && $arch == "arm64" ]]; then
  url="https://github.com/protocolbuffers/protobuf/releases/download/${PROTOC_VERSION}/protoc-${PROTOC_VERSION#v}-linux-aarch_64.zip"
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
