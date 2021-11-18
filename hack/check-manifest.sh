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

# For the check-manifest step the following environment variables are used:
# GIT_OAUTH_TOKEN - used for fetch the content from Github
# CHECKED_DIRS - the directories which will be checked for diff against origin/master. If Diff exists then the checks will be triggered.
# otherwise the script will exist with status code 0
set -e

if [[ $(uname) == 'Darwin' ]]; then
  READLINK_BIN="greadlink"
else
  READLINK_BIN="readlink"
fi

repoPath="$(${READLINK_BIN} -f $(dirname ${0})/..)"
manifestPath="${repoPath}/.docforge/manifest.yaml"
configPath="${repoPath}/hack/check-manifest-config"

cd ${repoPath}
diffExist=false

for arg in "$@"; do
  diff=$(git diff origin/master --name-only | grep "^${arg}" || true)
  if [[ ! -z "${diff}" ]] ; then
    diffExist=true
    break
  fi
done

if [[ "${diffExist}" = false ]] ; then
    echo "There is no diff in the checked directories so the script exits successfully"
    exit 0
fi

getGitHubToken() {
  # Check if gardener-ci is available (in local setup)
  command -v gardener-ci >/dev/null && gardenci="true" || gardenci=""
  if [[ $gardenci == "true" ]]; then
    # Get a (round-robin) random technical GitHub user credentials
    technicalUser=$(gardener-ci config model_element --cfg-type github --cfg-name "${1}" --key credentials | sed -e "s/^GithubCredentials //" -e "s/'/\"/g")
    if [[ -n "${technicalUser}" ]]; then
      # get auth token and strip lead/trail quotes
      authToken=$(sed -e 's/"//g' <<<$(jq -n '$c.authToken' --argjson c "$technicalUser"))
      # get username and strip lead/trail quotes
      username=$(sed -e 's/"//g' <<<$(jq -n '$c.username' --argjson c "$technicalUser"))
      echo "${username}:${authToken}"
    fi
  fi
}

GIT_OAUTH_TOKEN=${GIT_OAUTH_TOKEN:-$(getGitHubToken github_com)}
test $GIT_OAUTH_TOKEN


# Set config file
tmpConfigPath="tmp-config"
sed -e "s@REPO_PATH@${repoPath}@g" ${configPath} > ${tmpConfigPath}
export DOCFORGECONFIG=${tmpConfigPath}

echo "Running docforge command..."
docforge \
  -f ${manifestPath} \
  -d tmp \
  --github-oauth-token $GIT_OAUTH_TOKEN \
  --hugo \
  --use-git=true \
  --dry-run

rm -rf ${tmpConfigPath}
