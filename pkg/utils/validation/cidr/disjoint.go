// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package cidr

import (
	"fmt"

	"k8s.io/apimachinery/pkg/util/validation/field"
)

// ValidateNetworkDisjointedness validates that the given <seedNetworks> and <k8sNetworks> are disjoint.
func ValidateNetworkDisjointedness(fldPath *field.Path, shootNodes, shootPods, shootServices, seedNodes, seedVPN *string, seedPods, seedServices string, workerless bool) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, validateOverlapWithSeed(fldPath.Child("nodes"), shootNodes, "node", false, seedNodes, seedVPN, seedPods, seedServices)...)
	allErrs = append(allErrs, validateOverlapWithSeed(fldPath.Child("services"), shootServices, "service", true, seedNodes, seedVPN, seedPods, seedServices)...)
	allErrs = append(allErrs, validateOverlapWithSeed(fldPath.Child("pods"), shootPods, "pod", !workerless, seedNodes, seedVPN, seedPods, seedServices)...)

	return allErrs
}

func validateOverlapWithSeed(fldPath *field.Path, shootNetwork *string, networkType string, networkRequired bool, seedNodes, seedVPN *string, seedPods, seedServices string) field.ErrorList {
	allErrs := field.ErrorList{}

	if shootNetwork != nil {
		if NetworksIntersect(seedServices, *shootNetwork) {
			allErrs = append(allErrs, field.Invalid(fldPath, *shootNetwork, fmt.Sprintf("shoot %s network intersects with seed service network", networkType)))
		}

		if NetworksIntersect(seedPods, *shootNetwork) {
			allErrs = append(allErrs, field.Invalid(fldPath, *shootNetwork, fmt.Sprintf("shoot %s network intersects with seed pod network", networkType)))
		}

		if seedNodes != nil && NetworksIntersect(*seedNodes, *shootNetwork) {
			allErrs = append(allErrs, field.Invalid(fldPath, *shootNetwork, fmt.Sprintf("shoot %s network intersects with seed node network", networkType)))
		}

		if seedVPN != nil && NetworksIntersect(*seedVPN, *shootNetwork) {
			allErrs = append(allErrs, field.Invalid(fldPath, *shootNetwork, fmt.Sprintf("shoot %s network intersects with seed VPN network", networkType)))
		}
	} else if networkRequired {
		allErrs = append(allErrs, field.Required(fldPath, networkType+"s is required"))
	}

	return allErrs
}

// ValidateShootNetworkDisjointedness validates that the given shoot network is disjoint.
func ValidateShootNetworkDisjointedness(fldPath *field.Path, shootNodes, shootPods, shootServices *string, workerless bool) field.ErrorList {
	var (
		allErrs = field.ErrorList{}

		pathServices = fldPath.Child("services")
		pathPods     = fldPath.Child("pods")
	)

	if shootPods != nil && shootServices != nil {
		if NetworksIntersect(*shootPods, *shootServices) {
			allErrs = append(allErrs, field.Invalid(pathServices, *shootServices, "shoot pod network intersects with shoot service network"))
		}

		if shootNodes != nil && NetworksIntersect(*shootPods, *shootNodes) {
			allErrs = append(allErrs, field.Invalid(pathPods, *shootPods, "shoot pod network intersects with shoot node network"))
		}

		if shootNodes != nil && NetworksIntersect(*shootServices, *shootNodes) {
			allErrs = append(allErrs, field.Invalid(pathServices, *shootServices, "shoot service network intersects with shoot node network"))
		}
	} else if shootPods != nil {
		if shootNodes != nil && NetworksIntersect(*shootPods, *shootNodes) {
			allErrs = append(allErrs, field.Invalid(pathPods, *shootPods, "shoot pod network intersects with shoot node network"))
		}

		allErrs = append(allErrs, field.Required(pathServices, "shoot service network is required"))
	} else if shootServices != nil {
		if shootNodes != nil && NetworksIntersect(*shootServices, *shootNodes) {
			allErrs = append(allErrs, field.Invalid(pathServices, *shootServices, "shoot service network intersects with shoot node network"))
		}

		if !workerless {
			allErrs = append(allErrs, field.Required(pathPods, "shoot pod network is required"))
		}
	} else {
		if !workerless {
			allErrs = append(allErrs, field.Required(pathPods, "shoot pod network is required"))
		}

		allErrs = append(allErrs, field.Required(pathServices, "shoot service network is required"))
	}

	return allErrs
}

// NetworksIntersect returns true if the given network CIDRs intersect.
func NetworksIntersect(cidr1, cidr2 string) bool {
	c1 := NewCIDR(cidr1, field.NewPath(""))
	c2 := NewCIDR(cidr2, field.NewPath(""))
	return c1.ValidateOverlap(c2).ToAggregate() == nil
}
