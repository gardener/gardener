// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation

// ShouldEnforceImmutability compares the given slices and returns if a immutability should be enforced.
// It mainly checks if the order of the same elements in `new` and `old` is the same, i.e. only an addition
// of elements to `new` is allowed.
func ShouldEnforceImmutability(new, old []string) bool {
	sizeDelta := len(new) - len(old)
	if sizeDelta > 0 {
		newA := new[:len(new)-sizeDelta]
		if equal(newA, old) {
			return false
		}

		return ShouldEnforceImmutability(newA, old)
	}
	return sizeDelta < 0 || sizeDelta == 0
}

func equal(new, old []string) bool {
	for i := range new {
		if new[i] != old[i] {
			return false
		}
	}
	return true
}
