// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	"slices"
	"strings"

	"github.com/go-test/deep"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	extensionsv1alpha1helper "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1/helper"
	cidrvalidation "github.com/gardener/gardener/pkg/utils/validation/cidr"
)

// ValidateNetwork validates a Network object.
func ValidateNetwork(network *extensionsv1alpha1.Network) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, apivalidation.ValidateObjectMeta(&network.ObjectMeta, true, apivalidation.NameIsDNSSubdomain, field.NewPath("metadata"))...)
	allErrs = append(allErrs, ValidateNetworkSpec(&network.Spec, field.NewPath("spec"))...)

	return allErrs
}

// ValidateNetworkUpdate validates a Network object before an update.
func ValidateNetworkUpdate(new, old *extensionsv1alpha1.Network) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateObjectMetaUpdate(&new.ObjectMeta, &old.ObjectMeta, field.NewPath("metadata"))...)
	allErrs = append(allErrs, ValidateNetworkSpecUpdate(&new.Spec, &old.Spec, new.DeletionTimestamp != nil, field.NewPath("spec"))...)
	allErrs = append(allErrs, ValidateNetwork(new)...)

	return allErrs
}

// ValidateNetworkSpec validates the specification of a Network object.
func ValidateNetworkSpec(spec *extensionsv1alpha1.NetworkSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(spec.Type) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("type"), "field is required"))
	}

	if errs := ValidateIPFamilies(spec.IPFamilies, fldPath.Child("ipFamilies")); len(errs) > 0 {
		// further validation doesn't make any sense, because we don't know which IP family to check for in the CIDR fields
		return append(allErrs, errs...)
	}

	var (
		primaryIPFamily = extensionsv1alpha1helper.DeterminePrimaryIPFamily(spec.IPFamilies)
		cidrs           []cidrvalidation.CIDR
	)

	if len(spec.PodCIDR) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("podCIDR"), "field is required"))
	} else {
		cidrs = append(cidrs, cidrvalidation.NewCIDR(spec.PodCIDR, fldPath.Child("podCIDR")))
	}

	if len(spec.ServiceCIDR) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("serviceCIDR"), "field is required"))
	} else {
		cidrs = append(cidrs, cidrvalidation.NewCIDR(spec.ServiceCIDR, fldPath.Child("serviceCIDR")))
	}

	allErrs = append(allErrs, cidrvalidation.ValidateCIDRParse(cidrs...)...)
	allErrs = append(allErrs, cidrvalidation.ValidateCIDRIPFamily(cidrs, string(primaryIPFamily))...)

	if !(len(spec.IPFamilies) == 1 && spec.IPFamilies[0] == extensionsv1alpha1.IPFamilyIPv6) {
		allErrs = append(allErrs, cidrvalidation.ValidateCIDROverlap(cidrs, false)...)
	}
	return allErrs
}

// ValidateNetworkSpecUpdate validates the spec of a Network object before an update.
func ValidateNetworkSpecUpdate(new, old *extensionsv1alpha1.NetworkSpec, deletionTimestampSet bool, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if deletionTimestampSet && !apiequality.Semantic.DeepEqual(new, old) {
		if diff := deep.Equal(new, old); diff != nil {
			return field.ErrorList{field.Forbidden(fldPath, strings.Join(diff, ","))}
		}
		return apivalidation.ValidateImmutableField(new, old, fldPath)
	}

	allErrs = append(allErrs, apivalidation.ValidateImmutableField(new.Type, old.Type, fldPath.Child("type"))...)
	allErrs = append(allErrs, apivalidation.ValidateImmutableField(new.PodCIDR, old.PodCIDR, fldPath.Child("podCIDR"))...)
	allErrs = append(allErrs, apivalidation.ValidateImmutableField(new.ServiceCIDR, old.ServiceCIDR, fldPath.Child("serviceCIDR"))...)

	// allow upgrades from empty IPFamilies to the default of IPv4
	// the if condition can be removed once the network extension of all shoots have been updated
	// TODO: Remove in Gardener 1.87
	if !(old.IPFamilies == nil && slices.Equal(new.IPFamilies, []extensionsv1alpha1.IPFamily{extensionsv1alpha1.IPFamilyIPv4})) {
		allErrs = append(allErrs, apivalidation.ValidateImmutableField(new.IPFamilies, old.IPFamilies, fldPath.Child("ipFamilies"))...)
	}

	return allErrs
}

var availableIPFamilies = sets.New(
	string(extensionsv1alpha1.IPFamilyIPv4),
	string(extensionsv1alpha1.IPFamilyIPv6),
)

// ValidateIPFamilies validates the given list of IP families for valid values and combinations.
func ValidateIPFamilies(ipFamilies []extensionsv1alpha1.IPFamily, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	ipFamiliesSeen := sets.New[string]()
	for i, ipFamily := range ipFamilies {
		// validate: only supported IP families
		if !availableIPFamilies.Has(string(ipFamily)) {
			allErrs = append(allErrs, field.NotSupported(fldPath.Index(i), ipFamily, sets.List(availableIPFamilies)))
		}

		// validate: no duplicate IP families
		if ipFamiliesSeen.Has(string(ipFamily)) {
			allErrs = append(allErrs, field.Duplicate(fldPath.Index(i), ipFamily))
		} else {
			ipFamiliesSeen.Insert(string(ipFamily))
		}
	}

	return allErrs
}
