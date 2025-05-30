#!/usr/bin/env bash
#
# SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

set -e

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"

usage() {
  echo "Usage:"
  echo "> create-host-keys.sh [ -h | <seed-name> <host> <port> ]"
  echo
  echo ">> For example: create-host-keys.sh localhost 22"

  exit 0
}

if [ "$1" == "-h" ] || [ "$#" -ne 3 ]; then
  usage
fi

seed=$1
host=$2
port=$3

base_dir="$SCRIPT_DIR/seeds/$seed"
if [ ! -d "$base_dir" ]; then
  echo "missing seed directory: $base_dir"
  exit 1
fi

ssh-keygen -q -C "" -N "" -t rsa -b 4096 -f "$base_dir"/sshd/host-keys/ssh_host_rsa_key <<< y >/dev/null
ssh-keygen -q -C "" -N "" -t ecdsa -f "$base_dir"/sshd/host-keys/ssh_host_ecdsa_key <<< y >/dev/null
ssh-keygen -q -C "" -N "" -t ed25519 -f "$base_dir"/sshd/host-keys/ssh_host_ed25519_key <<< y >/dev/null

rm -rf "$base_dir"/ssh/client-keys/known_hosts

{
    echo "[$host]:$port $(cat "$base_dir"/sshd/host-keys/ssh_host_rsa_key.pub)"
    echo "[$host]:$port $(cat "$base_dir"/sshd/host-keys/ssh_host_ecdsa_key.pub)"
    echo "[$host]:$port $(cat "$base_dir"/sshd/host-keys/ssh_host_ed25519_key.pub)"
} >> "$base_dir"/ssh/client-keys/known_hosts

echo "$host" > "$base_dir"/ssh/client-keys/host
