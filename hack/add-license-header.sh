#!/usr/bin/env bash
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

set -e

echo "> Adding Apache License header to all go files where it is not present"

addlicense \
  -c "SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file." \
  -y "$(date +"%Y")" \
  -l apache \
  -ignore ".idea/**" \
  -ignore ".vscode/**" \
  -ignore "dev/**" \
  -ignore "vendor/**" \
  -ignore "**/*.md" \
  -ignore "**/*.html" \
  -ignore "**/*.yaml" \
  -ignore "**/Dockerfile" \
  -ignore "hack/tools/gomegacheck/**" \
  -ignore "pkg/component/**/*.sh" \
  -ignore "third_party/gopkg.in/yaml.v2/**" \
  .
