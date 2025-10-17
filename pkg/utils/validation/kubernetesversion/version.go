// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package kubernetesversion

import (
	"fmt"
	"os"

	versionutils "github.com/gardener/gardener/pkg/utils/version"
)

// SupportedVersions is the list of supported Kubernetes versions for all runtime and target clusters, i.e. all gardens,
// seeds, and shoots.
var SupportedVersions = []string{
	"1.30",
	"1.31",
	"1.32",
	"1.33",
	"1.34",
}

// DISABLE_VERSION_CHECK_ENV holds the name of the environment variable to prevent a crash
// if the detected k8s version is not in the list of supported k8s versions
const DISABLE_VERSION_CHECK_ENV = "DO_NOT_CRASH_ON_UNSUPPORTED_KUBERNETES_VERSION"

// CheckIfSupported checks if the provided version is part of the supported Kubernetes versions list.
func CheckIfSupported(gitVersion string) error {
	if os.Getenv(DISABLE_VERSION_CHECK_ENV) == "true" {
		return nil
	}

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
