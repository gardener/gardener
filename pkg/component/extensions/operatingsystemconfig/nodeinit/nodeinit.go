// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package nodeinit

import (
	"bytes"
	_ "embed"
	"fmt"
	"html/template"

	"k8s.io/utils/ptr"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components/nodeagent"
	nodeagentv1alpha1 "github.com/gardener/gardener/pkg/nodeagent/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/utils"
)

// PathInitScript is the path to the init script.
const PathInitScript = nodeagentv1alpha1.BaseDir + "/init.sh"

// Config returns the init units and the files for the OperatingSystemConfig for bootstrapping the gardener-node-agent.
// ### !CAUTION! ###
// Most cloud providers have a limit of 16 KB regarding the user-data that may be sent during VM creation.
// The result of this operating system config is exactly the user-data that will be sent to the providers.
// We must not exceed the 16 KB, so be careful when extending/changing anything in here.
// ### !CAUTION! ###
func Config(
	worker gardencorev1beta1.Worker,
	nodeAgentImage string,
	config *nodeagentv1alpha1.NodeAgentConfiguration,
) (
	[]extensionsv1alpha1.Unit,
	[]extensionsv1alpha1.File,
	error,
) {
	initScript, err := generateInitScript(nodeAgentImage)
	if err != nil {
		return nil, nil, fmt.Errorf("failed generating init script: %w", err)
	}

	var (
		nodeInitUnits = []extensionsv1alpha1.Unit{{
			Name:    nodeagentv1alpha1.InitUnitName,
			Command: ptr.To(extensionsv1alpha1.CommandStart),
			Enable:  ptr.To(true),
			Content: ptr.To(`[Unit]
Description=Downloads the gardener-node-agent binary from the container registry and bootstraps it.
After=network-online.target
Wants=network-online.target
[Service]
Type=oneshot
Restart=on-failure
RestartSec=5
StartLimitBurst=0
EnvironmentFile=/etc/environment
ExecStart=` + PathInitScript + `
[Install]
WantedBy=multi-user.target`),
			FilePaths: []string{PathInitScript},
		}}

		nodeInitFiles = []extensionsv1alpha1.File{
			{
				Path:        nodeagentv1alpha1.BootstrapTokenFilePath,
				Permissions: ptr.To[int32](0640),
				Content: extensionsv1alpha1.FileContent{
					Inline: &extensionsv1alpha1.FileContentInline{
						// The bootstrap token will be created by the machine-controller-manager when creating an actual
						// machine, and it will replace this "magic" string with the actual token in the user data. See
						// https://github.com/gardener/gardener/blob/master/docs/extensions/resources/operatingsystemconfig.md#bootstrap-tokens
						// for more details.
						Data: "<<BOOTSTRAP_TOKEN>>",
					},
					TransmitUnencoded: ptr.To(true),
				},
			},
			{
				Path:        PathInitScript,
				Permissions: ptr.To[int32](0755),
				Content: extensionsv1alpha1.FileContent{
					Inline: &extensionsv1alpha1.FileContentInline{
						Encoding: "b64",
						Data:     utils.EncodeBase64(initScript),
					},
				},
			},
		}
	)

	// The gardener-node-init script above will bootstrap the gardener-node-agent. This means that the unit file for
	// the gardener-node-agent unit will be written and eventually started (whilst gardener-node-init disables and stops
	// itself). Hence, the files for gardener-node-agent (component configuration and kubeconfig) must be present on the
	// machine so that it can start successfully.
	config = config.DeepCopy()
	config.Bootstrap, err = getBootstrapConfiguration(worker)
	if err != nil {
		return nil, nil, fmt.Errorf("failed computing bootstrap configuration: %w", err)
	}

	nodeAgentFiles, err := nodeagent.Files(config)
	if err != nil {
		return nil, nil, fmt.Errorf("failed computing gardener-node-agent files: %w", err)
	}
	nodeInitFiles = append(nodeInitFiles, nodeAgentFiles...)

	return nodeInitUnits, nodeInitFiles, nil
}

var (
	//go:embed templates/scripts/init.tpl.sh
	initScriptTplContent string
	initScriptTpl        *template.Template
)

func init() {
	initScriptTpl = template.Must(template.New("init-script").Parse(initScriptTplContent))
}

func generateInitScript(nodeAgentImage string) ([]byte, error) {
	var initScript bytes.Buffer
	if err := initScriptTpl.Execute(&initScript, map[string]any{
		"image":           nodeAgentImage,
		"binaryDirectory": nodeagentv1alpha1.BinaryDir,
		"configFile":      nodeagentv1alpha1.ConfigFilePath,
	}); err != nil {
		return nil, err
	}

	return initScript.Bytes(), nil
}
