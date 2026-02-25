// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	"fmt"
	"strings"

	"github.com/go-test/deep"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/util/sets"
	utilvalidation "k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/gardener/gardener/pkg/api/core/validation"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
)

// ValidateSelfHostedShootExposure validates a SelfHostedShootExposure object.
func ValidateSelfHostedShootExposure(exposure *extensionsv1alpha1.SelfHostedShootExposure) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, apivalidation.ValidateObjectMeta(&exposure.ObjectMeta, true, apivalidation.NameIsDNSSubdomain, field.NewPath("metadata"))...)
	allErrs = append(allErrs, ValidateSelfHostedShootExposureSpec(&exposure.Spec, exposure.Namespace, field.NewPath("spec"))...)

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
func ValidateSelfHostedShootExposureSpec(spec *extensionsv1alpha1.SelfHostedShootExposureSpec, namespace string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(spec.Type) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("type"), "field is required"))
	}

	if spec.CredentialsRef != nil {
		allErrs = append(allErrs, validateCredentialsRef(*spec.CredentialsRef, namespace, fldPath.Child("credentialsRef"))...)
	}

	if errs := utilvalidation.IsValidPortNum(int(spec.Port)); len(errs) > 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("port"), spec.Port, strings.Join(errs, ",")))
	}

	if len(spec.Endpoints) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("endpoints"), "field is required"))
	}

	for i, ep := range spec.Endpoints {
		epPath := fldPath.Child("endpoints").Index(i)

		if len(ep.NodeName) == 0 {
			allErrs = append(allErrs, field.Required(epPath.Child("nodeName"), "field is required"))
		}

		for j, addr := range ep.Addresses {
			addrPath := epPath.Child("addresses").Index(j)

			if len(addr.Type) == 0 {
				allErrs = append(allErrs, field.Required(addrPath.Child("type"), "field is required"))
			} else {
				var supportedAddressTypes = sets.New(corev1.NodeHostName, corev1.NodeInternalIP, corev1.NodeExternalIP, corev1.NodeInternalDNS, corev1.NodeExternalDNS)
				if !supportedAddressTypes.Has(addr.Type) {
					allErrs = append(allErrs, field.NotSupported(addrPath.Child("type"), addr.Type, sets.List(supportedAddressTypes)))
				}
			}

			if len(addr.Address) == 0 {
				allErrs = append(allErrs, field.Required(addrPath.Child("address"), "field is required"))
			} else {
				switch addr.Type {
				case corev1.NodeInternalIP, corev1.NodeExternalIP:
					if errs := utilvalidation.IsValidIPForLegacyField(addrPath.Child("address"), addr.Address, false, nil); errs != nil {
						allErrs = append(allErrs, errs...)
					}
				case corev1.NodeHostName, corev1.NodeInternalDNS, corev1.NodeExternalDNS:
					allErrs = append(allErrs, validation.ValidateDNS1123Subdomain(addr.Address, addrPath.Child("address"))...)
				default:
					// If we reach this case, the address type is not supported, but this should have already been caught by the
					// validation of the address type. We still return an error for the address field to ensure we don't
					// accidentally allow random data when extending the list of supported address types in the future.
					allErrs = append(allErrs, field.Invalid(addrPath.Child("address"), addr.Address, fmt.Sprintf("invalid address for unsupported address type %q", addr.Type)))
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

// validateCredentialsRef validates the credentials reference of an extension object.
// For now, only the SelfHostedShootExposure resource has a credentialsRef field, other resources have a secretRef field
// instead.
func validateCredentialsRef(ref corev1.ObjectReference, namespace string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(ref.APIVersion) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("apiVersion"), "must provide an apiVersion"))
	}

	if len(ref.Kind) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("kind"), "must provide a kind"))
	}

	// For now, only local references are allowed. So namespace must equal the namespace of the extension object.
	if len(ref.Namespace) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("namespace"), "must provide a namespace"))
	} else if ref.Namespace != namespace {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("namespace"), ref.Namespace, "must equal metadata.namespace"))
	}

	if len(ref.Name) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("name"), "must provide a name"))
	} else {
		allErrs = append(allErrs, validation.ValidateDNS1123Subdomain(ref.Name, fldPath.Child("name"))...)
	}

	// For now, only Secret references are allowed in extension objects.
	var (
		secret = corev1.SchemeGroupVersion.WithKind("Secret")

		allowedGVKs = sets.New(secret)
		validGVKs   = []string{secret.String()}
	)

	if !allowedGVKs.Has(ref.GroupVersionKind()) {
		allErrs = append(allErrs, field.NotSupported(fldPath, ref.GroupVersionKind(), validGVKs))
	}

	return allErrs
}
