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

SCRIPT_DIR="$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"

# Ensure that if GOPATH is set, the GOPATH/{bin,pkg} directory exists. This seems to be not always
# the case in certain environments like Prow. As we will create a symlink against the bin folder we
# need to make sure that the bin directory is present in the GOPATH.
if [ -n "$GOPATH" ] && [ ! -d "$GOPATH/bin" ]; then mkdir -p "$GOPATH/bin"; fi
if [ -n "$GOPATH" ] && [ ! -d "$GOPATH/pkg" ]; then mkdir -p "$GOPATH/pkg"; fi

VIRTUAL_GOPATH="$(mktemp -d)"
trap 'rm -rf "$VIRTUAL_GOPATH"' EXIT

# Use REPO_ROOT if set, otherwise default to $SCRIPT_DIR/..
TARGET_DIR="${REPO_ROOT:-$SCRIPT_DIR/..}"

# Setup virtual GOPATH
(cd "$TARGET_DIR"; go mod download && "$VGOPATH" -o "$VIRTUAL_GOPATH")

export GOROOT="${GOROOT:-"$(go env GOROOT)"}"
export GOPATH="$VIRTUAL_GOPATH"
