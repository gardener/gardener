FROM kindest/node:v1.35.1@sha256:05d7bcdefbda08b4e038f644c4df690cdac3fba8b06f8289f30e10026720a1ab

RUN apt-get update -yq && \
    apt-get install -yq --no-install-recommends wget apparmor apparmor-utils jq openssh-server sudo logrotate

# TODO(LucaBernstein): kindest/node:v1.35.1 ships containerd v2.2.1 which has a broken `ctr images mount` command
# (https://github.com/containerd/containerd/issues/12549, fixed in v2.2.2). Override with v2.2.2 until a new
# kindest/node image is available that includes the fix.
RUN arch="$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/')" && \
    wget -qO- "https://github.com/containerd/containerd/releases/download/v2.2.2/containerd-2.2.2-linux-${arch}.tar.gz" | \
    tar -xz -C /usr/local/bin --strip-components=1 bin/containerd bin/ctr

# remove kind's kubelet unit
RUN rm -f /etc/systemd/system/kubelet.service && \
    rm -rf /etc/systemd/system/kubelet.service.d/

# copy containerd hosts configurations for local registry mirrors
COPY containerd /etc/containerd/certs.d/
# names with colon are not allowed in the directory name checked into Git as it breaks go mod if gardener/gardener is used as a module
RUN mv /etc/containerd/certs.d/registry.local.gardener.cloud_5001 /etc/containerd/certs.d/registry.local.gardener.cloud:5001

# add our userdata executor unit
COPY run-userdata.sh /run-userdata.sh
COPY run-userdata.service /etc/systemd/system
RUN systemctl enable run-userdata.service

RUN runc --version ; containerd --version

RUN chmod 755 /etc/sysctl.d/

ENTRYPOINT ["/usr/local/bin/entrypoint", "/sbin/init"]
