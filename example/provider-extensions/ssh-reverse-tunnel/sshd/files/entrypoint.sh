#!/usr/bin/env sh

# Install openssh
apk add --no-cache openssh

# Run sshd for gardener-apiserver reverse tunnel
echo "Starting sshd"
exec /usr/sbin/sshd -D -e -f /gardener_apiserver_sshd/sshd_config
