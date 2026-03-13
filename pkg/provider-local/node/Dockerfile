# TODO(rfranzke): Update to v1.35 once https://github.com/gardener/gardener/pull/13651 is resolved.
FROM kindest/node:v1.34.3@sha256:08497ee19eace7b4b5348db5c6a1591d7752b164530a36f855cb0f2bdcbadd48

RUN apt-get update -yq && \
    apt-get install -yq --no-install-recommends wget apparmor apparmor-utils jq openssh-server sudo logrotate

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
