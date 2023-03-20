#!/usr/bin/env bash

set -e

echo "> Checking if license header is present in all requried files."

missing_license_header_files="$(addlicense -check -ignore "vendor/**" -ignore "**/*.md" -ignore "**/*.yaml" -ignore "**/Dockerfile" --ignore "hack/tools/gomegacheck/**" .)" || true

if [[ "$missing_license_header_files" ]]; then
  echo "Files with no license header detected:"
  echo "$missing_license_header_files"
  exit 1
fi

echo "All files have license headers."
