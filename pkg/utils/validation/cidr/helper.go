// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
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

// ValidateCIDRIPFamily validates that all the given CIDRs can be matched to the correct IP Family.
func ValidateCIDRIPFamily(cidrs []CIDR, ipFamily string) field.ErrorList {
	allErrs := field.ErrorList{}

	for _, cidr := range cidrs {
		if cidr == nil {
			continue
		}

		allErrs = append(allErrs, cidr.ValidateIPFamily(ipFamily)...)
	}

	return allErrs
}

// ValidateCIDROverlap validates that the provided CIDRs do not overlap.
func ValidateCIDROverlap(paths []CIDR, overlap bool) field.ErrorList {
	allErrs := field.ErrorList{}

	for i := 0; i < len(paths)-1; i++ {
		if paths[i] == nil {
			continue
		}

		if overlap {
			allErrs = append(allErrs, paths[i].ValidateOverlap(paths[i+1:]...)...)
		} else {
			allErrs = append(allErrs, paths[i].ValidateNotOverlap(paths[i+1:]...)...)
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
	ipAddress, ipNet, _ := net.ParseCIDR(cidrToValidate)
	if ipNet != nil {
		if !ipNet.IP.Equal(ipAddress) {
			allErrs = append(allErrs, field.Invalid(fldPath, cidrToValidate, "must be valid canonical CIDR"))
		}
	}
	return allErrs
}
