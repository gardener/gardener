// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/gardener/gardener/pkg/apis/core/validation"
	"github.com/gardener/gardener/plugin/pkg/shoot/dnsrewriting/apis/shootdnsrewriting"
)

// ValidateConfiguration validates the configuration.
func ValidateConfiguration(config *shootdnsrewriting.Configuration) field.ErrorList {
	var allErrs field.ErrorList

	if config == nil {
		return allErrs
	}

	allErrs = append(allErrs, validation.ValidateCoreDNSRewritingCommonSuffixes(config.CommonSuffixes, nil)...)

	return allErrs
}
