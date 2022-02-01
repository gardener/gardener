#!/bin/bash

FILE=/etc/containerd/config.toml
if [ ! -s "$FILE" ]; then
  mkdir -p $(dirname $FILE)
  containerd config default > "$FILE"
fi

# Inject cGroupDriver
cGroupDriver={{ .cGroupDriver }}
cGroupDriver_line="$(grep SystemdCgroup $FILE | sed -e 's/^[ ]*//')"
if [ "$cGroupDriver" = "systemd" ]; then
  sed -i "s|$cGroupDriver_line|SystemdCgroup = true|g" $FILE
fi

# use injected image as sandbox image
sandbox_image_line="$(grep sandbox_image $FILE | sed -e 's/^[ ]*//')"
pause_image={{ .pauseContainerImage }}
sed -i  "s|$sandbox_image_line|sandbox_image = \"$pause_image\"|g" $FILE

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
