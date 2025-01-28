// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
	componentbaseconfig "k8s.io/component-base/config"
	componentbaseconfigv1alpha1 "k8s.io/component-base/config/v1alpha1"
	componentbaseconfigvalidation "k8s.io/component-base/config/validation"

	admissioncontrollerconfigv1alpha1 "github.com/gardener/gardener/pkg/admissioncontroller/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/logger"
)

var configScheme = runtime.NewScheme()

func init() {
	schemeBuilder := runtime.NewSchemeBuilder(
		admissioncontrollerconfigv1alpha1.AddToScheme,
		componentbaseconfigv1alpha1.AddToScheme,
	)
	utilruntime.Must(schemeBuilder.AddToScheme(configScheme))
}

// ValidateAdmissionControllerConfiguration validates the given `AdmissionControllerConfiguration`.
func ValidateAdmissionControllerConfiguration(config *admissioncontrollerconfigv1alpha1.AdmissionControllerConfiguration) field.ErrorList {
	allErrs := field.ErrorList{}

	clientConnectionPath := field.NewPath("gardenClientConnection")
	internalClientConnectionConfig := &componentbaseconfig.ClientConnectionConfiguration{}
	if err := configScheme.Convert(&config.GardenClientConnection, internalClientConnectionConfig, nil); err != nil {
		allErrs = append(allErrs, field.InternalError(clientConnectionPath, err))
	} else {
		allErrs = append(allErrs, componentbaseconfigvalidation.ValidateClientConnectionConfiguration(internalClientConnectionConfig, clientConnectionPath)...)
	}

	if !sets.New(logger.AllLogLevels...).Has(config.LogLevel) {
		allErrs = append(allErrs, field.NotSupported(field.NewPath("logLevel"), config.LogLevel, logger.AllLogLevels))
	}

	if !sets.New(logger.AllLogFormats...).Has(config.LogFormat) {
		allErrs = append(allErrs, field.NotSupported(field.NewPath("logFormat"), config.LogFormat, logger.AllLogFormats))
	}

	serverPath := field.NewPath("server")
	if config.Server.ResourceAdmissionConfiguration != nil {
		allErrs = append(allErrs, ValidateResourceAdmissionConfiguration(config.Server.ResourceAdmissionConfiguration, serverPath.Child("resourceAdmissionConfiguration"))...)
	}

	return allErrs
}

// ValidateResourceAdmissionConfiguration validates the given `ResourceAdmissionConfiguration`.
func ValidateResourceAdmissionConfiguration(config *admissioncontrollerconfigv1alpha1.ResourceAdmissionConfiguration, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	validValues := sets.New(string(admissioncontrollerconfigv1alpha1.AdmissionModeBlock), string(admissioncontrollerconfigv1alpha1.AdmissionModeLog))

	if config.OperationMode != nil && !validValues.Has(string(*config.OperationMode)) {
		allErrs = append(allErrs, field.NotSupported(fldPath.Child("mode"), string(*config.OperationMode), validValues.UnsortedList()))
	}

	allowedSubjectKinds := sets.New(rbacv1.UserKind, rbacv1.GroupKind, rbacv1.ServiceAccountKind)

	for i, subject := range config.UnrestrictedSubjects {
		fld := fldPath.Child("unrestrictedSubjects").Index(i)

		if !allowedSubjectKinds.Has(subject.Kind) {
			allErrs = append(allErrs, field.NotSupported(fld.Child("kind"), subject.Kind, allowedSubjectKinds.UnsortedList()))
		}

		if subject.Name == "" {
			allErrs = append(allErrs, field.Invalid(fld.Child("name"), subject.Name, "name must not be empty"))
		}

		switch subject.Kind {
		case rbacv1.ServiceAccountKind:
			if subject.Namespace == "" {
				allErrs = append(allErrs, field.Invalid(fld.Child("namespace"), subject.Namespace, "name must not be empty"))
			}

			if subject.APIGroup != "" {
				allErrs = append(allErrs, field.Invalid(fld.Child("apiGroup"), subject.APIGroup, "apiGroup must be empty"))
			}
		case rbacv1.UserKind, rbacv1.GroupKind:
			if subject.Namespace != "" {
				allErrs = append(allErrs, field.Invalid(fld.Child("namespace"), subject.Namespace, "name must be empty"))
			}

			if subject.APIGroup != rbacv1.GroupName {
				allErrs = append(allErrs, field.NotSupported(fld.Child("apiGroup"), subject.APIGroup, []string{rbacv1.GroupName}))
			}
		}
	}

	for i, limit := range config.Limits {
		fld := fldPath.Child("limits").Index(i)
		hasResources := false

		for j, resource := range limit.Resources {
			hasResources = true

			if resource == "" {
				allErrs = append(allErrs, field.Invalid(fld.Child("resources").Index(j), resource, "must not be empty"))
			}
		}

		if !hasResources {
			allErrs = append(allErrs, field.Invalid(fld.Child("resources"), limit.Resources, "must at least have one element"))
		}

		if len(limit.APIGroups) < 1 {
			allErrs = append(allErrs, field.Invalid(fld.Child("apiGroups"), limit.Resources, "must at least have one element"))
		}

		hasVersions := false
		for j, version := range limit.APIVersions {
			hasVersions = true

			if version == "" {
				allErrs = append(allErrs, field.Invalid(fld.Child("versions").Index(j), version, "must not be empty"))
			}
		}

		if !hasVersions {
			allErrs = append(allErrs, field.Invalid(fld.Child("versions"), limit.Resources, "must at least have one element"))
		}

		if limit.Size.Cmp(resource.Quantity{}) < 0 {
			allErrs = append(allErrs, field.Invalid(fld.Child("size"), limit.Size.String(), "value must not be negative"))
		}
	}

	return allErrs
}
