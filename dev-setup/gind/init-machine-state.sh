#!/usr/bin/env bash
# SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

# This script seeds persistent state directories on first boot and bind-mounts them into place.
# A single Docker volume is mounted at /mnt/data — not directly at the real paths (e.g. /etc/containerd) because
# that would shadow the base image's built-in files with an empty directory. Instead, the script copies the image's
# defaults into /mnt/data/<name> on first boot, then bind-mounts each subdirectory to the real target path.
# On subsequent starts the subdirectories are already populated and the copy is skipped, preserving runtime changes.
# This mirrors the init container in dev-setup/gardenadm/machines/machine.yaml.

set -o errexit
set -o nounset
set -o pipefail

# The kind Docker network is dual-stack. Disable IPv6 so that hostname resolution returns IPv4 — otherwise
# kube-apiserver picks the IPv6 address as its advertise address, which conflicts with the IPv4 service CIDR.
sysctl -w net.ipv6.conf.all.disable_ipv6=1 net.ipv6.conf.default.disable_ipv6=1

seed_if_empty() {
  local src="$1" dst="$2"
  if [ -z "$(ls -A "$dst" 2>/dev/null)" ]; then
    cp -a "$src"/. "$dst"/
  fi
}

seed_if_empty /etc/systemd/system /mnt/data/systemd-system

# Delegate cgroup v2 controllers before kubelet starts. In gind the Docker container IS the node — there is no
# outer kubelet that has already delegated controllers. We add an ExecStartPre drop-in that runs the kindest/node
# helper /kind/bin/create-kubelet-cgroup-v2.sh (writes all available controllers to
# /sys/fs/cgroup/cgroup.subtree_control and creates /kubelet, /kubelet.slice cgroups). This is the same mechanism
# that kind uses via its kubelet.service.d/11-kind.conf drop-in.
mkdir -p /mnt/data/systemd-system/kubelet.service.d
cat > /mnt/data/systemd-system/kubelet.service.d/11-cgroup-v2.conf <<'DROPIN'
[Service]
ExecStartPre=/bin/sh -euc "if [ -f /sys/fs/cgroup/cgroup.controllers ]; then /kind/bin/create-kubelet-cgroup-v2.sh; fi"
DROPIN
seed_if_empty /etc/containerd     /mnt/data/containerd
seed_if_empty /etc/cni/net.d      /mnt/data/cni-net-d 2>/dev/null || true

# Ensure containerd finds registry mirror configs in /etc/containerd/certs.d.
# The base image ships without config_path, so we inject it here.
if [ -f /mnt/data/containerd/config.toml ] && ! grep -q 'config_path' /mnt/data/containerd/config.toml; then
  sed -i '/^\[plugins\."io\.containerd\.grpc\.v1\.cri"\.containerd\]$/a\  config_path = "/etc/containerd/certs.d"' /mnt/data/containerd/config.toml
fi

# Ensure all mount-point directories exist (some are not present in the base image).
mkdir -p /mnt/data/kubernetes /mnt/data/static-pods /mnt/data/kubelet /mnt/data/gardenadm-state /mnt/data/gardener-node-agent /mnt/data/opt-bin /mnt/data/root-home
mkdir -p /etc/cni/net.d /etc/kubernetes /var/lib/kubelet /var/lib/static-pods /var/lib/gardenadm /var/lib/gardener-node-agent /opt/bin

# Bind-mount the persistent subdirectories to their real target paths.
mount --bind /mnt/data/systemd-system          /etc/systemd/system
mount --bind /mnt/data/containerd              /etc/containerd
mount --bind /mnt/data/cni-net-d               /etc/cni/net.d
mount --bind /mnt/data/kubernetes              /etc/kubernetes
mount --bind /mnt/data/static-pods             /var/lib/static-pods
mount --bind /mnt/data/kubelet                 /var/lib/kubelet
mount --bind /mnt/data/gardenadm-state         /var/lib/gardenadm
mount --bind /mnt/data/gardener-node-agent     /var/lib/gardener-node-agent
mount --bind /mnt/data/opt-bin                 /opt/bin
mount --bind /mnt/data/root-home               /root

# Write .bashrc into the root-home volume (the bind-mount of /root shadows any earlier file).
cat > /root/.bashrc <<'BASHRC'
export KUBECONFIG=/etc/kubernetes/admin.conf
export HISTFILE=/root/.bash_history
alias k=kubectl
alias kg='kubectl -n garden'
BASHRC

exec /usr/local/bin/entrypoint /sbin/init
