// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package bootstrap

import (
	"bytes"
	_ "embed"
	"fmt"
	"os/exec"
	"text/template"

	"github.com/go-logr/logr"
	"github.com/spf13/afero"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	nodeagentconfigv1alpha1 "github.com/gardener/gardener/pkg/nodeagent/apis/config/v1alpha1"
)

var (
	//go:embed templates/scripts/format-kubelet-data-volume.tpl.sh
	formatKubeletDataVolumeTplContent string
	formatKubeletDataVolumeTpl        *template.Template

	// ExecScript is a function for executing the formatting script.
	// Exposed for testing.
	ExecScript = func(scriptPath string) ([]byte, error) {
		return exec.Command("/usr/bin/env", "bash", scriptPath).CombinedOutput()
	}
)

func init() {
	formatKubeletDataVolumeTpl = template.Must(template.New("format-kubelet-data-volume").Parse(formatKubeletDataVolumeTplContent))
}

func formatKubeletDataDevice(log logr.Logger, fs afero.Afero, kubeletDataVolumeSize int64) error {
	log.Info("Rendering script")
	var formatKubeletDataVolumeScript bytes.Buffer
	if err := formatKubeletDataVolumeTpl.Execute(&formatKubeletDataVolumeScript, map[string]any{"kubeletDataVolumeSize": kubeletDataVolumeSize}); err != nil {
		return fmt.Errorf("failed rendering script: %w", err)
	}

	log.Info("Creating temporary file")
	tmpFile, err := fs.TempFile(nodeagentconfigv1alpha1.TempDir, "format-kubelet-data-volume-")
	if err != nil {
		return fmt.Errorf("unable to create temporary directory: %w", err)
	}

	defer func() {
		log.Info("Removing temporary file", "path", tmpFile.Name())
		utilruntime.HandleError(fs.Remove(tmpFile.Name()))
	}()

	log.Info("Writing script into temporary file", "path", tmpFile.Name())
	if err := fs.WriteFile(tmpFile.Name(), formatKubeletDataVolumeScript.Bytes(), 0755); err != nil {
		return fmt.Errorf("unable to write script into temporary file %q: %w", tmpFile.Name(), err)
	}

	log.Info("Executing script")
	output, err := ExecScript(tmpFile.Name())
	if err != nil {
		return fmt.Errorf("failed executing formatter bash script: %w (output: %q)", err, string(output))
	}

	log.Info("Successfully formatted kubelet data volume", "output", string(output))
	return nil
}
