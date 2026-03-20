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
ETCD_DRUID_VERSION=$(yq '.images[] | select(.name == "etcd-druid") | .tag' "$1")
ETCD_DRUID_IMAGES_URL="https://raw.githubusercontent.com/gardener/etcd-druid/${ETCD_DRUID_VERSION}/internal/images/images.yaml"
echo "Fetching etcd-druid images.yaml..."
curl -fsSL "$ETCD_DRUID_IMAGES_URL" -o "$WORKDIR/images.yaml"

# Extract etcd-wrapper tag from images.yaml
WRAPPER_TAG=$(yq '.images[] | select(.name == "etcd-wrapper-next") | .tag' "$WORKDIR/images.yaml")
echo "Found etcd-wrapper tag: $WRAPPER_TAG"

echo "Fetching go.mod from etcd-wrapper at tag $WRAPPER_TAG..."
ETCD_WRAPPER_GOMOD_URL="https://raw.githubusercontent.com/gardener/etcd-wrapper/$WRAPPER_TAG/go.mod"
curl -fsSL "$ETCD_WRAPPER_GOMOD_URL" -o "$WORKDIR/go.mod"

# Extract etcd version from go.mod
ETCD_LINE=$(grep 'go.etcd.io/etcd' "$WORKDIR/go.mod" | head -n1)
ETCD_VERSION=$(echo "$ETCD_LINE" | awk '{print $2}')
echo "Found etcd version string: $ETCD_VERSION"

# Update images.yaml with the resolved etcd tag
echo "Updating $1 with etcd $ETCD_VERSION tag"
yq -i "(.images[] | select(.name == \"etcd\") ).tag = \"$ETCD_VERSION\"" "$1"
