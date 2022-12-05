#!/usr/bin/env bash
#
# Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
  echo "> create-client-keys.sh [ -h | <host> <name> ]"
  echo
  echo ">> For example: create-client-keys.sh localhost provider-extensions"

  exit 0
}

if [ "$1" == "-h" ] || [ "$#" -ne 2 ]; then
  usage
fi

host=$1
name=$2

ssh-keygen -q -N "" -C "root@$host" -f "$SCRIPT_DIR"/ssh/client-keys/"$name"_id_rsa <<< y >/dev/null

rm -rf "$SCRIPT_DIR"/sshd/host-keys/authorized_keys

for f in "$SCRIPT_DIR"/ssh/client-keys/*_id_rsa.pub
do
    [ -e "$f" ] || continue
    cat "$f" >> "$SCRIPT_DIR"/sshd/host-keys/authorized_keys
done

