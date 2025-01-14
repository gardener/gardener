// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package helper

import (
	"strings"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
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
