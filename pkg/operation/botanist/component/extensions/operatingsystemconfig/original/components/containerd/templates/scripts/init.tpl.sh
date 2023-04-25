#!/bin/bash

FILE=/etc/containerd/config.toml
if [ ! -s "$FILE" ]; then
  mkdir -p $(dirname $FILE)
  containerd config default > "$FILE"
fi

# use injected image as sandbox image
pause_image={{ .pauseContainerImage }}
sed -i  "s|sandbox_image.*$|sandbox_image = \"$pause_image\"|" $FILE

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
