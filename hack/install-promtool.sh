#!/bin/bash
#
# Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

set -e

echo "> Installing promtool"

if which promtool &>/dev/null; then
  echo "promtool is already installed, skipping the installation..."
  exit 0
fi

platform=$(uname -s | tr '[:upper:]' '[:lower:]')
version="2.24.1"
archive_name="prometheus-${version}.${platform}-amd64"
file_name="${archive_name}.tar.gz"

temp_dir="$(mktemp -d)"
function cleanup {
  rm -rf "${temp_dir}"
}
trap cleanup EXIT ERR INT TERM

curl \
  -L \
  --output ${temp_dir}/${file_name} \
  "https://github.com/prometheus/prometheus/releases/download/v${version}/${file_name}"

tar -xzm -C "${temp_dir}" -f "${temp_dir}/${file_name}"
mv "${temp_dir}/${archive_name}/promtool" /usr/local/bin/
chmod +x /usr/local/bin/promtool
