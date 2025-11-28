// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package helper

import (
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
)

// GetBootstrap returns the value of the given Bootstrap, or None if nil.
func GetBootstrap(bootstrap *seedmanagementv1alpha1.Bootstrap) seedmanagementv1alpha1.Bootstrap {
	if bootstrap != nil {
		return *bootstrap
	}
	return seedmanagementv1alpha1.BootstrapNone
}
