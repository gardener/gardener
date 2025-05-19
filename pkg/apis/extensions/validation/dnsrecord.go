// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	"fmt"
	"slices"
	"strings"

	"github.com/go-test/deep"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
)

// ValidateDNSRecord validates a DNSRecord object.
func ValidateDNSRecord(dns *extensionsv1alpha1.DNSRecord) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, apivalidation.ValidateObjectMeta(&dns.ObjectMeta, true, apivalidation.NameIsDNSSubdomain, field.NewPath("metadata"))...)
	allErrs = append(allErrs, ValidateDNSRecordSpec(&dns.Spec, field.NewPath("spec"))...)

	return allErrs
}

// ValidateDNSRecordUpdate validates a DNSRecord object before an update.
func ValidateDNSRecordUpdate(new, old *extensionsv1alpha1.DNSRecord) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateObjectMetaUpdate(&new.ObjectMeta, &old.ObjectMeta, field.NewPath("metadata"))...)
	allErrs = append(allErrs, ValidateDNSRecordSpecUpdate(&new.Spec, &old.Spec, new.DeletionTimestamp != nil, field.NewPath("spec"))...)
	allErrs = append(allErrs, ValidateDNSRecord(new)...)

	return allErrs
}

// ValidateDNSRecordSpec validates the specification of a DNSRecord object.
func ValidateDNSRecordSpec(spec *extensionsv1alpha1.DNSRecordSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(spec.Type) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("type"), "field is required"))
	}

	if len(spec.SecretRef.Name) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("secretRef", "name"), "field is required"))
	}

	if spec.Region != nil && len(*spec.Region) == 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("region"), *spec.Region, "field cannot be empty if specified"))
	}

	if spec.Zone != nil && len(*spec.Zone) == 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("zone"), *spec.Zone, "field cannot be empty if specified"))
	}

	// This will return FieldValueRequired for an empty spec.Name
	var nameToCheck string
	if spec.RecordType == extensionsv1alpha1.DNSRecordTypeTXT {
		// allow leading '_' as used for DNS challenges (e.g. Let's Encrypt)
		nameToCheck = strings.TrimPrefix(spec.Name, "_")
	} else {
		nameToCheck = strings.TrimPrefix(spec.Name, "*.")
	}
	allErrs = append(allErrs, validation.IsFullyQualifiedDomainName(fldPath.Child("name"), nameToCheck)...)

	validRecordTypes := []string{string(extensionsv1alpha1.DNSRecordTypeA), string(extensionsv1alpha1.DNSRecordTypeAAAA), string(extensionsv1alpha1.DNSRecordTypeCNAME), string(extensionsv1alpha1.DNSRecordTypeTXT)}
	if !slices.Contains(validRecordTypes, string(spec.RecordType)) {
		allErrs = append(allErrs, field.NotSupported(fldPath.Child("recordType"), spec.RecordType, validRecordTypes))
	}

	if len(spec.Values) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("values"), "field is required"))
	}
	if spec.RecordType == extensionsv1alpha1.DNSRecordTypeCNAME && len(spec.Values) > 1 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("values"), spec.Values, "CNAME records must have a single value"))
	}

	for i, value := range spec.Values {
		allErrs = append(allErrs, validateValue(spec.RecordType, value, fldPath.Child("values").Index(i))...)
	}

	if spec.TTL != nil {
		allErrs = append(allErrs, apivalidation.ValidateNonnegativeField(*spec.TTL, fldPath.Child("ttl"))...)
	}

	return allErrs
}

// ValidateDNSRecordSpecUpdate validates the spec of a DNSRecord object before an update.
func ValidateDNSRecordSpecUpdate(new, old *extensionsv1alpha1.DNSRecordSpec, deletionTimestampSet bool, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if deletionTimestampSet && !apiequality.Semantic.DeepEqual(new, old) {
		diff := deep.Equal(new, old)
		return field.ErrorList{field.Forbidden(fldPath, fmt.Sprintf("cannot update shoot spec if deletion timestamp is set. Requested changes: %s", strings.Join(diff, ",")))}
	}

	allErrs = append(allErrs, apivalidation.ValidateImmutableField(new.Type, old.Type, fldPath.Child("type"))...)
	allErrs = append(allErrs, apivalidation.ValidateImmutableField(new.Name, old.Name, fldPath.Child("name"))...)
	allErrs = append(allErrs, apivalidation.ValidateImmutableField(new.RecordType, old.RecordType, fldPath.Child("recordType"))...)

	return allErrs
}

func validateValue(recordType extensionsv1alpha1.DNSRecordType, value string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	switch recordType {
	case extensionsv1alpha1.DNSRecordTypeA:
		allErrs = append(allErrs, validation.IsValidIPv4Address(fldPath, value)...)
	case extensionsv1alpha1.DNSRecordTypeAAAA:
		allErrs = append(allErrs, validation.IsValidIPv6Address(fldPath, value)...)
	case extensionsv1alpha1.DNSRecordTypeCNAME:
		allErrs = append(allErrs, validation.IsFullyQualifiedDomainName(fldPath, value)...)
	}
	return allErrs
}
