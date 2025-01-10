FROM kindest/node:v1.32.0@sha256:c48c62eac5da28cdadcf560d1d8616cfa6783b58f0d94cf63ad1bf49600cb027 AS base

ARG TARGETOS
ARG TARGETARCH

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
RUN echo "Installing runc v1.2.4 ..." \
    && curl -sSL --retry 5 --output /tmp/runc.${TARGETARCH} "https://github.com/opencontainers/runc/releases/download/v1.2.4/runc.${TARGETARCH}" \
    && mv /tmp/runc.${TARGETARCH} /usr/local/sbin/runc \
    && chmod 0755 /usr/local/sbin/runc \
    && runc --version

ENTRYPOINT ["/usr/local/bin/entrypoint", "/sbin/init"]
