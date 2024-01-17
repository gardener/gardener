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

package valitail

import (
	"github.com/gardener/gardener/imagevector"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components"
	"github.com/gardener/gardener/pkg/features"
	imagevectorutils "github.com/gardener/gardener/pkg/utils/imagevector"
	"k8s.io/utils/ptr"
)

const (
	// UnitName is the name of the valitail service.
	UnitName           = v1beta1constants.OperatingSystemConfigUnitNameValitailService
	unitNameFetchToken = "valitail-fetch-token.service"

	// PathDirectory is the path for the valitail's directory.
	PathDirectory = "/var/lib/valitail"
	// PathFetchTokenScript is the path to a script which fetches valitail's token for communication with the Vali
	// sidecar proxy.
	PathFetchTokenScript = PathDirectory + "/scripts/fetch-token.sh"
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

func execStartPreCopyBinaryFromContainer(binaryName string, image *imagevectorutils.Image) string {
	return `/usr/bin/docker run --rm -v ` + v1beta1constants.OperatingSystemConfigFilePathBinaries + `:` + v1beta1constants.OperatingSystemConfigFilePathBinaries + `:rw --entrypoint /bin/sh ` + image.String() + ` -c "cp /usr/bin/` + binaryName + ` ` + v1beta1constants.OperatingSystemConfigFilePathBinaries + `"`
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

		files = append(files, valitailConfigFile, getValitailCAFile(ctx))

		if features.DefaultFeatureGate.Enabled(features.UseGardenerNodeAgent) {
			units = append(units, getValitailUnit(ctx))
			files = append(files, extensionsv1alpha1.File{
				Path:        valitailBinaryPath,
				Permissions: ptr.To(int32(0755)),
				Content: extensionsv1alpha1.FileContent{
					ImageRef: &extensionsv1alpha1.FileContentImageRef{
						Image:           ctx.Images[imagevector.ImageNameValitail].String(),
						FilePathInImage: "/usr/bin/valitail",
					},
				},
			})
		} else {
			fetchTokenScriptFile, err := getFetchTokenScriptFile()
			if err != nil {
				return nil, nil, err
			}
			files = append(files, fetchTokenScriptFile)
		}
	}

	if !features.DefaultFeatureGate.Enabled(features.UseGardenerNodeAgent) {
		units = append(units, getValitailUnit(ctx))
		units = append(units, getFetchTokenScriptUnit(ctx))
	}

	return units, files, nil
}
