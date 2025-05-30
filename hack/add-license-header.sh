#!/usr/bin/env bash
# SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

set -e

echo "> Adding Apache License header to all go files where it is not present"

# addlicence with a license file (parameter -f) expects no comments in the file.
# LICENSE_BOILERPLATE.txt is however also used also when generating go code.
# Therefore we remove '//' from LICENSE_BOILERPLATE.txt here before passing it to addlicense.

temp_file=$(mktemp)
trap "rm -f $temp_file" EXIT
sed 's|^// *||' hack/LICENSE_BOILERPLATE.txt > $temp_file

addlicense \
  -f $temp_file \
  -ignore ".idea/**" \
  -ignore ".vscode/**" \
  -ignore "dev/**" \
  -ignore "**/*.md" \
  -ignore "**/*.html" \
  -ignore "**/*.yaml" \
  -ignore "**/Dockerfile" \
  -ignore "pkg/**/*.sh" \
  -ignore "third_party/gopkg.in/yaml.v2/**" \
  .
