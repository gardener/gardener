// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package kubernetesversion

import (
	"fmt"

	versionutils "github.com/gardener/gardener/pkg/utils/version"
)

// SupportedVersions is the list of supported Kubernetes versions for all runtime and target clusters, i.e. all gardens,
// seeds, and shoots.
var SupportedVersions = []string{
	"1.22",
	"1.23",
	"1.24",
	"1.25",
	"1.26",
	"1.27",
	"1.28",
}

// CheckIfSupported checks if the provided version is part of the supported Kubernetes versions list.
func CheckIfSupported(gitVersion string) error {
	for _, supportedVersion := range SupportedVersions {
		ok, err := versionutils.CompareVersions(gitVersion, "~", supportedVersion)
		if err != nil {
			return err
		}

		if ok {
			return nil
		}
	}

	return fmt.Errorf("unsupported kubernetes version %q", gitVersion)
}
