# TODO(LucaBernstein): kindest/node images built with kind version `v0.26.0` and `v0.27.0` must not be used for the following reasons:
#  - `runc`: Starting with `v1.2.0`, we see tun/tap device issues with our vpn. We need to ensure at least `v1.2.4`: https://github.com/opencontainers/runc/pull/4555.
#  - `containerd`: Starting with `v2.0.1`, we see issues where nodes become unready due to a race condition with the go-sni. We need to ensure the next version after `v2.0.2`: https://github.com/containerd/containerd/issues/11275.
#      The root cause is fixed with go-cni >= v1.1.12 (https://github.com/containerd/containerd/pull/11244).
FROM kindest/node:v1.32.0@sha256:2458b423d635d7b01637cac2d6de7e1c1dca1148a2ba2e90975e214ca849e7cb

RUN apt-get update -yq && \
    apt-get install -yq --no-install-recommends wget apparmor apparmor-utils jq openssh-server sudo

# remove kind's kubelet unit
RUN rm -f /etc/systemd/system/kubelet.service && \
    rm -rf /etc/systemd/system/kubelet.service.d/

# copy containerd hosts configurations for local registry mirrors
COPY containerd /etc/containerd/certs.d/
# names with colon are not allowed in the directory name checked into Git as it breaks go mod if gardener/gardener is used as a module
RUN mv /etc/containerd/certs.d/garden.local.gardener.cloud_5001 /etc/containerd/certs.d/garden.local.gardener.cloud:5001

# add our userdata executor unit
COPY run-userdata.sh /run-userdata.sh
COPY run-userdata.service /etc/systemd/system
RUN systemctl enable run-userdata.service

RUN runc --version ; containerd --version

ENTRYPOINT ["/usr/local/bin/entrypoint", "/sbin/init"]
