// Copyright 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package containerd_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/pointer"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components"
	. "github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components/containerd"
	"github.com/gardener/gardener/pkg/features"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	"github.com/gardener/gardener/pkg/utils/test"
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
					Tag:        pointer.String(pauseContainerImageTag),
				},
			}
		)

		BeforeEach(func() {
			component = NewInitializer()
			ctx.Images = images
		})

		DescribeTable("should return the expected units and files",
			func(containerdRegistryHostsDirEnabled bool) {
				defer test.WithFeatureGate(features.DefaultFeatureGate, features.ContainerdRegistryHostsDir, containerdRegistryHostsDirEnabled)()

				units, files, err := component.Config(ctx)

				Expect(err).NotTo(HaveOccurred())
				Expect(units).To(ConsistOf(
					extensionsv1alpha1.Unit{
						Name:    "containerd-initializer.service",
						Command: pointer.String("start"),
						Enable:  pointer.Bool(true),
						Content: pointer.String(`[Unit]
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
						Permissions: pointer.Int32(744),
						Content: extensionsv1alpha1.FileContent{
							Inline: &extensionsv1alpha1.FileContentInline{
								Encoding: "b64",
								Data:     utils.EncodeBase64([]byte(initScriptFor(containerdRegistryHostsDirEnabled))),
							},
						},
					},
					extensionsv1alpha1.File{
						Path:        "/etc/systemd/system/containerd.service.d/10-require-containerd-initializer.conf",
						Permissions: pointer.Int32(0644),
						Content: extensionsv1alpha1.FileContent{
							Inline: &extensionsv1alpha1.FileContentInline{
								Data: `[Unit]
After=containerd-initializer.service
Requires=containerd-initializer.service`,
							},
						},
					},
				))
			},

			Entry("when ContainerdRegistryHostsDir feature gate is disabled", false),
			Entry("when ContainerdRegistryHostsDir feature gate is enabled", true),
		)
	})
})

const (
	pauseContainerImageRepo = "foo.io"
	pauseContainerImageTag  = "v1.2.3"
)

func initScriptFor(containerdRegistryHostsDirEnabled bool) string {
	var registryHostsDirPart string
	if containerdRegistryHostsDirEnabled {
		registryHostsDirPart = `CONFIG_PATH=/etc/containerd/certs.d
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
fi`
	} else {
		registryHostsDirPart = `if grep --quiet --fixed-strings '[plugins."io.containerd.grpc.v1.cri".registry] # gardener-managed' "$FILE"; then
  echo "CRI registry section is gardener managed. Removing CRI registry section from $FILE."
  sed --null-data --in-place 's/\[plugins\."io\.containerd\.grpc\.v1\.cri"\.registry\]\ #\ gardener-managed\n\s*config_path.*\n//' "$FILE"
else
  echo "A gardener managed CRI section is not found. Nothing to do."
fi`
	}

	return `#!/bin/bash

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
# or remove registry hosts directory configuration
` + registryHostsDirPart + `

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

BIN_PATH=/var/bin/containerruntimes
mkdir -p $BIN_PATH

ENV_FILE=/etc/systemd/system/containerd.service.d/30-env_config.conf
if [ ! -f "$ENV_FILE" ]; then
  cat <<EOF | tee $ENV_FILE
[Service]
Environment="PATH=$BIN_PATH:$PATH"
EOF
  systemctl daemon-reload
fi
`
}
