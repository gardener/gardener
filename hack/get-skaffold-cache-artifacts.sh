#!/usr/bin/env bash
#
# SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

# This script aims to solve the problem described in: https://github.com/gardener/gardener/issues/10209
# Both Skaffold configuration files, `skaffold.yaml` and `skaffold-operator.yaml`, generate dynamic patch files for Kustomize by invoking the following script through `build.hooks.after.command` lifecycle hooks:
# `hack/generate-kustomize-patch-gardenlet.sh`
# Since the patch files are not checked into the repository they are generated after Skaffold has built the artifacts.
# However, by default, Skaffold caches artifacts and does not rebuild them if the source files have not changed.
# Skaffold does not invoke build lifecycle hooks if the artifacts are cached.
# As part of the local deployment Kustomize is invoked to apply the patches generated by the script.
# If, for whatever reason, the patch files are not generated, Kustomize will fail to apply the patches and break the deployment.
# This script is a workaround that checks whether the patch files exist and that they contain the expected version pattern.
# It returns a literal boolean value to be passed to Skaffold's `--cache-artifacts` CLI flag.

set -e
set -o pipefail

if [ -n "$(git status --porcelain)" ]; then
  # The working directory is dirty.
  # Skaffold should not cache artifacts.
  echo "false"
  exit 0
fi

# Patch files to check for their existence.
# The files below are generated by both `skaffold.yaml` and `skaffold-operator.yaml`.
patch_files=(
  "example/gardener-local/gardenlet/operator/patch-helm-ref.yaml"
  "example/gardener-local/gardenlet/operator/patch-imagevector-overwrite.yaml"
)

# The patch files should contain the Gardener version and the Git commit SHA.
gardener_version=$(cat "VERSION")
git_abbrev_commit_sha=$(git rev-parse --short=10 HEAD)

# Iterate over all patch files and check for their existence.
for patch_file in "${patch_files[@]}"; do
  # Check if the patch file contains the expected version pattern: `<Gardener version>-<Git commit abbreviated SHA>` -> e.g. `v1.113.0-dev-215797c884`.
  if ! grep -q -E "(tag: |ref: .+)$gardener_version-$git_abbrev_commit_sha" "$patch_file"; then
    # The patch file does not exist or does not contain the expected version pattern.
    # Skaffold should not cache artifacts.
    echo "false"
    exit 0
  fi
done

# Skaffold should cache artifacts.
echo "true"
exit 0
