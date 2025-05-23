// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"strconv"

	"sigs.k8s.io/controller-runtime/pkg/client"

	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
)

// IsIgnored returns true when the resources.gardener.cloud/ignore annotation has a truthy value.
func IsIgnored(obj client.Object) bool {
	value, ok := obj.GetAnnotations()[resourcesv1alpha1.Ignore]
	if !ok {
		return false
	}
	truthy, _ := strconv.ParseBool(value)
	return truthy
}
