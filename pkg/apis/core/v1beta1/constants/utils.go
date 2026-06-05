// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package constants

import (
	"regexp"
)

// GetShootVPADeploymentNames returns the names of all VPA related deployments related to shoot clusters.
func GetShootVPADeploymentNames() []string {
	return []string{
		DeploymentNameVPAAdmissionController,
		DeploymentNameVPARecommender,
		DeploymentNameVPAUpdater,
	}
}

// ResourceReferenceRegexp matches a Go template expression of the form `{{ .resources.<name>.data.<key> }}`
// (alphanumeric <name> and <key>, optional surrounding whitespace). The named capture group "name" extracts <name>.
var ResourceReferenceRegexp = regexp.MustCompile(`\{\{\s*\.resources\.(?P<name>[a-zA-Z0-9]+)\.data\.[a-zA-Z0-9]+\s*\}\}`)
