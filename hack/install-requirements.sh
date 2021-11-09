#!/bin/bash
#
# Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

# this script is kept for compatability reasons (other repos might use this script as well to install these tools)
# TODO: drop this script in a future release
echo "> [DEPRECATED] Installing requirements"

export GO111MODULE=on
curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin v1.41.1
curl -s "https://raw.githubusercontent.com/helm/helm/master/scripts/get-helm-3" | bash -s -- --version 'v3.5.4'

platform=$(uname -s)
if [[ ${platform} == "Linux" ]]; then
  if ! which jq &>/dev/null; then
    echo "Installing jq ..."
    curl -L -o /usr/local/bin/jq https://github.com/stedolan/jq/releases/download/jq-1.6/jq-linux64
    chmod +x /usr/local/bin/jq
  fi
fi

if [[ ${platform} == *"Darwin"* ]]; then
  cat <<EOM
You are running in a MAC OS environment!

Please make sure you have installed the following requirements:

- GNU Core Utils
- GNU Tar
- GNU Sed
- GNU Parallel

Brew command:
$ brew install coreutils gnu-sed gnu-tar grep jq parallel

Please allow them to be used without their "g" prefix:
$ export PATH=/usr/local/opt/coreutils/libexec/gnubin:\$PATH
$ export PATH=/usr/local/opt/gnu-sed/libexec/gnubin:\$PATH
$ export PATH=/usr/local/opt/gnu-tar/libexec/gnubin:\$PATH
$ export PATH=/usr/local/opt/grep/libexec/gnubin:\$PATH
EOM
fi
