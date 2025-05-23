// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/gardener/gardener/pkg/apis/core/validation"
	"github.com/gardener/gardener/plugin/pkg/shoot/tolerationrestriction/apis/shoottolerationrestriction"
)

// ValidateConfiguration validates the configuration.
func ValidateConfiguration(config *shoottolerationrestriction.Configuration) field.ErrorList {
	var allErrs field.ErrorList

	allErrs = append(allErrs, validation.ValidateTolerations(config.Defaults, field.NewPath("defaults"))...)
	allErrs = append(allErrs, validation.ValidateTolerations(config.Whitelist, field.NewPath("whitelist"))...)

	return allErrs
}
