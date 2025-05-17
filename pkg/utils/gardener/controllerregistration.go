// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gardener

import (
	"slices"

	"k8s.io/apimachinery/pkg/util/sets"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
)

func extensionEnabledForCluster(clusterType gardencorev1beta1.ClusterType, resource gardencorev1beta1.ControllerResource, disabledExtensions sets.Set[string]) bool {
	return resource.Kind == extensionsv1alpha1.ExtensionResource &&
		!disabledExtensions.Has(resource.Type) &&
		slices.Contains(resource.AutoEnable, clusterType)
}
