#!/usr/bin/env bash
# SPDX-FileCopyrightText: 2019 SAP SE or an SAP affiliate company and Gardener contributors
# SPDX-License-Identifier: Apache-2.0

set -e

echo "> Adding Apache License header to all go files where it is not present"

addlicense \
  -c "SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file." \
  -y "$(date +"%Y")" \
  -l apache \
  -ignore ".idea/**" \
  -ignore ".vscode/**" \
  -ignore "dev/**" \
  -ignore "**/*.md" \
  -ignore "**/*.html" \
  -ignore "**/*.yaml" \
  -ignore "**/Dockerfile" \
  -ignore "pkg/component/**/*.sh" \
  -ignore "third_party/gopkg.in/yaml.v2/**" \
  .
