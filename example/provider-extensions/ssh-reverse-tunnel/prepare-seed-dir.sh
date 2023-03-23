#!/usr/bin/env bash
#
# Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#      http://www.apache.org/licenses~LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

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
