#!/usr/bin/env bash
#
# SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

set -e

echo "> Installing promtool"

TOOLS_BIN_DIR=${TOOLS_BIN_DIR:-$(dirname $0)/bin}

platform=$(uname -s | tr '[:upper:]' '[:lower:]')
version=$PROMTOOL_VERSION
case $(uname -m) in
  aarch64 | arm64)
    arch="arm64"
    ;;
  x86_64)
    arch="amd64"
    ;;
  *)
    echo "Unknown architecture"
    exit -1
    ;;
esac

archive_name="prometheus-${version}.${platform}-${arch}"
file_name="${archive_name}.tar.gz"

temp_dir="$(mktemp -d)"
function cleanup {
  rm -rf "${temp_dir}"
}
trap cleanup EXIT ERR INT TERM

curl -L -o ${temp_dir}/${file_name} "https://github.com/prometheus/prometheus/releases/download/v${version}/${file_name}"

tar -xzm -C "${temp_dir}" -f "${temp_dir}/${file_name}"
mv "${temp_dir}/${archive_name}/promtool" $TOOLS_BIN_DIR
chmod +x $TOOLS_BIN_DIR/promtool
