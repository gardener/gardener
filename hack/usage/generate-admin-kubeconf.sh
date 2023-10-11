#!/usr/bin/env bash
#
# Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

namespace="garden-local"
shoot_name="local"

if [[ -n $1 ]] ; then
    namespace=$1
fi

if [[ -n $2 ]] ; then
    shoot_name=$1
fi

kubectl create \
    -f "$(dirname "${0}")"/kubeconfig-request.json \
    --raw /apis/core.gardener.cloud/v1beta1/namespaces/"${namespace}"/shoots/"${shoot_name}"/adminkubeconfig | jq -r ".status.kubeconfig" | base64 -d
