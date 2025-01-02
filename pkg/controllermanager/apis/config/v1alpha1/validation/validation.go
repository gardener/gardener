// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	metav1validation "k8s.io/apimachinery/pkg/apis/meta/v1/validation"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"

	controllermanagerconfigv1alpha1 "github.com/gardener/gardener/pkg/controllermanager/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/logger"
)

// ValidateControllerManagerConfiguration validates the given `ControllerManagerConfiguration`.
func ValidateControllerManagerConfiguration(conf *controllermanagerconfigv1alpha1.ControllerManagerConfiguration) field.ErrorList {
	allErrs := field.ErrorList{}

	if conf.LogLevel != "" {
		if !sets.New(logger.AllLogLevels...).Has(conf.LogLevel) {
			allErrs = append(allErrs, field.NotSupported(field.NewPath("logLevel"), conf.LogLevel, logger.AllLogLevels))
		}
	}

	if conf.LogFormat != "" {
		if !sets.New(logger.AllLogFormats...).Has(conf.LogFormat) {
			allErrs = append(allErrs, field.NotSupported(field.NewPath("logFormat"), conf.LogFormat, logger.AllLogFormats))
		}
	}

	allErrs = append(allErrs, validateControllerManagerControllerConfiguration(conf.Controllers, field.NewPath("controllers"))...)
	return allErrs
}

func validateControllerManagerControllerConfiguration(conf controllermanagerconfigv1alpha1.ControllerManagerControllerConfiguration, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	projectFldPath := fldPath.Child("project")
	if conf.Project != nil {
		allErrs = append(allErrs, validateProjectControllerConfiguration(conf.Project, projectFldPath)...)
	}

	return allErrs
}

func validateProjectControllerConfiguration(conf *controllermanagerconfigv1alpha1.ProjectControllerConfiguration, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	for i, quotaConfig := range conf.Quotas {
		allErrs = append(allErrs, validateProjectQuotaConfiguration(quotaConfig, fldPath.Child("quotas").Index(i))...)
	}
	return allErrs
}

func validateProjectQuotaConfiguration(conf controllermanagerconfigv1alpha1.QuotaConfiguration, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, metav1validation.ValidateLabelSelector(conf.ProjectSelector, metav1validation.LabelSelectorValidationOptions{AllowInvalidLabelValueInSelector: true}, fldPath.Child("projectSelector"))...)

	return allErrs
}
