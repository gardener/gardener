#!/usr/bin/env bash
# SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

set -e

echo "> Checking if license header is present in all required files"

missing_license_header_files="$(addlicense \
  -check \
  -ignore ".git/**" \
  -ignore ".idea/**" \
  -ignore ".vscode/**" \
  -ignore "dev/**" \
  -ignore "**/*.md" \
  -ignore "**/*.html" \
  -ignore "**/*.yaml" \
  -ignore "**/Dockerfile" \
  -ignore "pkg/**/*.sh" \
  -ignore "third_party/gopkg.in/yaml.v2/**" \
  .)" || true

if [[ "$missing_license_header_files" ]]; then
  echo "Files with no license header detected:"
  echo "$missing_license_header_files"
  echo "Consider running \`make add-license-headers\` to automatically add all missing headers."
  exit 1
fi

echo "All files have license headers, nothing to be done."
