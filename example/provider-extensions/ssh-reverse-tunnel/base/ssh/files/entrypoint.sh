#!/usr/bin/env sh
# SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0


# Install openssh
apk add --no-cache openssh

host=$(cat /gardener-apiserver-ssh-keys/host)

# Connect to sshd for gardener-apiserver reverse tunnel
echo "Connecting to sshd for gardener-apiserver reverse tunnel @ $host"
exec ssh "root@$host" -R 6443:kubernetes.default.svc.cluster.local:443 -NT -p 443 -F /gardener_apiserver_ssh/ssh_config
