#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

# reduce flakiness in contended pipelines
export GOMEGA_DEFAULT_EVENTUALLY_TIMEOUT=5s
export GOMEGA_DEFAULT_EVENTUALLY_POLLING_INTERVAL=200ms
# if we're running low on resources, it might take longer for tested code to do something "wrong"
# poll for 5s to make sure, we're not missing any wrong action
export GOMEGA_DEFAULT_CONSISTENTLY_DURATION=5s
export GOMEGA_DEFAULT_CONSISTENTLY_POLLING_INTERVAL=200ms

ginkgo_flags=

# If running in prow, we want to generate a machine-readable output file under the location specified via $ARTIFACTS.
# This will add a JUnit view above the build log that shows an overview over successful and failed test cases.
if [ -n "${CI:-}" -a -n "${ARTIFACTS:-}" ] ; then
  mkdir -p "$ARTIFACTS"
  ginkgo_flags="--output-dir=$ARTIFACTS --junit-report=junit.xml"
fi

cat <<EOF | tee -a /etc/hosts

# Manually created to access the test Gardener shoot clusters with name 'admin-kc-shoot' in the 'garden-local' namespace.
# TODO: Remove this again when the shoot cluster access is no longer required.
127.0.0.1 api.admin-kc-shoot.local.external.local.gardener.cloud
127.0.0.1 api.admin-kc-shoot.local.internal.local.gardener.cloud
EOF

GO111MODULE=on ginkgo --timeout=1h $ginkgo_flags --v --progress "$@" ./test/e2e/...
