#!/usr/bin/env bash
#
# Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -e

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"

usage() {
  echo "Usage:"
  echo "> deploy-quic-relay.sh [ -h | <garden-kubeconfig> <seed-kubeconfig> <quic host> ]"
  echo
  echo ">> For example: deploy-quic-relay.sh ~/.kube/garden-kubeconfig.yaml ~/.kube/kubeconfig.yaml quic.gardener.cloud"

  exit 0
}

if [ "$1" == "-h" ] || [ "$#" -ne 3 ]; then
  usage
fi

garden_kubeconfig=$1
seed_kubeconfig=$2
host=$3

echo "Deploying quic server to seed at $host"
sed "s/\$HOST/$host/g" $SCRIPT_DIR/quic-relay-server.yaml | kubectl --kubeconfig $seed_kubeconfig --server-side=true apply -f -

echo "Deploying quic client to garden"
sed "s/\$HOST/$host/g" $SCRIPT_DIR/quic-relay-client.yaml | kubectl --kubeconfig $garden_kubeconfig --server-side=true apply -f -
