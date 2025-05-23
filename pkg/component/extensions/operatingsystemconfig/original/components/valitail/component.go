// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package valitail

import (
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener/imagevector"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components"
)

const (
	// UnitName is the name of the valitail service.
	UnitName = v1beta1constants.OperatingSystemConfigUnitNameValitailService

	// PathDirectory is the path for the valitail's directory.
	PathDirectory = "/var/lib/valitail"
	// PathAuthToken is the path for the file containing valitail's authentication token for communication with the Vali
	// sidecar proxy.
	PathAuthToken = PathDirectory + "/auth-token"
	// PathConfig is the path for the valitail's configuration file.
	PathConfig = v1beta1constants.OperatingSystemConfigFilePathValitailConfig
	// PathCACert is the path for the vali-tls certificate authority.
	PathCACert = PathDirectory + "/ca.crt"

	valitailBinaryPath = v1beta1constants.OperatingSystemConfigFilePathBinaries + "/valitail"
)

type component struct{}

// New returns a new valitail component.
func New() *component {
	return &component{}
}

func (component) Name() string {
	return "valitail"
}

func (component) Config(ctx components.Context) ([]extensionsv1alpha1.Unit, []extensionsv1alpha1.File, error) {
	var (
		units []extensionsv1alpha1.Unit
		files []extensionsv1alpha1.File
	)

	if ctx.ValitailEnabled {
		valitailConfigFile, err := getValitailConfigurationFile(ctx)
		if err != nil {
			return nil, nil, err
		}

		units = append(units, getValitailUnit())
		files = append(files, valitailConfigFile, getValitailCAFile(ctx), extensionsv1alpha1.File{
			Path:        valitailBinaryPath,
			Permissions: ptr.To[uint32](0755),
			Content: extensionsv1alpha1.FileContent{
				ImageRef: &extensionsv1alpha1.FileContentImageRef{
					Image:           ctx.Images[imagevector.ContainerImageNameValitail].String(),
					FilePathInImage: "/usr/bin/valitail",
				},
			},
		})
	}

	return units, files, nil
}
