// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
	ipAdress, ipNet, _ := net.ParseCIDR(cidrToValidate)
	if ipNet != nil {
		if !ipNet.IP.Equal(ipAdress) {
			allErrs = append(allErrs, field.Invalid(fldPath, cidrToValidate, "must be valid canonical CIDR"))
		}
	}
	return allErrs
}
