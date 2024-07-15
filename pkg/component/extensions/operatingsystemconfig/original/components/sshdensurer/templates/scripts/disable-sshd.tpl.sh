#!/bin/bash -eu
set -e

# Disable and mask sshd service if not masked
if [ "$(systemctl is-enabled sshd.service)" != "masked" ] ; then
    systemctl disable sshd.service || true
    systemctl mask --now sshd.service
fi

# Stopping the sshd service does not terminate already established connections
# Kill all currently established orphaned ssh connections on the host
pkill -P 1 -x sshd || true
