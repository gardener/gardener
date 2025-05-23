// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package utils

// MinGreaterThanZero returns the minimum of the given two integers that is greater than 0. If both integers are less
// than or equal to 0, it returns 0.
// I.e., it works like the min builtin function but any value less than or equal to 0 is ignored.
func MinGreaterThanZero[T int | int8 | int16 | int32 | int64](a, b T) T {
	if a <= 0 || b <= 0 {
		// if one of both is <= 0, return the other one
		// if bother are <=, return 0
		return max(a, b, 0)
	}

	return min(a, b)
}
