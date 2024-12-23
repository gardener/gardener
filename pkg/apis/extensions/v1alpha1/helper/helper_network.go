// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package helper

import (
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
)

// DeterminePrimaryIPFamily determines the primary IP family out of a specified list of IP families.
func DeterminePrimaryIPFamily(ipFamilies []extensionsv1alpha1.IPFamily) extensionsv1alpha1.IPFamily {
	if len(ipFamilies) == 0 {
		return extensionsv1alpha1.IPFamilyIPv4
	}
	return ipFamilies[0]
}
