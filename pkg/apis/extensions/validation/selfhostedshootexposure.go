// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	"fmt"
	"net"
	"strings"

	"github.com/go-test/deep"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	utilvalidation "k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
)

// ValidateSelfHostedShootExposure validates a SelfHostedShootExposure object.
func ValidateSelfHostedShootExposure(exposure *extensionsv1alpha1.SelfHostedShootExposure) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, apivalidation.ValidateObjectMeta(&exposure.ObjectMeta, true, apivalidation.NameIsDNSSubdomain, field.NewPath("metadata"))...)
	allErrs = append(allErrs, ValidateSelfHostedShootExposureSpec(&exposure.Spec, field.NewPath("spec"))...)

	return allErrs
}

// ValidateSelfHostedShootExposureUpdate validates a SelfHostedShootExposure object before an update.
func ValidateSelfHostedShootExposureUpdate(new, old *extensionsv1alpha1.SelfHostedShootExposure) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateObjectMetaUpdate(&new.ObjectMeta, &old.ObjectMeta, field.NewPath("metadata"))...)
	allErrs = append(allErrs, ValidateSelfHostedShootExposureSpecUpdate(&new.Spec, &old.Spec, new.DeletionTimestamp != nil, field.NewPath("spec"))...)
	allErrs = append(allErrs, ValidateSelfHostedShootExposure(new)...)

	return allErrs
}

// ValidateSelfHostedShootExposureSpec validates the specification of a SelfHostedShootExposure object.
func ValidateSelfHostedShootExposureSpec(spec *extensionsv1alpha1.SelfHostedShootExposureSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(spec.Type) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("type"), "field is required"))
	}

	if len(spec.Endpoints) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("endpoints"), "field is required"))
	}

	for i, ep := range spec.Endpoints {
		epPath := fldPath.Child("endpoints").Index(i)

		if ep.Port <= 0 || ep.Port > 65535 {
			allErrs = append(allErrs, field.Invalid(epPath.Child("port"), ep.Port, "must be between 1 and 65535"))
		}

		for j, addr := range ep.Addresses {
			addrPath := epPath.Child("addresses").Index(j)

			if addr.Type != "" {
				switch addr.Type {
				case corev1.NodeHostName, corev1.NodeInternalIP, corev1.NodeExternalIP:
				default:
					allErrs = append(allErrs, field.Invalid(addrPath.Child("type"), addr.Type, "unknown node address type"))
				}
			}

			if len(addr.Address) > 0 {
				switch addr.Type {
				case corev1.NodeInternalIP, corev1.NodeExternalIP:
					if net.ParseIP(addr.Address) == nil {
						allErrs = append(allErrs, field.Invalid(addrPath.Child("address"), addr.Address, "invalid IP address"))
					}
				case corev1.NodeHostName:
					if errs := utilvalidation.IsDNS1123Subdomain(addr.Address); len(errs) != 0 {
						allErrs = append(allErrs, field.Invalid(addrPath.Child("address"), addr.Address, strings.Join(errs, ",")))
					}
				}
			}
		}
	}

	return allErrs
}

// ValidateSelfHostedShootExposureSpecUpdate validates the spec of an SelfHostedShootExposure object before an update.
func ValidateSelfHostedShootExposureSpecUpdate(new, old *extensionsv1alpha1.SelfHostedShootExposureSpec, deletionTimestampSet bool, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if deletionTimestampSet && !apiequality.Semantic.DeepEqual(new, old) {
		diff := deep.Equal(new, old)
		return field.ErrorList{field.Forbidden(fldPath, fmt.Sprintf("cannot update SelfHostedShootExposure spec if deletion timestamp is set. Requested changes: %s", strings.Join(diff, ",")))}
	}

	allErrs = append(allErrs, apivalidation.ValidateImmutableField(new.Type, old.Type, fldPath.Child("type"))...)

	return allErrs
}
