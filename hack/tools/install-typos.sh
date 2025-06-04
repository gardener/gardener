#!/usr/bin/env bash
#
# SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

set -o errexit
set -o nounset
set -o pipefail

echo "> Installing typos"

TOOLS_BIN_DIR=${TOOLS_BIN_DIR:-$(dirname "$0")/bin}

os=$(uname -s | tr '[:upper:]' '[:lower:]')
arch=$(uname -m | sed 's/amd64/x86_64/;s/arm64/aarch64/')
typos_tar=typos-"$TYPOS_VERSION"-"$os"-"$arch".zip

if [[ $os == "darwin" ]]; then
  url="https://github.com/crate-ci/typos/releases/download/${TYPOS_VERSION}/typos-${TYPOS_VERSION}-${arch}-apple-darwin.tar.gz"
elif [[ $os == "linux" ]]; then
  url="https://github.com/crate-ci/typos/releases/download/${TYPOS_VERSION}/typos-${TYPOS_VERSION}-${arch}-unknown-linux-musl.tar.gz"
else
  if ! command -v typos &>/dev/null; then
    echo "Unable to automatically install typos(https://github.com/crate-ci/typos) for ${os}/${arch}. Please install it yourself and retry."
    exit 1
  fi
fi

out_dir=$(mktemp -d)
function cleanup_output {
    rm -rf "$out_dir"
}
trap cleanup_output EXIT

curl -L -o "$out_dir"/"$typos_tar" "$url"
tar -xf "$out_dir"/"$typos_tar" -C "$out_dir" >/dev/null
cp -f "$out_dir"/typos "$TOOLS_BIN_DIR"/typos
