FROM kindest/node:v1.36.1@sha256:3489c7674813ba5d8b1a9977baea8a6e553784dab7b84759d1014dbd78f7ebd5

RUN apt-get update -yq && \
    apt-get install -yq --no-install-recommends wget apparmor apparmor-utils jq openssh-server sudo logrotate

# remove kind's kubelet unit
RUN rm -f /etc/systemd/system/kubelet.service && \
    rm -rf /etc/systemd/system/kubelet.service.d/

# Add cgroup namespace to cri-base.json so that containers (including privileged pods running nested
# kubelets) get their own cgroup namespace. Without this, nested kubelets see the host's cgroup
# hierarchy and fail to manage cgroups properly. This is the same adjustment that kind.sh performs
# at cluster creation time for KinD nodes.
RUN jq '.linux.namespaces += [{"type": "cgroup"}]' /etc/containerd/cri-base.json > /tmp/cri-base.json && \
    mv /tmp/cri-base.json /etc/containerd/cri-base.json

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
