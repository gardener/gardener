#!/usr/bin/env bash
#
# SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

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
