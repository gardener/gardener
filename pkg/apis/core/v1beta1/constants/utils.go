// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package constants

// GetShootVPADeploymentNames returns the names of all VPA related deployments related to shoot clusters.
func GetShootVPADeploymentNames() []string {
	return []string{
		DeploymentNameVPAAdmissionController,
		DeploymentNameVPARecommender,
		DeploymentNameVPAUpdater,
	}
}
