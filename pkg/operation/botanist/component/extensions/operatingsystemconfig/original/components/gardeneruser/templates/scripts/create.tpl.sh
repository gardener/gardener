#!/bin/bash -eu

DIR_SSH="/home/gardener/.ssh"
PATH_AUTHORIZED_KEYS="$DIR_SSH/authorized_keys"
PATH_SUDOERS="/etc/sudoers.d/99-gardener-user"
USERNAME="gardener"

id $USERNAME || useradd $USERNAME -mU
if [ ! -f "$PATH_AUTHORIZED_KEYS" ]; then
  mkdir -p $DIR_SSH
  cp -f {{ .pathPublicSSHKey }} $PATH_AUTHORIZED_KEYS
  chown $USERNAME:$USERNAME $PATH_AUTHORIZED_KEYS
fi
if [ ! -f "$PATH_SUDOERS" ]; then
  echo "$USERNAME ALL=(ALL) NOPASSWD:ALL" > $PATH_SUDOERS
fi
