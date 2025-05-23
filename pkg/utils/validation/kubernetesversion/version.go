// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package kubernetesversion

import (
	"fmt"

	versionutils "github.com/gardener/gardener/pkg/utils/version"
)

// SupportedVersions is the list of supported Kubernetes versions for all runtime and target clusters, i.e. all gardens,
// seeds, and shoots.
var SupportedVersions = []string{
	"1.27",
	"1.28",
	"1.29",
	"1.30",
	"1.31",
	"1.32",
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
