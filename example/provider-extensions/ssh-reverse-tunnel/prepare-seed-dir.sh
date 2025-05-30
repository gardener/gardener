#!/usr/bin/env bash
#
# SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

set -e

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"

usage() {
  echo "Usage:"
  echo "> prepare-seed-dir.sh [ -h | <seed-name> ]"
  echo
  echo ">> For example: prepare-seed-dir.sh provider-extensions"

  exit 0
}

if [ "$1" == "-h" ] || [ "$#" -ne 1 ]; then
  usage
fi

name=$1

base_dir="$SCRIPT_DIR/seeds/$name"
echo "seed directory: $base_dir"
if [ ! -d "$base_dir" ]; then
  mkdir -p "$base_dir"
fi
cp -r "$SCRIPT_DIR"/seed-template/sshd "$base_dir"
cp -r "$SCRIPT_DIR"/seed-template/ssh "$base_dir"
mkdir -p "$base_dir/ssh/client-keys"
mkdir -p "$base_dir/sshd/host-keys"
sed -i -e "s/namespace: relay$/namespace: relay-$name/g" "$base_dir/ssh/kustomization.yaml"
sed -i -e "s/name: relay$/name: relay-$name/g" "$base_dir/ssh/namespace.yaml"
