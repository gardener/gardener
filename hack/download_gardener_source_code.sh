#!/usr/bin/env bash
# Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
