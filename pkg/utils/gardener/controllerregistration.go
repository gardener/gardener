// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gardener

import (
	"slices"

	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/ptr"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
)

func extensionEnabledForCluster(clusterType gardencorev1beta1.ClusterType, resource gardencorev1beta1.ControllerResource, disabledExtensions sets.Set[string]) bool {
	return resource.Kind == extensionsv1alpha1.ExtensionResource &&
		!disabledExtensions.Has(resource.Type) &&
		slices.Contains(resource.AutoEnable, clusterType)
}

func computeEnabledTypesForKindExtension(
	clusterType gardencorev1beta1.ClusterType,
	extensions []gardencorev1beta1.Extension,
	controllerRegistrationList *gardencorev1beta1.ControllerRegistrationList,
	resourceFilter func(res gardencorev1beta1.ControllerResource) bool,
) sets.Set[string] {
	var (
		enabledTypes  = sets.New[string]()
		disabledTypes = sets.New[string]()
	)

	for _, extension := range extensions {
		if ptr.Deref(extension.Disabled, false) {
			disabledTypes.Insert(extension.Type)
		} else {
			enabledTypes.Insert(extension.Type)
		}
	}

	for _, controllerRegistration := range controllerRegistrationList.Items {
		for _, resource := range controllerRegistration.Spec.Resources {
			if resourceFilter != nil && !resourceFilter(resource) {
				continue
			}
			if extensionEnabledForCluster(clusterType, resource, disabledTypes) {
				enabledTypes.Insert(resource.Type)
			}
		}
	}

	return enabledTypes
}
