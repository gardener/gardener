#!/usr/bin/env bash
#
# SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

set -o nounset
set -o pipefail
set -o errexit

# Temp dirs
WORKDIR=$(mktemp -d)
ETCD_DRUID_REPO="https://github.com/gardener/etcd-druid"
ETCD_WRAPPER_REPO="https://github.com/gardener/etcd-wrapper"
ETCD_REPO="https://github.com/etcd-io/etcd.git"

echo "Cloning etcd-druid..."
git clone -q --depth 1 "$ETCD_DRUID_REPO" "$WORKDIR/etcd-druid"

# Extract etcd-wrapper tag from images.yaml
WRAPPER_TAG=$(yq '.images[] | select(.name == "etcd-wrapper") | .tag' "$WORKDIR/etcd-druid/internal/images/images.yaml")
echo "Found etcd-wrapper tag: $WRAPPER_TAG"

echo "Cloning etcd-wrapper at tag $WRAPPER_TAG..."
git clone -q --depth 1 --branch "$WRAPPER_TAG" "$ETCD_WRAPPER_REPO" "$WORKDIR/etcd-wrapper" 2>/dev/null

# Extract etcd version from go.mod
ETCD_LINE=$(grep 'go.etcd.io/etcd' "$WORKDIR/etcd-wrapper/go.mod" | head -n1)
ETCD_VERSION=$(echo "$ETCD_LINE" | awk '{print $2}')
echo "Found etcd version string: $ETCD_VERSION"

# Check if it's a real tag (vX.Y.Z) or a pseudo-version
# TODO: This if-else handling can be removed after https://github.com/gardener/etcd-druid/issues/445 is resolved (with
#  v3.5, the etcd-io/etcd repo supports proper usage of the direct tag, i.e., no commit hash is needed anymore).
if [[ "$ETCD_VERSION" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "Resolved etcd tag directly: $ETCD_VERSION"
else
  # Extract commit hash from pseudo-version
  COMMIT_HASH=$(echo "$ETCD_VERSION" | sed -E 's/.*-([a-f0-9]{12})$/\1/')

  if [ -z "$COMMIT_HASH" ]; then
    echo "Could not parse commit hash from version string: $ETCD_VERSION"
    exit 1
  fi

  echo "Extracted commit hash: $COMMIT_HASH"

  echo "Cloning etcd repo to resolve tag..."
  git clone -q "$ETCD_REPO" "$WORKDIR/etcd"
  pushd "$WORKDIR/etcd" > /dev/null

  # Find tag that contains the commit
  FULL_COMMIT=$(git rev-parse "$COMMIT_HASH") || {
    echo "Commit $COMMIT_HASH not found in etcd repo"
    exit 1
  }

  ETCD_TAG=$(git tag --contains "$FULL_COMMIT" | grep '^v' | sort -V | head -n 1)
  popd > /dev/null
fi

if [ -n "$ETCD_TAG" ]; then
  echo "Resolved etcd tag: $ETCD_TAG"
else
  echo "No tag found containing commit $FULL_COMMIT"
  exit 1
fi

# Update images.yaml with the resolved etcd tag
echo "Updating $1 with etcd $ETCD_TAG tag"
yq -i "(.images[] | select(.name == \"etcd\") ).tag = \"$ETCD_TAG\"" "$1"
