#!/usr/bin/env bash
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

docCommitHash="6a018dc6a0307e64192ab48dcb90034dd0c5102a"

echo "> Check Docforge Manifest"
repoPath=${1-"$(readlink -f "$(dirname "${0}")/..")"}
manifestPath=${2-"${repoPath}/.docforge/manifest.yaml"}
diffDirs=${3-".docforge/;docs/"}
repoName=${4-"gardener"}
useToken=${5-false}

tmpDir=$(mktemp -d)
function cleanup {
    rm -rf "$tmpDir"
}
trap cleanup EXIT ERR INT TERM

curl https://raw.githubusercontent.com/gardener/documentation/${docCommitHash}/.ci/check-manifest --output "${tmpDir}/check-manifest-script.sh" && chmod +x "${tmpDir}/check-manifest-script.sh"
curl https://raw.githubusercontent.com/gardener/documentation/${docCommitHash}/.ci/check-manifest-config --output "${tmpDir}/manifest-config"
scriptPath="${tmpDir}/check-manifest-script.sh"
configPath="${tmpDir}/manifest-config"

${scriptPath} --repo-path "${repoPath}" --repo-name "${repoName}" --use-token "${useToken}" --manifest-path "${manifestPath}" --diff-dirs "${diffDirs}" --config-path "${configPath}"
