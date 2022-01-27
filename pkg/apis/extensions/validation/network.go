// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package validation

import (
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	cidrvalidation "github.com/gardener/gardener/pkg/utils/validation/cidr"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
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

	var cidrs []cidrvalidation.CIDR

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
	allErrs = append(allErrs, cidrvalidation.ValidateCIDROverlap(cidrs, false)...)

	return allErrs
}

// ValidateNetworkSpecUpdate validates the spec of a Network object before an update.
func ValidateNetworkSpecUpdate(new, old *extensionsv1alpha1.NetworkSpec, deletionTimestampSet bool, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if deletionTimestampSet && !apiequality.Semantic.DeepEqual(new, old) {
		allErrs = append(allErrs, apivalidation.ValidateImmutableField(new, old, fldPath)...)
		return allErrs
	}

	allErrs = append(allErrs, apivalidation.ValidateImmutableField(new.Type, old.Type, fldPath.Child("type"))...)
	allErrs = append(allErrs, apivalidation.ValidateImmutableField(new.PodCIDR, old.PodCIDR, fldPath.Child("podCIDR"))...)
	allErrs = append(allErrs, apivalidation.ValidateImmutableField(new.ServiceCIDR, old.ServiceCIDR, fldPath.Child("serviceCIDR"))...)

	return allErrs
}

// ValidateNetworkStatus validates the status of a Network object.
func ValidateNetworkStatus(status *extensionsv1alpha1.NetworkStatus, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	return allErrs
}

// ValidateNetworkStatusUpdate validates the status field of a Network object before an update.
func ValidateNetworkStatusUpdate(newStatus, oldStatus *extensionsv1alpha1.NetworkStatus, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	return allErrs
}
