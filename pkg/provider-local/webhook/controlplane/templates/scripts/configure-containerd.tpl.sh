#!/bin/bash
# Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.


cluster_name={{ .kindClusterName }}

FILENAME=/etc/containerd/config.toml
if ! grep -q plugins.\"io.containerd.grpc.v1.cri\".registry.mirrors.\"localhost:5001\" "$FILENAME"; then
  cat <<EOF >> $FILENAME
[plugins."io.containerd.grpc.v1.cri".registry.mirrors."localhost:5001"]
  endpoint = ["http://$cluster_name:5001"]
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
