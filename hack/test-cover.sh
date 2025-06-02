#!/usr/bin/env bash
#
# SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

set -o errexit
set -o nounset
set -o pipefail

echo "> Test Cover"

REPO_ROOT="$(git rev-parse --show-toplevel)"
COVERPROFILE="$REPO_ROOT/test.coverprofile"
COVERPROFILE_TMP="$REPO_ROOT/test.coverprofile.tmp"
COVERPROFILE_HTML="$REPO_ROOT/test.coverage.html"

trap "rm -rf \"$COVERPROFILE_TMP\"" EXIT ERR INT TERM

GO111MODULE=on go test -cover -coverprofile "$COVERPROFILE_TMP" -race -timeout=2m $@ | grep -v 'no test files'

cat "$COVERPROFILE_TMP" | grep -vE "\.pb\.go|zz_generated" > "$COVERPROFILE"
go tool cover -html="$COVERPROFILE" -o="$COVERPROFILE_HTML"

go tool cover -func="$COVERPROFILE"
