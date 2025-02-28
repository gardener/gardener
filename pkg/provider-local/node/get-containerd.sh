#!/usr/bin/env bash

# SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

set -o errexit
set -o nounset
set -o pipefail

# TODO(LucaBernstein): Remove once containerd v2.0.3 is released and included in the kindest/node image.
# To fix an issue with containerd v2.0.2 (see also: https://github.com/containerd/containerd/issues/11275),
# we need to wait for an updated version of containerd v2 with go-cni >= v1.1.12 (https://github.com/containerd/containerd/pull/11244).
# See also https://github.com/kubernetes-sigs/kind/blob/440ae61ace7e92ddf12ff6e5b138027040fc987f/images/base/Dockerfile#L122.

CONTAINERD_VERSION="2.0.0"
ARCH="$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/')"
SCRIPT_DIR="$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"
DEV_DIR="${SCRIPT_DIR}/../../../dev"

echo "Getting containerd ${CONTAINERD_VERSION} for ${ARCH}..."
curl -sSL --retry 5 --output "${DEV_DIR}/containerd.${ARCH}.tar.gz" "https://github.com/containerd/containerd/releases/download/v${CONTAINERD_VERSION}/containerd-${CONTAINERD_VERSION}-linux-${ARCH}.tar.gz"
mkdir -p "${DEV_DIR}/containerd"
tar -xf "${DEV_DIR}/containerd.${ARCH}.tar.gz" -C "${DEV_DIR}/containerd"
rm -r "${SCRIPT_DIR}/containerd" 2>/dev/null || true
mv "${DEV_DIR}/containerd" "${SCRIPT_DIR}/"
chmod 0755 "${SCRIPT_DIR}/containerd/"*
