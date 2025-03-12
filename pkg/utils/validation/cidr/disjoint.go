// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package cidr

import (
	"fmt"
	"k8s.io/apimachinery/pkg/util/validation/field"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
)

// ValidateNetworkDisjointedness validates that the given <seedNetworks> and <k8sNetworks> are disjoint.
func ValidateNetworkDisjointedness(fldPath *field.Path, shootNodes, shootPods, shootServices, seedNodes *string, seedPods, seedServices string, workerless, allowOverlap bool) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, validateOverlapWithSeedWrapper(fldPath.Child("nodes"), shootNodes, "node", false, allowOverlap, seedNodes, seedPods, seedServices)...)
	allErrs = append(allErrs, validateOverlapWithSeedWrapper(fldPath.Child("services"), shootServices, "service", true, allowOverlap, seedNodes, seedPods, seedServices)...)
	allErrs = append(allErrs, validateOverlapWithSeedWrapper(fldPath.Child("pods"), shootPods, "pod", !workerless, allowOverlap, seedNodes, seedPods, seedServices)...)

	return allErrs
}

// ValidateMultiNetworkDisjointedness validates that the given <seedNetworks> and <k8sNetworks> are disjoint.
func ValidateMultiNetworkDisjointedness(fldPath *field.Path, shootNodes, shootPods, shootServices []string, seedNodes *string, seedPods, seedServices string, workerless, allowOverlap bool) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, validateOverlapWithSeed(fldPath.Child("nodes"), shootNodes, "node", false, allowOverlap, seedNodes, seedPods, seedServices)...)
	allErrs = append(allErrs, validateOverlapWithSeed(fldPath.Child("services"), shootServices, "service", true, allowOverlap, seedNodes, seedPods, seedServices)...)
	allErrs = append(allErrs, validateOverlapWithSeed(fldPath.Child("pods"), shootPods, "pod", !workerless, allowOverlap, seedNodes, seedPods, seedServices)...)

	return allErrs
}

func validateOverlapWithSeedWrapper(fldPath *field.Path, shootNetwork *string, networkType string, networkRequired, allowOverlap bool, seedNodes *string, seedPods, seedServices string) field.ErrorList {
	var network []string
	if shootNetwork != nil {
		network = append(network, *shootNetwork)
	}
	return validateOverlapWithSeed(fldPath, network, networkType, networkRequired, allowOverlap, seedNodes, seedPods, seedServices)
}

func validateOverlapWithSeed(fldPath *field.Path, shootNetwork []string, networkType string, networkRequired, allowOverlap bool, seedNodes *string, seedPods, seedServices string) field.ErrorList {
	allErrs := field.ErrorList{}

	for _, network := range shootNetwork {
		// we allow overlapping with seed networks for non-haVPN, IPv4 shoots
		if !allowOverlap || NewCIDR(network, fldPath).IsIPv6() {
			if NetworksIntersect(seedServices, network) {
				allErrs = append(allErrs, field.Invalid(fldPath, network, fmt.Sprintf("shoot %s network intersects with seed service network", networkType)))
			}

			if NetworksIntersect(seedPods, network) {
				allErrs = append(allErrs, field.Invalid(fldPath, network, fmt.Sprintf("shoot %s network intersects with seed pod network", networkType)))
			}

			if seedNodes != nil && NetworksIntersect(*seedNodes, network) {
				allErrs = append(allErrs, field.Invalid(fldPath, network, fmt.Sprintf("shoot %s network intersects with seed node network", networkType)))
			}
		}

		if NetworksIntersect(v1beta1constants.DefaultVPNRangeV6, network) {
			allErrs = append(allErrs, field.Invalid(fldPath, network, fmt.Sprintf("shoot %s network intersects with default vpn network (%s)", networkType, v1beta1constants.DefaultVPNRangeV6)))
		}

		if NetworksIntersect(v1beta1constants.ReservedKubeApiServerMappingRange, network) {
			allErrs = append(allErrs, field.Invalid(fldPath, network, fmt.Sprintf("shoot %s network intersects with reserved kube-apiserver mapping range (%s)", networkType, v1beta1constants.ReservedKubeApiServerMappingRange)))
		}

		if NetworksIntersect(v1beta1constants.ReservedSeedPodNetworkMappedRange, network) {
			allErrs = append(allErrs, field.Invalid(fldPath, network, fmt.Sprintf("shoot %s network intersects with reserved seed pod network mapping range (%s)", networkType, v1beta1constants.ReservedSeedPodNetworkMappedRange)))
		}

		if NetworksIntersect(v1beta1constants.ReservedShootNodeNetworkMappedRange, network) {
			allErrs = append(allErrs, field.Invalid(fldPath, network, fmt.Sprintf("shoot %s network intersects with reserved shoot node network mapping range (%s)", networkType, v1beta1constants.ReservedShootNodeNetworkMappedRange)))
		}

		if NetworksIntersect(v1beta1constants.ReservedShootServiceNetworkMappedRange, network) {
			allErrs = append(allErrs, field.Invalid(fldPath, network, fmt.Sprintf("shoot %s network intersects with reserved shoot service network mapping range (%s)", networkType, v1beta1constants.ReservedShootServiceNetworkMappedRange)))
		}

		if NetworksIntersect(v1beta1constants.ReservedShootPodNetworkMappedRange, network) {
			allErrs = append(allErrs, field.Invalid(fldPath, network, fmt.Sprintf("shoot %s network intersects with reserved shoot pod network mapping range (%s)", networkType, v1beta1constants.ReservedShootPodNetworkMappedRange)))
		}

	}
	if len(shootNetwork) == 0 && networkRequired {
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
