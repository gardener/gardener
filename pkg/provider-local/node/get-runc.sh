#!/usr/bin/env bash

# SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

set -o errexit
set -o nounset
set -o pipefail

RUNC_VERSION="v1.2.4"
ARCH="$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/')"
SCRIPT_DIR="$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"

echo "Getting runc ${RUNC_VERSION} for ${ARCH}..."
curl -sSL --retry 5 --output "/tmp/runc.${ARCH}" "https://github.com/opencontainers/runc/releases/download/${RUNC_VERSION}/runc.${ARCH}"
mv "/tmp/runc.${ARCH}" "${SCRIPT_DIR}/runc"
chmod 0755 "${SCRIPT_DIR}/runc"
