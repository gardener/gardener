#!/usr/bin/env bash
#
# Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#      http://www.apache.org/licenses~LICENSE-2.0
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
  echo "> create-cloud-profiles.sh [ -h | <garden-kubeconfig> ]"
  echo
  echo ">> For example: create-cloud-profiles.sh ~/.kube/garden-kubeconfig.yaml"

  exit 0
}

if [ "$1" == "-h" ] || [ "$#" -ne 1 ]; then
  usage
fi

garden_kubeconfig=$1

echo "Creating controller-registrations"
kubectl --kubeconfig $garden_kubeconfig --server-side=true apply -f https://raw.githubusercontent.com/gardener/gardener-extension-provider-gcp/v1.24.0/example/controller-registration.yaml
kubectl --kubeconfig $garden_kubeconfig --server-side=true apply -f https://raw.githubusercontent.com/gardener/gardener-extension-provider-aws/v1.37.0/example/controller-registration.yaml
kubectl --kubeconfig $garden_kubeconfig --server-side=true apply -f https://raw.githubusercontent.com/gardener/gardener-extension-provider-azure/v1.29.0/example/controller-registration.yaml
kubectl --kubeconfig $garden_kubeconfig --server-side=true apply -f https://raw.githubusercontent.com/gardener/gardener-extension-os-gardenlinux/v0.14.0/example/controller-registration.yaml
kubectl --kubeconfig $garden_kubeconfig --server-side=true apply -f https://raw.githubusercontent.com/gardener/gardener-extension-networking-calico/v1.26.0/example/controller-registration.yaml

echo "Creating cloud-profiles"
kubectl --kubeconfig $garden_kubeconfig --server-side=true apply -f $SCRIPT_DIR/profiles
