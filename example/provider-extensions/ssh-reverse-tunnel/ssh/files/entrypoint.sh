#!/usr/bin/env sh

# Install openssh
apk add --no-cache openssh

host=$(cat /gardener-apiserver-ssh-keys/host)

# Connect to sshd for gardener-apiserver reverse tunnel
echo "Connecting to sshd for gardener-apiserver reverse tunnel @ $host"
exec ssh "root@$host" -R 6443:kubernetes.default.svc.cluster.local:443 -NT -p 6222 -F /gardener_apiserver_ssh/ssh_config
