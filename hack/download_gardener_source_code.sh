#!/usr/bin/env bash
# SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

set -o errexit
set -o nounset
set -o pipefail

GARDENER_VERSION="$(curl -s https://api.github.com/repos/gardener/gardener/releases/latest | grep tag_name | cut -d '"' -f 4)"
GARDENER_RELEASE_DOWNLOAD_PATH="$(dirname $0)/dev/gardener-releases"

parse_flags() {
  while test $# -gt 0; do
    case "$1" in
    --gardener-version)
      shift
      GARDENER_VERSION="$1"
      ;;
    --download-path)
      shift
      GARDENER_RELEASE_DOWNLOAD_PATH="$1"
      ;;
    esac
    shift
  done
}

parse_flags "$@"

curl -sL "https://codeload.github.com/gardener/gardener/legacy.tar.gz/${GARDENER_VERSION}" -o /tmp/source_code.tar.gz && 
  mkdir -p "${GARDENER_RELEASE_DOWNLOAD_PATH}/${GARDENER_VERSION}" &&
  tar -C "${GARDENER_RELEASE_DOWNLOAD_PATH}/${GARDENER_VERSION}" -xzf /tmp/source_code.tar.gz --strip-components=1 && rm /tmp/source_code.tar.gz
