// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	"fmt"
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
	// For dualstack, primaryIPFamily might not match configured CIDRs.
	if len(spec.IPFamilies) < 2 {
		allErrs = append(allErrs, cidrvalidation.ValidateCIDRIPFamily(cidrs, string(primaryIPFamily))...)
	}

	if len(spec.IPFamilies) != 1 || spec.IPFamilies[0] != extensionsv1alpha1.IPFamilyIPv6 {
		allErrs = append(allErrs, cidrvalidation.ValidateCIDROverlap(cidrs, false)...)
	}
	return allErrs
}

// ValidateNetworkSpecUpdate validates the spec of a Network object before an update.
func ValidateNetworkSpecUpdate(new, old *extensionsv1alpha1.NetworkSpec, deletionTimestampSet bool, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if deletionTimestampSet && !apiequality.Semantic.DeepEqual(new, old) {
		diff := deep.Equal(new, old)
		return field.ErrorList{field.Forbidden(fldPath, fmt.Sprintf("cannot update network spec if deletion timestamp is set. Requested changes: %s", strings.Join(diff, ",")))}
	}

	allErrs = append(allErrs, ValidateIPFamiliesUpdate(new.IPFamilies, old.IPFamilies, fldPath.Child("ipFamilies"))...)
	allErrs = append(allErrs, apivalidation.ValidateImmutableField(new.Type, old.Type, fldPath.Child("type"))...)
	allErrs = append(allErrs, apivalidation.ValidateImmutableField(new.PodCIDR, old.PodCIDR, fldPath.Child("podCIDR"))...)
	allErrs = append(allErrs, apivalidation.ValidateImmutableField(new.ServiceCIDR, old.ServiceCIDR, fldPath.Child("serviceCIDR"))...)
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

	if len(allErrs) > 0 {
		// further validation doesn't make any sense, because there are unsupported or duplicate IP families
		return allErrs
	}

	return allErrs
}

// ValidateIPFamiliesUpdate validates the update of IP families to ensure only allowed transitions.
func ValidateIPFamiliesUpdate(newIPFamilies, oldIPFamilies []extensionsv1alpha1.IPFamily, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(oldIPFamilies) == 1 &&
		((oldIPFamilies[0] == extensionsv1alpha1.IPFamilyIPv6 && len(newIPFamilies) == 2 && newIPFamilies[0] == extensionsv1alpha1.IPFamilyIPv6 && newIPFamilies[1] == extensionsv1alpha1.IPFamilyIPv4) ||
			(oldIPFamilies[0] == extensionsv1alpha1.IPFamilyIPv4 && len(newIPFamilies) == 2 && newIPFamilies[0] == extensionsv1alpha1.IPFamilyIPv4 && newIPFamilies[1] == extensionsv1alpha1.IPFamilyIPv6)) {
		// Allow transition from [IPv6] to [IPv6, IPv4] or [IPv4] to [IPv4, IPv6]
		return allErrs
	}

	if !apiequality.Semantic.DeepEqual(newIPFamilies, oldIPFamilies) {
		allErrs = append(allErrs, field.Forbidden(fldPath,
			fmt.Sprintf("unsupported IP family update: oldIPFamilies=%v, newIPFamilies=%v", oldIPFamilies, newIPFamilies)))
	}

	return allErrs
}
