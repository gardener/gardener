#!/usr/bin/env bash
#
# Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

# Create symlinks to local mod chache for logr and controller-runtime log.

SCRIPT_DIR="$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"
LOGCHECK_DIR="$LOGCHECK_DIR"

cd "$SCRIPT_DIR"/..
echo $LOGCHECK_DIR
LOGR_DIR=$(go list -f '{{ .Dir }}' github.com/go-logr/logr)
CONTROLLER_RUNTIME_LOGR_DIR=$(go list -f '{{ .Dir }}' sigs.k8s.io/controller-runtime/pkg/log)

if [ ! -h  "./$LOGCHECK_DIR/pkg/logcheck/testdata/src/github.com/go-logr/logr" ]; then
  ln -s "$LOGR_DIR" "./$LOGCHECK_DIR/pkg/logcheck/testdata/src/github.com/go-logr/logr"
fi
if [ ! -h "./$LOGCHECK_DIR/pkg/logcheck/testdata/src/sigs.k8s.io/controller-runtime/pkg/log" ]; then
  ln -s "$CONTROLLER_RUNTIME_LOGR_DIR" "./$LOGCHECK_DIR/pkg/logcheck/testdata/src/sigs.k8s.io/controller-runtime/pkg/log"
fi
