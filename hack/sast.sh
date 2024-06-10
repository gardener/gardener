#!/usr/bin/env bash
#
# SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

set -e

root_dir="$( cd "$( dirname "${BASH_SOURCE[0]}" )/.." &> /dev/null && pwd )"

gosec_report="false"
gosec_report_parse_flags=""

parse_flags() {
  while test $# -gt 1; do
    case "$1" in
      --gosec-report)
        shift; gosec_report="$1"
        ;;
      *)
        echo "Unknown argument: $1"
        exit 1
        ;;
    esac
    shift
  done
}

parse_flags "$@"

echo "> Running gosec"
gosec --version
if [[ "$gosec_report" != "false" ]]; then
  echo "Exporting report to $root_dir/gosec-report.sarif"
  gosec_report_parse_flags="-track-suppressions -fmt=sarif -out=gosec-report.sarif -stdout"
fi

# Gardener uses code-generators https://github.com/kubernetes/code-generator and https://github.com/protocolbuffers/protobuf
# which create lots of G103 (CWE-242: Use of unsafe calls should be audited) & G104 (CWE-703: Errors unhandled) errors.
# However, those generators are best-pratice in Kubernetes environment and their results are tested well.
# Thus, generated code is excluded from gosec scan.
# Nested go modules are not supported by gosec (see https://github.com/securego/gosec/issues/501), so the ./hack folder
# is excluded too. It does not contain productive code anyway.
gosec -exclude-generated -exclude-dir=hack $gosec_report_parse_flags ./...
