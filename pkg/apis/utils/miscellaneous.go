// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"iter"
)

// TransformElements takes a slice of elements and a transformation function and returns a sequence of transformed elements.
// This function is commonly used in combination with 'slices.Collect' to transform a slice of elements into another slice of transformed elements.
func TransformElements[S, T any](elements []S, transform func(S) T) iter.Seq[T] {
	return func(yield func(T) bool) {
		for _, element := range elements {
			transformedElement := transform(element)
			if !yield(transformedElement) {
				return
			}
		}
	}
}

// FilterElements takes a slice of elements and a match function and returns a sequence of filtered elements.
// This function is commonly used in combination with 'slices.Collect' to filter a slice of elements into another slice of filtered elements.
func FilterElements[S any](elements []S, match func(S) bool) iter.Seq[S] {
	return func(yield func(S) bool) {
		for _, element := range elements {
			if !match(element) {
				continue
			}
			if !yield(element) {
				return
			}
		}
	}
}
