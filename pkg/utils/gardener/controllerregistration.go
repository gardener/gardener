// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gardener

import (
	"context"
	"slices"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
)

// GetControllerRegistrationsForInstallations returns the distinct ControllerRegistrations referenced by the given
// ControllerInstallations, fetched by name (a global List is not permitted for a self-hosted shoot's gardenlet, RBAC).
func GetControllerRegistrationsForInstallations(ctx context.Context, reader client.Reader, controllerInstallations *gardencorev1beta1.ControllerInstallationList) (*gardencorev1beta1.ControllerRegistrationList, error) {
	controllerRegistrations := &gardencorev1beta1.ControllerRegistrationList{}
	seen := sets.New[string]()

	for _, controllerInstallation := range controllerInstallations.Items {
		name := controllerInstallation.Spec.RegistrationRef.Name
		if seen.Has(name) {
			continue
		}
		seen.Insert(name)

		controllerRegistration := gardencorev1beta1.ControllerRegistration{}
		if err := reader.Get(ctx, client.ObjectKey{Name: name}, &controllerRegistration); err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			return nil, err
		}
		controllerRegistrations.Items = append(controllerRegistrations.Items, controllerRegistration)
	}

	return controllerRegistrations, nil
}

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
