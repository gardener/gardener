# To fix an issue with containerd v2.0.2 (see also: https://github.com/containerd/containerd/issues/11275),
# we need to build a custom image of containerd v2 with go-cni >= v1.1.12 (https://github.com/containerd/containerd/pull/11244).
# TODO(LucaBernstein): Remove once containerd v2.0.3 is released and included in the kindest/node image.
# See also https://github.com/kubernetes-sigs/kind/blob/440ae61ace7e92ddf12ff6e5b138027040fc987f/images/base/Dockerfile#L122.
FROM golang:1.24.0-bookworm AS build-containerd
ARG CONTAINERD_CLONE_URL="https://github.com/containerd/containerd"
ARG CONTAINERD_VERSION="release/2.0"
# we don't build with optional snapshotters, we never select any of these
# they're not ideal inside kind anyhow, and we save some disk space
ARG BUILDTAGS="no_aufs no_zfs no_btrfs no_devmapper"
RUN git clone --filter=tree:0 "${CONTAINERD_CLONE_URL}" /containerd \
    && cd /containerd \
    && git checkout "${CONTAINERD_VERSION}" \
    && go version \
    && cat go.mod | grep 'github.com/containerd/go-cni' \
    && export CGO_ENABLED=1 \
    && make bin/ctr bin/containerd bin/containerd-shim-runc-v2

FROM kindest/node:v1.32.2@sha256:f226345927d7e348497136874b6d207e0b32cc52154ad8323129352923a3142f AS base

RUN apt-get update -yq && \
    apt-get install -yq --no-install-recommends wget apparmor apparmor-utils jq openssh-server sudo

# remove kind's kubelet unit
RUN rm -f /etc/systemd/system/kubelet.service && \
    rm -rf /etc/systemd/system/kubelet.service.d/

# copy containerd hosts configurations for local registry mirrors
COPY hosts.toml /etc/containerd/certs.d/garden.local.gardener.cloud:5001/hosts.toml

# add our userdata executor unit
COPY run-userdata.sh /run-userdata.sh
COPY run-userdata.service /etc/systemd/system
RUN systemctl enable run-userdata.service

# TODO(marc1404): Remove once kindest/node uses runc >= v1.2.4
# workaround issue with runc v1.2.3 provided by kindest/node:v1.32.0 by installing runc v1.2.4 manually (https://github.com/opencontainers/runc/pull/4555)
COPY runc /usr/local/sbin/runc

# TODO(LucaBernstein): Remove once containerd v2.0.3 is released and included in the kindest/node image.
COPY --from=build-containerd /containerd/bin/containerd /usr/local/bin/
COPY --from=build-containerd /containerd/bin/ctr /usr/local/bin/
COPY --from=build-containerd /containerd/bin/containerd-shim-runc-v2 /usr/local/bin/

RUN runc --version ; containerd --version

ENTRYPOINT ["/usr/local/bin/entrypoint", "/sbin/init"]
