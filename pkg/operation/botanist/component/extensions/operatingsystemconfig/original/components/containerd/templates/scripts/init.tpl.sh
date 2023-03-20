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


FILE=/etc/containerd/config.toml
if [ ! -s "$FILE" ]; then
  mkdir -p $(dirname $FILE)
  containerd config default > "$FILE"
fi

# use injected image as sandbox image
sandbox_image_line="$(grep sandbox_image $FILE | sed -e 's/^[ ]*//')"
pause_image={{ .pauseContainerImage }}
sed -i  "s|$sandbox_image_line|sandbox_image = \"$pause_image\"|g" $FILE

# allow to import custom configuration files
CUSTOM_CONFIG_DIR=/etc/containerd/conf.d
CUSTOM_CONFIG_FILES="$CUSTOM_CONFIG_DIR/*.toml"
mkdir -p $CUSTOM_CONFIG_DIR
if ! grep -E "^imports" $FILE >/dev/null ; then
  # imports directive not present -> add it to the top
  existing_content="$(cat "$FILE")"
  cat <<EOF > $FILE
imports = ["$CUSTOM_CONFIG_FILES"]
$existing_content
EOF
elif ! grep -F "$CUSTOM_CONFIG_FILES" $FILE >/dev/null ; then
  # imports directive present, but does not contain conf.d -> append conf.d to imports
  existing_imports="$(grep -E "^imports" $FILE | sed -E 's#imports = \[(.*)\]#\1#g')"
  [ -z "$existing_imports" ] || existing_imports="$existing_imports, "
  sed -Ei 's#imports = \[(.*)\]#imports = ['"$existing_imports"'"'"$CUSTOM_CONFIG_FILES"'"]#g' $FILE
fi

BIN_PATH={{ .binaryPath }}
mkdir -p $BIN_PATH

ENV_FILE=/etc/systemd/system/containerd.service.d/30-env_config.conf
if [ ! -f "$ENV_FILE" ]; then
  cat <<EOF | tee $ENV_FILE
[Service]
Environment="PATH=$BIN_PATH:$PATH"
EOF
  systemctl daemon-reload
fi
