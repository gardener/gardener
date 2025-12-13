FROM kindest/node:v1.34.2@sha256:745f8ed46d8e99517774768227fd1a0af34a6bf395aef9c7ed98fbce0e263918

RUN apt-get update -yq && \
    apt-get install -yq --no-install-recommends wget apparmor apparmor-utils jq openssh-server sudo logrotate

# remove kind's kubelet unit
RUN rm -f /etc/systemd/system/kubelet.service && \
    rm -rf /etc/systemd/system/kubelet.service.d/

# copy containerd hosts configurations for local registry mirrors
COPY containerd /etc/containerd/certs.d/
# names with colon are not allowed in the directory name checked into Git as it breaks go mod if gardener/gardener is used as a module
RUN mv /etc/containerd/certs.d/registry.local.gardener.cloud_5000 /etc/containerd/certs.d/registry.local.gardener.cloud:5000

# add our userdata executor unit
COPY run-userdata.sh /run-userdata.sh
COPY run-userdata.service /etc/systemd/system
RUN systemctl enable run-userdata.service

RUN runc --version ; containerd --version

RUN chmod 755 /etc/sysctl.d/

ENTRYPOINT ["/usr/local/bin/entrypoint", "/sbin/init"]
