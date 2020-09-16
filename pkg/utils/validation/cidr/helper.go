// SPDX-FileCopyrightText: 2018 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package cidr

import (
	"net"

	"k8s.io/apimachinery/pkg/util/validation/field"
)

// ValidateCIDRParse validates that all the given CIDRs can be parsed successfully.
func ValidateCIDRParse(cidrPaths ...CIDR) (allErrs field.ErrorList) {
	for _, cidrPath := range cidrPaths {
		if cidrPath == nil {
			continue
		}
		allErrs = append(allErrs, cidrPath.ValidateParse()...)
	}
	return allErrs
}

// ValidateCIDROverlap validates that the provided CIDRs do not overlap.
func ValidateCIDROverlap(leftPaths, rightPaths []CIDR, overlap bool) (allErrs field.ErrorList) {
	for _, left := range leftPaths {
		if left == nil {
			continue
		}
		if overlap {
			allErrs = append(allErrs, left.ValidateSubset(rightPaths...)...)
		} else {
			allErrs = append(allErrs, left.ValidateNotSubset(rightPaths...)...)
		}
	}
	return allErrs
}

// ValidateCIDRIsCanonical validates that the provided CIDR is in canonical form.
func ValidateCIDRIsCanonical(fldPath *field.Path, cidrToValidate string) field.ErrorList {
	allErrs := field.ErrorList{}
	if len(cidrToValidate) == 0 {
		return allErrs
	}
	// CIDR parse error already validated
	ipAdress, ipNet, _ := net.ParseCIDR(string(cidrToValidate))
	if ipNet != nil {
		if !ipNet.IP.Equal(ipAdress) {
			allErrs = append(allErrs, field.Invalid(fldPath, cidrToValidate, "must be valid canonical CIDR"))
		}
	}
	return allErrs
}
