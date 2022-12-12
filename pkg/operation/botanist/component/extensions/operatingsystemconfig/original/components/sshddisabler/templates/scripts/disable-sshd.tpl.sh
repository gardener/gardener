#!/bin/bash -eu
set -e

# Stop sshd service if active
if systemctl is-active --quiet sshd.service ; then
    systemctl stop sshd.service
fi

# Disable sshd service if enabled
if systemctl is-enabled --quiet sshd.service ; then
    systemctl disable sshd.service
fi

# Disabling the sshd service does not terminate already established connections
# Kill all currently established ssh connections
pids=$(pidof sshd || true)
if [ -n "$pids" ]; then
    kill $pids
fi
