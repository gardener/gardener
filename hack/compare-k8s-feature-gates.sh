#!/usr/bin/env bash
#
# SPDX-FileCopyrightText: Contributors to the Gardener project
#
# SPDX-License-Identifier: Apache-2.0

set -e

usage() {
  echo "Usage:"
  echo "> compare-k8s-feature-gates.sh [ -h | <old version> <new version> ]"
  echo
  echo ">> For example: compare-k8s-feature-gates.sh 1.34 1.35"
  echo
  echo ">> Note: The script only works for Kubernetes versions 1.33+"

  exit 0
}

if [ "$1" == "-h" ] || [ "$#" -ne 2 ]; then
  usage
fi

old_major=$(echo "$1" | cut -d '.' -f 1)
old_minor=$(echo "$1" | cut -d '.' -f 2)
new_major=$(echo "$2" | cut -d '.' -f 1)
new_minor=$(echo "$2" | cut -d '.' -f 2)

# Check if the new version is exactly one minor version higher than the old version
if [ "$old_major" -ne "$new_major" ] || [ "$((old_minor + 1))" -ne "$new_minor" ]; then
  echo "Error: The new version must be exactly one minor version higher than the old version."
  exit 1
fi

versions=("$1" "$2")
out_dir=$(mktemp -d)
function cleanup_output {
    rm -rf "$out_dir"
}
trap cleanup_output EXIT

for version in "${versions[@]}"; do
  if [ "$version" \< "1.33" ]; then
    echo "Versions less than 1.33 are not supported."
    exit 1
  fi
  wget -q -O - "https://raw.githubusercontent.com/kubernetes/kubernetes/release-${version}/test/compatibility_lifecycle/reference/versioned_feature_list.yaml" > "${out_dir}/versioned_featuregates_${version}.yaml"
  yq '.[] | .name' "${out_dir}/versioned_featuregates_${version}.yaml" > "${out_dir}/featuregates_list_${version}.yaml"
  # client-go feature gates are not part of versioned_feature_list.yaml, so fetch the Go source and extract their names too.
  wget -q -O - "https://raw.githubusercontent.com/kubernetes/kubernetes/release-${version}/staging/src/k8s.io/client-go/features/known_features.go" > "${out_dir}/known_features_${version}.go"
  grep -E '^[[:space:]]*[A-Za-z].*Feature = "' "${out_dir}/known_features_${version}.go" | sed -E 's/.*Feature = "([^"]+)".*/\1/' >> "${out_dir}/featuregates_list_${version}.yaml"
  # Sort feature gate list for the diff to function correctly
  sort -o "${out_dir}/featuregates_list_${version}.yaml" "${out_dir}/featuregates_list_${version}.yaml"
done

echo "Feature gates added in $2 compared to $1:"
diff "${out_dir}/featuregates_list_${1}.yaml" "${out_dir}/featuregates_list_${2}.yaml" | grep '>' | awk '{print $2}'
echo
echo "Feature gates removed in $2 compared to $1:"
diff "${out_dir}/featuregates_list_${1}.yaml" "${out_dir}/featuregates_list_${2}.yaml" | grep '<' | awk '{print $2}'
echo

echo "Feature gates locked to default true in $2 compared to $1:"
# Get all feature gate names that have a version spec containing $2, are locked to default with default value of true
yq '.[] | select(.versionedSpecs[] | select(.version == "'$2'" and .lockToDefault == true and .default == true)) | .name' "${out_dir}/versioned_featuregates_${version}.yaml"
# client-go feature gates are not in the yaml, so parse their lock state from the Go source.
awk -v ver="$2" '/^[[:space:]]+[A-Za-z][A-Za-z0-9_]*: \{$/ { gate=$1; sub(/:$/, "", gate) } $0 ~ ("MustParse\\(\"" ver "\"") && /LockToDefault: true/ && / Default: true/ { print gate }' "${out_dir}/known_features_${2}.go"
echo
echo "Feature gates locked to default false in $2 compared to $1:"

# Get all feature gate names that have a version spec containing $2, are locked to default with default value of false
yq '.[] | select(.versionedSpecs[] | select(.version == "'$2'" and .lockToDefault == true and .default == false)) | .name' "${out_dir}/versioned_featuregates_${version}.yaml"
# client-go feature gates are not in the yaml, so parse their lock state from the Go source.
awk -v ver="$2" '/^[[:space:]]+[A-Za-z][A-Za-z0-9_]*: \{$/ { gate=$1; sub(/:$/, "", gate) } $0 ~ ("MustParse\\(\"" ver "\"") && /LockToDefault: true/ && / Default: false/ { print gate }' "${out_dir}/known_features_${2}.go"
