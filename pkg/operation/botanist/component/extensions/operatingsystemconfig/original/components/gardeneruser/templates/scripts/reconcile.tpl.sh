#!/bin/bash -eu
# Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.


DIR_SSH="/home/gardener/.ssh"
PATH_AUTHORIZED_KEYS="$DIR_SSH/authorized_keys"
PATH_SUDOERS="/etc/sudoers.d/99-gardener-user"
USERNAME="gardener"

# create user if missing
id $USERNAME || useradd $USERNAME -mU

# copy authorized_keys file
mkdir -p $DIR_SSH
cp -f "{{ .pathAuthorizedSSHKeys }}" $PATH_AUTHORIZED_KEYS
chown $USERNAME:$USERNAME $PATH_AUTHORIZED_KEYS

# remove unused legacy file
if [ -f "{{ .pathPublicSSHKey }}" ]; then
  rm -f "{{ .pathPublicSSHKey }}"
fi

# allow sudo for gardener user
if [ ! -f "$PATH_SUDOERS" ]; then
  echo "$USERNAME ALL=(ALL) NOPASSWD:ALL" > $PATH_SUDOERS
fi
