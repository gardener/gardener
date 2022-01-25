#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

ginkgo_flags=

# If running in prow, we want to generate a machine-readable output file under the location specified via $ARTIFACTS.
# This will add a JUnit view above the build log that shows an overview over successful and failed test cases.
if [ -n "${CI:-}" -a -n "${ARTIFACTS:-}" ] ; then
  mkdir -p "$ARTIFACTS"
  ginkgo_flags="--output-dir=$ARTIFACTS --junit-report=junit.xml"
fi

ginkgo --timeout=1h $ginkgo_flags --v --progress "$@" ./test/e2e/...
