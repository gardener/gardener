#!/usr/bin/env bash
# SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

# This script seeds persistent state directories on first boot and bind-mounts them into place.
# Named Docker volumes are mounted at /mnt/<name> to avoid shadowing the image's built-in files.
# On the very first start the volumes are empty, so we copy the image's defaults into them.
# On subsequent starts the volumes are already populated and the copy is skipped, preserving runtime changes.
# Finally, we bind-mount the volumes to their real target paths.
# This mirrors the init container in dev-setup/gardenadm/machines/machine.yaml.

set -o errexit
set -o nounset
set -o pipefail

seed_if_empty() {
  local src="$1" dst="$2"
  if [ -z "$(ls -A "$dst" 2>/dev/null)" ]; then
    cp -a "$src"/. "$dst"/
  fi
}

seed_if_empty /etc/systemd/system /mnt/systemd-system

# Ensure cgroup v2 controllers are delegated before kubelet starts.
# The kindest/node image ships /kind/bin/create-kubelet-cgroup-v2.sh which writes all available controllers
# to /sys/fs/cgroup/cgroup.subtree_control (and creates /kubelet, /kubelet.slice cgroups). Originally, this
# script was wired as ExecStartPre via a kubelet.service.d/11-kind.conf drop-in — but the gind Dockerfile
# removes the entire kubelet service (and its .d/ directory) because gardenadm installs its own.
#
# When running on top of KinD (the normal local setup), this isn't needed: the outer KinD node's kubelet has
# already delegated cgroup controllers, and the cgroupns gives the inner container a view with controllers
# pre-delegated. In gind, there is no outer kubelet — the Docker container IS the node, so we must ensure
# the delegation happens before gardenadm's kubelet starts.
mkdir -p /mnt/systemd-system/kubelet.service.d
cat > /mnt/systemd-system/kubelet.service.d/11-cgroup-v2.conf <<'DROPIN'
[Service]
ExecStartPre=/bin/sh -euc "if [ -f /sys/fs/cgroup/cgroup.controllers ]; then /kind/bin/create-kubelet-cgroup-v2.sh; fi"
DROPIN
seed_if_empty /etc/containerd     /mnt/containerd
seed_if_empty /etc/cni/net.d      /mnt/cni-net-d 2>/dev/null || true

# Ensure containerd finds registry mirror configs in /etc/containerd/certs.d.
# The base image ships without config_path, so we inject it here.
if [ -f /mnt/containerd/config.toml ] && ! grep -q 'config_path' /mnt/containerd/config.toml; then
  sed -i '/^\[plugins\."io\.containerd\.grpc\.v1\.cri"\.containerd\]$/a\  config_path = "/etc/containerd/certs.d"' /mnt/containerd/config.toml
fi

# Ensure all mount-point directories exist (some are not present in the base image).
mkdir -p /etc/cni/net.d /etc/kubernetes /var/lib/kubelet /var/lib/static-pods /var/lib/gardenadm /var/lib/gardener-node-agent /opt/bin

# Bind-mount the persistent volumes to their real target paths.
mount --bind /mnt/systemd-system          /etc/systemd/system
mount --bind /mnt/containerd              /etc/containerd
mount --bind /mnt/cni-net-d               /etc/cni/net.d
mount --bind /mnt/kubernetes              /etc/kubernetes
mount --bind /mnt/static-pods             /var/lib/static-pods
mount --bind /mnt/kubelet                 /var/lib/kubelet
mount --bind /mnt/gardenadm-state         /var/lib/gardenadm
mount --bind /mnt/gardener-node-agent     /var/lib/gardener-node-agent
mount --bind /mnt/opt-bin                 /opt/bin
mount --bind /mnt/root-home               /root

# Write .bashrc into the root-home volume (the bind-mount of /root shadows any earlier file).
cat > /root/.bashrc <<'BASHRC'
export KUBECONFIG=/etc/kubernetes/admin.conf
export HISTFILE=/root/.bash_history
alias k=kubectl
alias kg='kubectl -n garden'
BASHRC

exec /usr/local/bin/entrypoint /sbin/init
