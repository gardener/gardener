#!/usr/bin/env bash
#
# SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

set -o errexit
set -o nounset
set -o pipefail

source "$(dirname "$0")/prepare-envtest.sh"

echo "> Integration Tests"

test_flags=
# If running in Prow, we want to generate a machine-readable output file under the location specified via $ARTIFACTS.
# This will add a JUnit view above the build log that shows an overview over successful and failed test cases.
if [ -n "${CI:-}" -a -n "${ARTIFACTS:-}" ] ; then
  mkdir -p "$ARTIFACTS"
  trap "report-collector \"$ARTIFACTS/junit.xml\"" EXIT
  test_flags="--ginkgo.junit-report=junit.xml"
  # Use Ginkgo timeout in Prow to print everything that is buffered in GinkgoWriter.
  test_flags+=" --ginkgo.timeout=5m"
else
  # We don't want Ginkgo's timeout flag locally because it causes skipping the test cache.
  timeout_flag=-timeout=5m
fi

GO111MODULE=on go test ${timeout_flag:-} $@ $test_flags | grep -v 'no test files'
