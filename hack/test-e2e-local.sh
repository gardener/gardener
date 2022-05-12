#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

echo "> E2E Tests"

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
  
  printf "\n127.0.0.1 api.e2e-default.local.external.local.gardener.cloud\n127.0.0.1 api.e2e-default.local.internal.local.gardener.cloud\n" >> /etc/hosts
else
  if ! grep -q "127.0.0.1 api.e2e-default.local.external.local.gardener.cloud" /etc/hosts ; then
    printf "To access the shoot cluster and running e2e tests, you have to extend your /etc/hosts file.\nPlease refer https://github.com/gardener/gardener/blob/master/docs/deployment/getting_started_locally.md#accessing-the-shoot-cluster"
  fi
fi

GO111MODULE=on ginkgo --timeout=1h $ginkgo_flags --v --progress "$@" ./test/e2e/...
