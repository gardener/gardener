#!/bin/bash -eu

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
