#!/bin/bash

FILE=/etc/containerd/config.toml
if [ ! -s "$FILE" ]; then
  mkdir -p $(dirname $FILE)
  containerd config default > "$FILE"
fi

# use injected image as sandbox image
sandbox_image_line="$(grep sandbox_image $FILE | sed -e 's/^[ ]*//')"
pause_image={{ .pauseContainerImage }}
sed -i  "s|$sandbox_image_line|sandbox_image = \"$pause_image\"|g" $FILE

# create and configure registry hosts directory
# or remove registry hosts directory configuration
{{- if .containerdRegistryHostsDirEnabled }}
CONFIG_PATH=/etc/containerd/certs.d
mkdir -p "$CONFIG_PATH"
if ! grep --quiet --fixed-strings '[plugins."io.containerd.grpc.v1.cri".registry]' "$FILE"; then
  echo "CRI registry section not found. Adding CRI registry section with config_path = \"$CONFIG_PATH\" in $FILE."
  # TODO(ialidzhikov): Drop the "# gardener-managed" comment when removing the ContainerdRegistryHostsDir feature gate.
  # Currently we need such comment to distinguish whether the config is added by this script or externally by the Shoot owner.
  # When the feature gate is disabled, the config section is being removed only when it was added by this script.
  cat <<EOF >> $FILE
[plugins."io.containerd.grpc.v1.cri".registry] # gardener-managed
  config_path = "/etc/containerd/certs.d"
EOF
else
  if grep --quiet --fixed-strings '[plugins."io.containerd.grpc.v1.cri".registry] # gardener-managed' "$FILE"; then
    echo "CRI registry section is already gardener managed. Nothing to do."
  else
    echo "CRI registry section is not gardener managed. Setting config_path = \"$CONFIG_PATH\" in $FILE."
    sed --null-data --in-place 's/\(\[plugins\."io\.containerd\.grpc\.v1\.cri"\.registry\]\)\n\(\s*config_path\s*=\s*\)""\n/\1 \# gardener-managed\n\2"\/etc\/containerd\/certs.d"\n/' "$FILE"
  fi
fi
{{- else }}
if grep --quiet --fixed-strings '[plugins."io.containerd.grpc.v1.cri".registry] # gardener-managed' "$FILE"; then
  echo "CRI registry section is gardener managed. Removing CRI registry section from $FILE."
  sed --null-data --in-place 's/\[plugins\."io\.containerd\.grpc\.v1\.cri"\.registry\]\ #\ gardener-managed\n\s*config_path.*\n//' "$FILE"
else
  echo "A gardener managed CRI section is not found. Nothing to do."
fi
{{- end }}

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
