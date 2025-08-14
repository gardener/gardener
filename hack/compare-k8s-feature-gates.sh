#!/usr/bin/env bash
#
# SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

set -e

usage() {
  echo "Usage:"
  echo "> compare-k8s-feature-gates.sh [ -h | <old version> <new version> ]"
  echo
  echo ">> For example: compare-k8s-feature-gates.sh 1.33 1.34"
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
echo
echo "Feature gates locked to default false in $2 compared to $1:"
# Get all feature gate names that have a version spec containing $2, are locked to default with default value of false
yq '.[] | select(.versionedSpecs[] | select(.version == "'$2'" and .lockToDefault == true and .default == false)) | .name' "${out_dir}/versioned_featuregates_${version}.yaml"
