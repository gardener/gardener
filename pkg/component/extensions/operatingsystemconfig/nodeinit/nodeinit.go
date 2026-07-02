// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package nodeinit

import (
	"bytes"
	_ "embed"
	"fmt"
	"text/template"

	machinecontroller "github.com/gardener/machine-controller-manager/pkg/util/provider/machinecontroller"

	nodeagentconfigv1alpha1 "github.com/gardener/gardener/pkg/apis/config/nodeagent/v1alpha1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components/nodeagent"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components/rootcertificates"
	"github.com/gardener/gardener/pkg/utils"
)

// PathInitScript is the path to the init script.
const PathInitScript = nodeagentconfigv1alpha1.BaseDir + "/init.sh"

// Config returns the init units and the files for the OperatingSystemConfig for bootstrapping the gardener-node-agent.
// ### !CAUTION! ###
// Most cloud providers have a limit of 16 KB regarding the user-data that may be sent during VM creation.
// The result of this operating system config is exactly the user-data that will be sent to the providers.
// We must not exceed the 16 KB, so be careful when extending/changing anything in here.
// ### !CAUTION! ###
func Config(
	worker gardencorev1beta1.Worker,
	nodeAgentImage string,
	config *nodeagentconfigv1alpha1.NodeAgentConfiguration,
	clusterCABundle []byte,
	registryCAEnabled bool,
) (
	[]extensionsv1alpha1.Unit,
	[]extensionsv1alpha1.File,
	error,
) {
	initScript, err := generateInitScript(nodeAgentImage, registryCAEnabled, config.APIServer.Server)
	if err != nil {
		return nil, nil, fmt.Errorf("failed generating init script: %w", err)
	}

	var (
		nodeInitUnits = []extensionsv1alpha1.Unit{
			generateInitScriptUnit(nodeagentconfigv1alpha1.InitUnitName, "gardener-node-agent", PathInitScript, registryCAEnabled),
		}

		nodeInitFiles = []extensionsv1alpha1.File{
			{
				Path:        nodeagentconfigv1alpha1.ClusterCAFilePath,
				Permissions: new(uint32(0640)),
				Content: extensionsv1alpha1.FileContent{
					Inline: &extensionsv1alpha1.FileContentInline{
						Encoding: "b64",
						Data:     utils.EncodeBase64(clusterCABundle),
					},
				},
			},
			{
				Path:        nodeagentconfigv1alpha1.BootstrapTokenFilePath,
				Permissions: new(uint32(0640)),
				Content: extensionsv1alpha1.FileContent{
					Inline: &extensionsv1alpha1.FileContentInline{
						Data: machinecontroller.BootstrapTokenPlaceholder,
					},
					TransmitUnencoded: new(true),
				},
			},
			{
				Path:        PathInitScript,
				Permissions: new(uint32(0755)),
				Content: extensionsv1alpha1.FileContent{
					Inline: &extensionsv1alpha1.FileContentInline{
						Encoding: "b64",
						Data:     utils.EncodeBase64(initScript),
					},
				},
			},
			{
				Path:        nodeagentconfigv1alpha1.MachineNameFilePath,
				Permissions: new(uint32(0640)),
				Content: extensionsv1alpha1.FileContent{
					Inline: &extensionsv1alpha1.FileContentInline{
						Data: machinecontroller.MachineNamePlaceholder,
					},
					TransmitUnencoded: new(true),
				},
			},
		}
	)

	if registryCAEnabled {
		updateCACertsScriptFile, err := rootcertificates.UpdateLocalCACertificatesScriptFile()
		if err != nil {
			return nil, nil, fmt.Errorf("failed generating update-ca-certificates script file: %w", err)
		}
		nodeInitFiles = append(nodeInitFiles, updateCACertsScriptFile)
	}

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

func generateInitScript(nodeAgentImage string, registryCAEnabled bool, apiServerURL string) ([]byte, error) {
	var initScript bytes.Buffer

	data := map[string]any{
		"image":           nodeAgentImage,
		"binaryName":      "gardener-node-agent",
		"binaryDirectory": nodeagentconfigv1alpha1.BinaryDir,
		"configDir":       nodeagentconfigv1alpha1.BaseDir,
	}

	if registryCAEnabled {
		data["registryCAEnabled"] = registryCAEnabled
		data["localCACertsDir"] = rootcertificates.PathLocalSSLCerts
		data["registryCAFilePath"] = rootcertificates.PathLocalSSLRegistryCACerts
		data["pkiTrustAnchorsDir"] = rootcertificates.PathPKITrustAnchors
		data["registryCAFilePathPKI"] = rootcertificates.PathPKITrustAnchorsRegistryCACerts
		data["bootstrapTokenFilePath"] = nodeagentconfigv1alpha1.BootstrapTokenFilePath
		data["clusterCAFilePath"] = nodeagentconfigv1alpha1.ClusterCAFilePath
		data["apiServerURL"] = apiServerURL
		data["updateCACertificatesScript"] = rootcertificates.PathUpdateLocalCACertificates
	}

	if err := initScriptTpl.Execute(&initScript, data); err != nil {
		return nil, err
	}

	return initScript.Bytes(), nil
}

func generateInitScriptUnit(unitName, binaryName, filePath string, registryCAEnabled bool) extensionsv1alpha1.Unit {
	// When a custom registry CA is configured, the init script restarts containerd after updating the
	// system CA store so that the running daemon picks up the new certificate. Using Wants instead of
	// Requires ensures that this temporary stop of containerd does not cause systemd to also stop the
	// init service itself due to the dependency which would cause an infinite loop.
	containerdDep := "Requires=containerd.service"
	if registryCAEnabled {
		containerdDep = "Wants=containerd.service"
	}
	return extensionsv1alpha1.Unit{
		Name:    unitName,
		Command: new(extensionsv1alpha1.CommandStart),
		Enable:  new(true),
		Content: new(`[Unit]
Description=Downloads the ` + binaryName + ` binary from the container registry and bootstraps it.
` + containerdDep + `
After=containerd.service
After=network-online.target
Wants=network-online.target
[Service]
Type=oneshot
Restart=on-failure
RestartSec=5
StartLimitBurst=0
EnvironmentFile=/etc/environment
ExecStart=` + filePath + `
StandardOutput=journal+console
StandardError=journal+console
[Install]
WantedBy=multi-user.target`),
		FilePaths: []string{filePath},
	}
}
