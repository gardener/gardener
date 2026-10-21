#!/bin/bash -eu

DIR_SSH="/home/gardener/.ssh"
PATH_AUTHORIZED_KEYS="$DIR_SSH/authorized_keys"
PATH_SUDOERS="/etc/sudoers.d/99-gardener-user"
USERNAME="gardener"
HOME_DIR="/home/$USERNAME"

# create user if missing
if ! id "$USERNAME" >/dev/null 2>&1; then
  if [ -d "$HOME_DIR" ]; then
    # The home directory survived an /etc wipe (e.g. GardenLinux wipe-based in-place OS upgrade) while the
    # passwd/group entries did not. Recreate the user/group with the UID/GID that still own the home directory
    # so its files (and anything else owned by the gardener user) keep their owner instead of being orphaned.
    GARDENER_UID="$(stat -c %u "$HOME_DIR")"
    GARDENER_GID="$(stat -c %g "$HOME_DIR")"
    getent group "$GARDENER_GID" >/dev/null 2>&1 || groupadd -g "$GARDENER_GID" "$USERNAME"
    useradd "$USERNAME" -m -u "$GARDENER_UID" -g "$GARDENER_GID"
  else
    useradd "$USERNAME" -mU
  fi
fi

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
