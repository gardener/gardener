#!/bin/bash -eu
set -e

# Unmask and enable sshd service if not enabled
if ! systemctl is-enabled --quiet sshd.service ; then
    systemctl unmask sshd.service
    # When sshd.service is disabled on gardenlinux the service is deleted
    # On gardenlinux sshd.service is enabled by enabling ssh.service
    if ! systemctl enable sshd.service ; then
        systemctl enable ssh.service
    fi
fi

# Start sshd service if not active
if ! systemctl is-active --quiet sshd.service ; then
    systemctl start sshd.service
fi
