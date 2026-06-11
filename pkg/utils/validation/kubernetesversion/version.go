// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package kubernetesversion

import (
	"fmt"
	"os"

	logf "sigs.k8s.io/controller-runtime/pkg/log"

	versionutils "github.com/gardener/gardener/pkg/utils/version"
)

// SupportedVersions is the list of supported Kubernetes versions for all runtime and target clusters, i.e. all gardens,
// seeds, and shoots.
var SupportedVersions = []string{
	"1.32",
	"1.33",
	"1.34",
	"1.35",
	"1.36",
}

// EnvExperimentalDisableKubernetesVersionCheck holds the name of the environment variable to prevent a crash
// if the detected k8s version is not in the list of supported k8s versions.
// This should only be used if you know exactly what you are doing and on your own risk.
const EnvExperimentalDisableKubernetesVersionCheck = "EXPERIMENTAL_DISABLE_KUBERNETES_VERSION_CHECK"

// CheckIfSupported checks if the provided version is part of the supported Kubernetes versions list.
// Experimental: If the environment variable `EXPERIMENTAL_DISABLE_KUBERNETES_VERSION_CHECK` is set to "true",
// the check will be skipped.
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

	if os.Getenv(EnvExperimentalDisableKubernetesVersionCheck) == "true" {
		logf.Log.Info("Proceeding with unsupported Kubernetes version (version check disabled via flag)", "version", gitVersion, "flag", EnvExperimentalDisableKubernetesVersionCheck)
		return nil
	}

	return fmt.Errorf("unsupported kubernetes version %q", gitVersion)
}
