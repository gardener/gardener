// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	"fmt"
	"strings"
	"unicode"

	"k8s.io/apimachinery/pkg/util/validation/field"
)

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

const allowedSpecialCharacters = ".,:-_"

// ValidateFreeFormText checks that the given text contains only letters, digits, spaces, or punctuation characters.
// It uses the unicode package to determine character types, allowing a wide range of characters from various languages.
func ValidateFreeFormText(text string, fldPath *field.Path) field.ErrorList {
	var (
		allErrs           = field.ErrorList{}
		invalidCharacters []rune
	)

	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsSpace(r) || strings.ContainsRune(allowedSpecialCharacters, r) {
			continue
		} else {
			invalidCharacters = append(invalidCharacters, r)
		}
	}

	if len(invalidCharacters) > 0 {
		allErrs = append(allErrs, field.Forbidden(fldPath, fmt.Sprintf("must not contain invalid character: %q", invalidCharacters)))
	}

	return allErrs
}
