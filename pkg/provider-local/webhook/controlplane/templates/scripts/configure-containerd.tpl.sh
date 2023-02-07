#!/bin/bash

cluster_name={{ .kindClusterName }}

FILENAME=/etc/containerd/config.toml
if ! grep -q plugins.\"io.containerd.grpc.v1.cri\".registry.mirrors.\"localhost:5001\" "$FILENAME"; then
  cat <<EOF >> $FILENAME
[plugins."io.containerd.grpc.v1.cri".registry.mirrors."localhost:5001"]
  endpoint = ["http://$cluster_name:5001"]
[plugins."io.containerd.grpc.v1.cri".registry.mirrors."docker.io"]
  endpoint = ["http://$cluster_name:5002"]
[plugins."io.containerd.grpc.v1.cri".registry.mirrors."gcr.io"]
  endpoint = ["http://$cluster_name:5003"]
[plugins."io.containerd.grpc.v1.cri".registry.mirrors."eu.gcr.io"]
  endpoint = ["http://$cluster_name:5004"]
[plugins."io.containerd.grpc.v1.cri".registry.mirrors."ghcr.io"]
  endpoint = ["http://$cluster_name:5005"]
[plugins."io.containerd.grpc.v1.cri".registry.mirrors."registry.k8s.io"]
  endpoint = ["http://$cluster_name:5006"]
[plugins."io.containerd.grpc.v1.cri".registry.mirrors."quay.io"]
  endpoint = ["http://$cluster_name:5007"]
EOF
  echo "Configured containerd with registry mirrors for local-setup."
else
  echo "Containerd already configured with registry mirrors."
fi
