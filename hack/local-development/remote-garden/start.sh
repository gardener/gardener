#!/usr/bin/env bash
#
# Copyright 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

set -o errexit
set -o nounset
set -o pipefail

REMOTE_GARDEN_LABEL=${1:-remote-garden}

echo "# Remove old containers and create the docker user network"
$(dirname $0)/cleanup $REMOTE_GARDEN_LABEL
docker network create gardener-dev-remote --label $REMOTE_GARDEN_LABEL

echo "# Start gardener etcd used to store gardener resources (e.g., seeds, shoots)"
$(dirname $0)/run-gardener-etcd $REMOTE_GARDEN_LABEL

echo "# Open tunnels for accessing local gardener components from the remote cluster"
$(dirname $0)/open-gardener-tunnels $REMOTE_GARDEN_LABEL

echo "# Now, run \`make dev-setup\` to setup config and certificates files for gardener's components and to register the gardener-apiserver."
echo "# Finally, run \`make start-apiserver,start-controller-manager,start-scheduler,start-gardenlet\` to start the gardener components as usual."