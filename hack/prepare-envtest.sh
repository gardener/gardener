#!/usr/bin/env bash
#
# SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

set -o errexit
set -o nounset
set -o pipefail

export ENVTEST_K8S_VERSION=${ENVTEST_K8S_VERSION:-"1.30"}

echo "> Installing envtest tools@${ENVTEST_K8S_VERSION} with setup-envtest if necessary"
if ! command -v setup-envtest &> /dev/null ; then
  >&2 echo "setup-envtest not available"
  exit 1
fi

# --use-env allows overwriting the envtest tools path via the KUBEBUILDER_ASSETS env var
export KUBEBUILDER_ASSETS="$(setup-envtest use --use-env -p path ${ENVTEST_K8S_VERSION})"
echo "using envtest tools installed at '$KUBEBUILDER_ASSETS'"

source "$(dirname "$0")/test-integration.env"
