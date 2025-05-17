// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

// SetDefaults_Gardenlet sets default values for Gardenlet objects.
func SetDefaults_Gardenlet(obj *Gardenlet) {
	SetDefaults_GardenletDeployment(&obj.Spec.Deployment.GardenletDeployment)
	setDefaultsGardenletConfig(&obj.Spec.Config)
}
