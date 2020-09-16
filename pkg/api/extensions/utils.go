// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package extensions

import (
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
)

// GetShootNamespacedCRsLists returns an empty CR list struct, for each CR used for Shoot managment
func GetShootNamespacedCRsLists() []runtime.Object {
	return []runtime.Object{
		&extensionsv1alpha1.ControlPlaneList{},
		&extensionsv1alpha1.ExtensionList{},
		&extensionsv1alpha1.InfrastructureList{},
		//The Network CR is now handled as a shoot component
		//&extensionsv1alpha1.NetworkList{},
		&extensionsv1alpha1.OperatingSystemConfigList{},
		&extensionsv1alpha1.WorkerList{},
		//The ContainerRuntime CR is now handled as a shoot component
		//&extensionsv1alpha1.ContainerRuntimeList{},
	}
}
