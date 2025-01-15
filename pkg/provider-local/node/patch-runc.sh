#!/usr/bin/env bash
# SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0


set -o errexit
set -o nounset
set -o pipefail

ARCH="$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/')"

echo "Installing runc v1.2.4 ..."
curl -sSL --retry 5 --output "/tmp/runc.${ARCH}" "https://github.com/opencontainers/runc/releases/download/v1.2.4/runc.${ARCH}"
mv "/tmp/runc.${ARCH}" /usr/local/sbin/runc
chmod 0755 /usr/local/sbin/runc
runc --version
