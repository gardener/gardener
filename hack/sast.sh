#!/usr/bin/env bash
#
# SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

set -e

report_dir="$(git rev-parse --show-toplevel)"

gosec_report="false"
gosec_report_parse_flags=""
exclude_dirs="hack"

parse_flags() {
  while test $# -gt 1; do
    case "$1" in
      --gosec-report)
        shift; gosec_report="$1"
        ;;
      --report-dir)
        shift; report_dir="$1"
        ;;
      --exclude-dirs)
        shift; exclude_dirs="$1"
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
  echo "Exporting report to ${report_dir}/gosec-report.sarif"
  gosec_report_parse_flags="-track-suppressions -fmt=sarif -out=${report_dir}/gosec-report.sarif -stdout"
fi

# Gardener uses code-generators https://github.com/kubernetes/code-generator and https://github.com/protocolbuffers/protobuf
# which create lots of G103 (CWE-242: Use of unsafe calls should be audited) & G104 (CWE-703: Errors unhandled) errors.
# However, those generators are best-practice in Kubernetes environment and their results are tested well.
# Thus, generated code is excluded from gosec scan.
# Nested go modules are not supported by gosec (see https://github.com/securego/gosec/issues/501), so the ./hack folder
# is excluded too. It does not contain productive code anyway.
gosec -exclude-generated $(echo "$exclude_dirs" | awk -v RS=',' '{printf "-exclude-dir %s ", $1}') $gosec_report_parse_flags ./...
