// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package nodeinit

import (
	"bytes"
	_ "embed"
	"fmt"
	"path/filepath"

	"k8s.io/utils/ptr"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/utils"
)

const (
	// GardenadmBaseDir is the directory on the control plane machine that contains gardenadm-related files.
	GardenadmBaseDir = "/var/lib/gardenadm"
	// GardenadmPathDownloadScript is the path to the download script.
	GardenadmPathDownloadScript = GardenadmBaseDir + "/download.sh"
	// GardenadmBinaryDir is the directory on the control plane machine that contains the gardenadm binary.
	GardenadmBinaryDir = "/opt/bin"
	// GardenadmBinaryName is the name of the gardenadm binary.
	GardenadmBinaryName = "gardenadm"
)

var (
	// GardenadmBinaryPath is the path to the gardenadm binary.
	GardenadmBinaryPath = filepath.Join(GardenadmBinaryDir, GardenadmBinaryName)
)

// GardenadmConfig returns the init units and the files for the OperatingSystemConfig for downloading gardenadm.
// ### !CAUTION! ###
// Most cloud providers have a limit of 16 KB regarding the user-data that may be sent during VM creation.
// The result of this operating system config is exactly the user-data that will be sent to the providers.
// We must not exceed the 16 KB, so be careful when extending/changing anything in here.
// ### !CAUTION! ###
func GardenadmConfig(gardenadmImage string) ([]extensionsv1alpha1.Unit, []extensionsv1alpha1.File, error) {
	downloadScript, err := generateGardenadmDownloadScript(gardenadmImage)
	if err != nil {
		return nil, nil, fmt.Errorf("failed generating download script: %w", err)
	}

	var (
		units = []extensionsv1alpha1.Unit{
			generateInitScriptUnit("gardenadm-download.service", "gardenadm", GardenadmPathDownloadScript),
		}

		files = []extensionsv1alpha1.File{
			{
				Path:        GardenadmPathDownloadScript,
				Permissions: ptr.To[uint32](0755),
				Content: extensionsv1alpha1.FileContent{
					Inline: &extensionsv1alpha1.FileContentInline{
						Encoding: "b64",
						Data:     utils.EncodeBase64(downloadScript),
					},
				},
			},
		}
	)

	return units, files, nil
}

func generateGardenadmDownloadScript(gardenadmImage string) ([]byte, error) {
	var script bytes.Buffer
	if err := initScriptTpl.Execute(&script, map[string]any{
		"image":           gardenadmImage,
		"binaryName":      GardenadmBinaryName,
		"binaryDirectory": GardenadmBinaryDir,
	}); err != nil {
		return nil, err
	}

	return script.Bytes(), nil
}
