// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package helper

import (
	"strings"

	"k8s.io/utils/ptr"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
)

// IsResourceSupported returns true if a given combination of kind/type is part of a controller resources list.
func IsResourceSupported(resources []gardencorev1beta1.ControllerResource, resourceKind, resourceType string) bool {
	for _, resource := range resources {
		if resource.Kind == resourceKind && strings.EqualFold(resource.Type, resourceType) {
			return true
		}
	}

	return false
}

// ContinuousEndpointUpdateEnabled returns whether the SelfHostedShootExposure controller registered for the given
// extension type opts into continuous endpoint updates. Defaults to the API default (true).
func ContinuousEndpointUpdateEnabled(registrations []gardencorev1beta1.ControllerRegistration, extensionType string) bool {
	for _, reg := range registrations {
		for _, res := range reg.Spec.Resources {
			if res.Kind == extensionsv1alpha1.SelfHostedShootExposureResource && strings.EqualFold(res.Type, extensionType) {
				return ptr.Deref(res.ContinuousEndpointUpdate, true)
			}
		}
	}
	return true
}
