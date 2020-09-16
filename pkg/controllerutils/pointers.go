// SPDX-FileCopyrightText: 2019 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controllerutils

// BoolPtrDerefOr dereferences the given bool if it's non-nil. Otherwise, returns the default.
func BoolPtrDerefOr(b *bool, defaultValue bool) bool {
	if b == nil {
		return defaultValue
	}
	return *b
}
