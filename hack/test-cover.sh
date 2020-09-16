#!/bin/bash
#
# SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0
set -e

source "$(dirname $0)/setup-envtest.sh"

echo "> Test Cover"

GO111MODULE=on ginkgo -cover -race -mod=vendor $@

COVERPROFILE="$(dirname $0)/../test.coverprofile"
COVERPROFILE_TMP="$(dirname $0)/../test.coverprofile.tmp"
COVERPROFILE_HTML="$(dirname $0)/../test.coverage.html"

echo "mode: set" > "$COVERPROFILE_TMP"
find . -name "*.coverprofile" -type f | xargs cat | grep -v mode: | sort -r | awk '{if($1 != last) {print $0;last=$1}}' >> "$COVERPROFILE_TMP"
cat "$COVERPROFILE_TMP" | grep -vE "\.pb\.go|zz_generated" > "$COVERPROFILE"
rm -rf "$COVERPROFILE_TMP"
go tool cover -html="$COVERPROFILE" -o="$COVERPROFILE_HTML"

go tool cover -func="$COVERPROFILE"
