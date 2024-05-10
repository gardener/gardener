// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	"fmt"
	"strings"
	"unicode"

	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"

	gardencorevalidation "github.com/gardener/gardener/pkg/apis/core/validation"
	"github.com/gardener/gardener/pkg/apis/security"
)

// GetSubClaimPrefixAndDelimiterFunc is func providing the prefix value for the 'sub' claim
// and the delimiter used to concatenate the various parts.
var GetSubClaimPrefixAndDelimiterFunc = func() (string, string) {
	return "gardener.cloud:workloadidentity", ":"
}

// ValidateWorkloadIdentity validates a WorkloadIdentity.
func ValidateWorkloadIdentity(workloadIdentity *security.WorkloadIdentity) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateObjectMeta(&workloadIdentity.ObjectMeta, true, gardencorevalidation.ValidateName, field.NewPath("metadata"))...)
	allErrs = append(allErrs, validateSpec(workloadIdentity.Spec, field.NewPath("spec"))...)
	allErrs = append(allErrs, validateSubClaim(*workloadIdentity, GetSubClaimPrefixAndDelimiterFunc)...)

	return allErrs
}

func validateSpec(spec security.WorkloadIdentitySpec, path *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, validateAudiences(spec.Audiences, path.Child("audiences"))...)
	allErrs = append(allErrs, validateTargetSystem(spec.TargetSystem, path.Child("targetSystem"))...)

	return allErrs
}

// validateAudiences validates a WorkloadIdentity audiences object.
func validateAudiences(audiences []string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(audiences) == 0 {
		allErrs = append(allErrs, field.Required(fldPath, "must provide at least one audience"))
	}

	for idx, aud := range audiences {
		if aud == "" {
			allErrs = append(allErrs, field.Required(fldPath.Index(idx), "must specify non-empty audience"))
		}
	}

	return allErrs
}

// validateTargetSystem validates a WorkloadIdentity TargetSystem object.
func validateTargetSystem(targetSystem security.TargetSystem, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(targetSystem.Type) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("type"), "must specify a provider type"))
	}

	if len(strings.Split(targetSystem.Type, ",")) > 1 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("type"), targetSystem.Type, "multiple providers specified"))
	}

	return allErrs
}

func validateSubClaim(obj security.WorkloadIdentity, getPrefixAndDelimiter func() (string, string)) field.ErrorList {
	allErrs := field.ErrorList{}
	prefix, delimiter := getPrefixAndDelimiter()
	subClaimValue := prefix + delimiter + obj.Namespace + delimiter + obj.Name + delimiter + string(obj.UID)
	for idx, r := range subClaimValue {
		if r > unicode.MaxASCII {
			allErrs = append(allErrs, field.Invalid(nil, subClaimValue, fmt.Sprintf("sub claim contains non-ascii symbol(%q) at index %d", r, idx)))
		}
	}

	if len(subClaimValue) > 255 {
		allErrs = append(allErrs, field.Invalid(nil, subClaimValue, "sub claim is too long, it is allowed to be no more than 255 ASCII characters"))
	}

	return allErrs
}

// ValidateWorkloadIdentityUpdate validates a WorkloadIdentity object before an update.
func ValidateWorkloadIdentityUpdate(newWorkloadIdentity, oldWorkloadIdentity *security.WorkloadIdentity) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateObjectMetaUpdate(&newWorkloadIdentity.ObjectMeta, &oldWorkloadIdentity.ObjectMeta, field.NewPath("metadata"))...)
	allErrs = append(allErrs, apivalidation.ValidateImmutableField(newWorkloadIdentity.Spec.TargetSystem.Type, oldWorkloadIdentity.Spec.TargetSystem.Type, field.NewPath("spec").Child("targetSystem").Child("type"))...)
	allErrs = append(allErrs, ValidateWorkloadIdentity(newWorkloadIdentity)...)

	return allErrs
}
