#!/usr/bin/env bash
#
# SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

set -e

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"

usage() {
  echo "Usage:"
  echo "> create-client-keys.sh [ -h | <seed-name> <host> ]"
  echo
  echo ">> For example: create-client-keys.sh localhost provider-extensions"

  exit 0
}

if [ "$1" == "-h" ] || [ "$#" -ne 2 ]; then
  usage
fi

name=$1
host=$2

base_dir="$SCRIPT_DIR/seeds/$name"
if [ ! -d "$base_dir" ]; then
  echo "missing seed directory: $base_dir"
  exit 1
fi

ssh-keygen -q -N "" -C "root@$host" -f "$base_dir"/ssh/client-keys/seed_id_rsa <<< y >/dev/null

rm -rf "$base_dir"/sshd/host-keys/authorized_keys

for f in "$base_dir"/ssh/client-keys/*_id_rsa.pub
do
    [ -e "$f" ] || continue
    cat "$f" >> "$base_dir"/sshd/host-keys/authorized_keys
done


