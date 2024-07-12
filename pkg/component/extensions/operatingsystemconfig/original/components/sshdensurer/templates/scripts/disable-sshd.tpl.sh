#!/bin/bash -eu
set -e

# Disable sshd service if enabled
if systemctl is-enabled --quiet sshd.service ; then
    systemctl disable sshd.service
fi

# Stop sshd service if active
if systemctl is-active --quiet sshd.service ; then
    systemctl stop sshd.service
fi

# Stopping the sshd service does not terminate already established connections
# Kill all currently established orphaned ssh connections on the host
pkill -P 1 -x sshd || true
