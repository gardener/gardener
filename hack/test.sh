#!/usr/bin/env bash
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

set -o errexit
set -o nounset
set -o pipefail

echo "> Test"

test_flags=
# If running in prow, we want to generate a machine-readable output file under the location specified via $ARTIFACTS.
# This will add a JUnit view above the build log that shows an overview over successful and failed test cases.
if [ -n "${CI:-}" -a -n "${ARTIFACTS:-}" ] ; then
  if which report-collector &>/dev/null; then
    mkdir -p "$ARTIFACTS"
    trap "report-collector \"$ARTIFACTS/junit.xml\"" EXIT
    test_flags="--ginkgo.junit-report=junit.xml"
  else
    echo "report-collector not found in PATH, not generating machine-readable test report"
  fi
fi

GO111MODULE=on go test -race -timeout=2m -mod=vendor $@ $test_flags | grep -v 'no test files'
