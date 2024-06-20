// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package containerd_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components"
	. "github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components/containerd"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/imagevector"
)

var _ = Describe("Initializer", func() {
	Describe("#Config", func() {
		var (
			component components.Component
			ctx       components.Context

			images = map[string]*imagevector.Image{
				"pause-container": {
					Name:       "pause-container",
					Repository: pauseContainerImageRepo,
					Tag:        ptr.To(pauseContainerImageTag),
				},
			}
		)

		BeforeEach(func() {
			component = NewInitializer()
			ctx.Images = images
		})

		It("should return the expected units and files", func() {
			units, files, err := component.Config(ctx)

			Expect(err).NotTo(HaveOccurred())
			Expect(units).To(ConsistOf(
				extensionsv1alpha1.Unit{
					Name:    "containerd-initializer.service",
					Command: ptr.To(extensionsv1alpha1.CommandStart),
					Enable:  ptr.To(true),
					Content: ptr.To(`[Unit]
Description=Containerd initializer
[Install]
WantedBy=multi-user.target
[Service]
Type=oneshot
RemainAfterExit=yes
ExecStart=/opt/bin/init-containerd`),
				},
			))
			Expect(files).To(ConsistOf(
				extensionsv1alpha1.File{
					Path:        "/opt/bin/init-containerd",
					Permissions: ptr.To[int32](744),
					Content: extensionsv1alpha1.FileContent{
						Inline: &extensionsv1alpha1.FileContentInline{
							Encoding: "b64",
							Data:     utils.EncodeBase64([]byte(initScript)),
						},
					},
				},
				extensionsv1alpha1.File{
					Path:        "/etc/systemd/system/containerd.service.d/10-require-containerd-initializer.conf",
					Permissions: ptr.To[int32](0644),
					Content: extensionsv1alpha1.FileContent{
						Inline: &extensionsv1alpha1.FileContentInline{
							Data: `[Unit]
After=containerd-initializer.service
Requires=containerd-initializer.service`,
						},
					},
				},
			))
		})
	})
})

const (
	pauseContainerImageRepo = "foo.io"
	pauseContainerImageTag  = "v1.2.3"
	initScript              = `#!/bin/bash

FILE=/etc/containerd/config.toml
if [ ! -s "$FILE" ]; then
  mkdir -p $(dirname $FILE)
  containerd config default > "$FILE"
fi

# use injected image as sandbox image
sandbox_image_line="$(grep sandbox_image $FILE | sed -e 's/^[ ]*//')"
pause_image=` + pauseContainerImageRepo + `:` + pauseContainerImageTag + `
sed -i  "s|$sandbox_image_line|sandbox_image = \"$pause_image\"|g" $FILE

# create and configure registry hosts directory
CONFIG_PATH=/etc/containerd/certs.d
if ! grep --quiet --fixed-strings '[plugins."io.containerd.grpc.v1.cri".registry]' "$FILE"; then
  echo "CRI registry section not found. Adding CRI registry section with config_path = \"$CONFIG_PATH\" in $FILE."
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

# allow to import custom configuration files
CUSTOM_CONFIG_DIR=/etc/containerd/conf.d
CUSTOM_CONFIG_FILES="$CUSTOM_CONFIG_DIR/*.toml"
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

BIN_PATH=/var/bin/containerruntimes

ENV_FILE=/etc/systemd/system/containerd.service.d/30-env_config.conf
if [ ! -f "$ENV_FILE" ]; then
  cat <<EOF | tee $ENV_FILE
[Service]
Environment="PATH=$BIN_PATH:$PATH"
EOF
  systemctl daemon-reload
fi
`
)
