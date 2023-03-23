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

echo "# Cleanup tunnels for accessing local gardener components from the remote cluster"
$(dirname $0)/open-gardener-tunnels -c
echo "# Remove docker containers and networks"
$(dirname $0)/cleanup $REMOTE_GARDEN_LABEL