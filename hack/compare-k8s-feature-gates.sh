#!/usr/bin/env bash
#
# SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

set -e

usage() {
  echo "Usage:"
  echo "> compare-k8s-feature-gates.sh [ -h | <old version> <new version> ]"
  echo
  echo ">> For example: compare-k8s-feature-gates.sh 1.26 1.27"

  exit 0
}

if [ "$1" == "-h" ] || [ "$#" -ne 2 ]; then
  usage
fi

versions=("$1" "$2")
out_dir=$(mktemp -d)
function cleanup_output {
    rm -rf "$out_dir"
}
trap cleanup_output EXIT

for version in "${versions[@]}"; do
  versions_dir=test/compatibility_lifecycle/reference
  
  # TODO Drop this when Kubernetes v1.34 gets released
  if [ "$version" == "1.32" ]; then
    versions_dir="test/featuregates_linter/test_data"
  fi

 # Get the list of versioned and unversioned feature gates. After v1.34+ unversioned_feature_list.yaml should most likely be dropped
  wget -q -O - "https://raw.githubusercontent.com/kubernetes/kubernetes/release-${version}/${versions_dir}/versioned_feature_list.yaml" > "${out_dir}/versioned_featuregates_${version}.yaml"
  wget -q -O - "https://raw.githubusercontent.com/kubernetes/kubernetes/release-${version}/${versions_dir}/unversioned_feature_list.yaml" > "${out_dir}/unversioned_featuregates_${version}.yaml"
  # Extract only the names of the feature gates
  yq '.[] | .name' "${out_dir}/versioned_featuregates_${version}.yaml" > "${out_dir}/featuregates_list_${version}.yaml"
  yq '.[] | .name' "${out_dir}/unversioned_featuregates_${version}.yaml" >> "${out_dir}/featuregates_list_${version}.yaml"
  # Sort feature gate list for the diff to function correctly
  sort -o "${out_dir}/featuregates_list_${version}.yaml" "${out_dir}/featuregates_list_${version}.yaml"
done

echo "Feature gates added in $2 compared to $1:"
diff "${out_dir}/featuregates_list_${1}.yaml" "${out_dir}/featuregates_list_${2}.yaml" | grep '>' | awk '{print $2}'
echo
echo "Feature gates removed in $2 compared to $1:"
diff "${out_dir}/featuregates_list_${1}.yaml" "${out_dir}/featuregates_list_${2}.yaml" | grep '<' | awk '{print $2}'
echo
echo "Feature gates locked to default true:"
# Get all feature gate names that have a version spec containing $2, are locked to default with default value of true
yq '.[] | select(.versionedSpecs[] | select(.version == "'$2'" and .lockToDefault == true and .default == true)) | .name' "${out_dir}/versioned_featuregates_${version}.yaml"
yq '.[] | select(.versionedSpecs[] | select(.version == "'$2'" and .lockToDefault == true and .default == true)) | .name' "${out_dir}/unversioned_featuregates_${version}.yaml"
echo
echo "Feature gates locked to default false:"
# Get all feature gate names that have a version spec containing $2, are locked to default with default value of false
yq '.[] | select(.versionedSpecs[] | select(.version == "'$2'" and .lockToDefault == true and .default == false)) | .name' "${out_dir}/versioned_featuregates_${version}.yaml"
yq '.[] | select(.versionedSpecs[] | select(.version == "'$2'" and .lockToDefault == true and .default == false)) | .name' "${out_dir}/unversioned_featuregates_${version}.yaml"
