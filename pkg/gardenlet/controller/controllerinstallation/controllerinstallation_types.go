// SPDX-FileCopyrightText: 2018 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controllerinstallation

// HelmDeployment is a providerConfig specific type for ControllerInstallation.
type HelmDeployment struct {
	// Chart is a Helm chart tarball.
	Chart []byte `json:"chart,omitempty"`
	// Values is a map of values for the given chart.
	Values map[string]interface{} `json:"values,omitempty"`
}
