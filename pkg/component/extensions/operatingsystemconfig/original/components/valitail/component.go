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
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components/docker"
	"github.com/gardener/gardener/pkg/features"
	imagevectorutils "github.com/gardener/gardener/pkg/utils/imagevector"
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

	// ServerPort is the valitail listening port.
	ServerPort = 3001
	// PositionFile is the path for storing the scraped file offsets.
	PositionFile = "/var/log/positions.yaml"
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
	return docker.PathBinary + ` run --rm -v ` + v1beta1constants.OperatingSystemConfigFilePathBinaries + `:` + v1beta1constants.OperatingSystemConfigFilePathBinaries + `:rw --entrypoint /bin/sh ` + image.String() + ` -c "cp /usr/bin/` + binaryName + ` ` + v1beta1constants.OperatingSystemConfigFilePathBinaries + `"`
}

func (component) Config(ctx components.Context) ([]extensionsv1alpha1.Unit, []extensionsv1alpha1.File, error) {
	valitailUnit, err := getValitailUnit(ctx)
	if err != nil {
		return nil, nil, err
	}

	fetchTokenScriptUnit, err := getFetchTokenScriptUnit(ctx)
	if err != nil {
		return nil, nil, err
	}

	var files []extensionsv1alpha1.File
	if ctx.ValitailEnabled && !features.DefaultFeatureGate.Enabled(features.UseGardenerNodeAgent) {
		valitailConfigFile, err := getValitailConfigurationFile(ctx)
		if err != nil {
			return nil, nil, err
		}

		fetchTokenScriptFile, err := getFetchTokenScriptFile()
		if err != nil {
			return nil, nil, err
		}
		files = append(files, valitailConfigFile, fetchTokenScriptFile, getValitailCAFile(ctx))
	}

	return []extensionsv1alpha1.Unit{valitailUnit, fetchTokenScriptUnit}, files, nil
}
