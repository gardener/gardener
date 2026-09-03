#!/usr/bin/env bash
#
# SPDX-FileCopyrightText: Contributors to the Gardener project
#
# SPDX-License-Identifier: Apache-2.0

set -e

GOLANGCI_LINT_CONFIG_FILE=""

for arg in "$@"; do
  case $arg in
    --golangci-lint-config=*)
    GOLANGCI_LINT_CONFIG_FILE="-c ${arg#*=}"
    shift
    ;;
  esac
done

echo "> Check"

echo "Executing golangci-lint"
golangci-lint run $GOLANGCI_LINT_CONFIG_FILE --timeout 10m $@

echo "Executing gofmt/goimports"
folders=()
for f in "$@"; do
  folders+=( "$(echo $f | sed 's/\.\.\.//')" )
done
unformatted_files="$(goimports -format-only -l ${folders[*]})"
if [[ "$unformatted_files" ]]; then
  echo "Unformatted files detected:"
  echo "$unformatted_files"
  exit 1
fi

echo "All checks successful"
