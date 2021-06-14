#!/usr/bin/env bash
#
#  Copyright 2020 The Kubernetes Authors.
#
#  Licensed under the Apache License, Version 2.0 (the "License");
#  you may not use this file except in compliance with the License.
#  You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
#  Unless required by applicable law or agreed to in writing, software
#  distributed under the License is distributed on an "AS IS" BASIS,
#  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
#  See the License for the specific language governing permissions and
#  limitations under the License.

set -o errexit
set -o pipefail

ENVTEST_K8S_VERSION=${ENVTEST_K8S_VERSION:-"1.20"}

echo "> Installing envtest tools@${ENVTEST_K8S_VERSION} with setup-envtest if necessary"
if ! command -v setup-envtest &> /dev/null ; then
  # Some repos that vendor g/g and reuse this hack script might not need the envtest tools.
  # Thus, just skip installing anything if setup-envtest is not installed.
  # If envtest tools are needed, users will notice when their tests fail.
  echo "setup-envtest not available, skip installing envtest tools"
else
  # --use-env allows overwriting the envtest tools path via the KUBEBUILDER_ASSETS env var just like it was before
  setup-envtest use --use-env -p env ${ENVTEST_K8S_VERSION}
  source <(setup-envtest use --use-env -p env ${ENVTEST_K8S_VERSION})
  echo "using envtest tools installed at '$KUBEBUILDER_ASSETS'"
fi
