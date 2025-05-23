// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	"fmt"
	"strings"
	"unicode"

	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"

	gardencorevalidation "github.com/gardener/gardener/pkg/apis/core/validation"
	"github.com/gardener/gardener/pkg/apis/security"
)

// GetSubClaimPrefixAndDelimiterFunc is func providing the prefix value for the 'sub' claim
// and the delimiter used to concatenate the various parts.
var GetSubClaimPrefixAndDelimiterFunc = func() (string, string) {
	delimiter := ":"
	return strings.Join([]string{"gardener.cloud", "workloadidentity"}, delimiter), delimiter
}

// ValidateWorkloadIdentity validates a WorkloadIdentity.
func ValidateWorkloadIdentity(workloadIdentity *security.WorkloadIdentity) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateObjectMeta(&workloadIdentity.ObjectMeta, true, gardencorevalidation.ValidateName, field.NewPath("metadata"))...)
	allErrs = append(allErrs, validateSpec(workloadIdentity.Spec, field.NewPath("spec"))...)
	allErrs = append(allErrs, validateSubClaim(*workloadIdentity, GetSubClaimPrefixAndDelimiterFunc, field.NewPath("status").Child("sub"))...)

	return allErrs
}

func validateSpec(spec security.WorkloadIdentitySpec, path *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, validateAudiences(spec.Audiences, path.Child("audiences"))...)
	allErrs = append(allErrs, validateTargetSystem(spec.TargetSystem, path.Child("targetSystem"))...)

	return allErrs
}

// validateAudiences validates a WorkloadIdentity audiences list.
func validateAudiences(audiences []string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(audiences) == 0 {
		allErrs = append(allErrs, field.Required(fldPath, "must provide at least one audience"))
	}

	duplicatedAudiences := sets.Set[string]{}
	for idx, aud := range audiences {
		if aud == "" {
			allErrs = append(allErrs, field.Required(fldPath.Index(idx), "must specify non-empty audience"))
		}
		if duplicatedAudiences.Has(aud) {
			allErrs = append(allErrs, field.Duplicate(fldPath.Index(idx), aud))
		} else {
			duplicatedAudiences.Insert(aud)
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

func validateSubClaim(obj security.WorkloadIdentity, getPrefixAndDelimiter func() (string, string), fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if len(obj.Status.Sub) == 0 {
		allErrs = append(allErrs, field.Required(fldPath, "must specify sub claim value"))
	}

	prefix, delimiter := getPrefixAndDelimiter()
	expectedSubClaimValue := strings.Join([]string{prefix, obj.Namespace, obj.Name, string(obj.UID)}, delimiter)
	if expectedSubClaimValue != obj.Status.Sub {
		allErrs = append(allErrs, field.Invalid(fldPath, obj.Status.Sub, fmt.Sprintf("sub claim does not match expected value: %q", expectedSubClaimValue)))
	}

	for idx, r := range expectedSubClaimValue {
		if r > unicode.MaxASCII {
			allErrs = append(allErrs, field.Invalid(fldPath, expectedSubClaimValue, fmt.Sprintf("sub claim contains non-ascii symbol(%q) at index %d", r, idx)))
		}
	}

	if len(expectedSubClaimValue) > 255 {
		allErrs = append(allErrs, field.Invalid(fldPath, expectedSubClaimValue, fmt.Sprintf("sub claim is too long (%d), it is allowed to be no more than 255 ASCII characters", len(expectedSubClaimValue))))
	}

	return allErrs
}

// ValidateWorkloadIdentityUpdate validates a WorkloadIdentity object before an update.
func ValidateWorkloadIdentityUpdate(newWorkloadIdentity, oldWorkloadIdentity *security.WorkloadIdentity) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateObjectMetaUpdate(&newWorkloadIdentity.ObjectMeta, &oldWorkloadIdentity.ObjectMeta, field.NewPath("metadata"))...)
	allErrs = append(allErrs, apivalidation.ValidateImmutableField(newWorkloadIdentity.Spec.TargetSystem.Type, oldWorkloadIdentity.Spec.TargetSystem.Type, field.NewPath("spec").Child("targetSystem").Child("type"))...)
	allErrs = append(allErrs, ValidateWorkloadIdentity(newWorkloadIdentity)...)
	if len(oldWorkloadIdentity.Status.Sub) != 0 {
		allErrs = append(allErrs, apivalidation.ValidateImmutableField(newWorkloadIdentity.Status.Sub, oldWorkloadIdentity.Status.Sub, field.NewPath("status").Child("sub"))...)
	}

	return allErrs
}
