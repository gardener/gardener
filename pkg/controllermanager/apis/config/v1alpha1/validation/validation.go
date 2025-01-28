// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	metav1validation "k8s.io/apimachinery/pkg/apis/meta/v1/validation"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
	componentbaseconfig "k8s.io/component-base/config"
	componentbaseconfigv1alpha1 "k8s.io/component-base/config/v1alpha1"
	componentbaseconfigvalidation "k8s.io/component-base/config/validation"

	controllermanagerconfigv1alpha1 "github.com/gardener/gardener/pkg/controllermanager/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/logger"
)

var configScheme = runtime.NewScheme()

func init() {
	schemeBuilder := runtime.NewSchemeBuilder(
		controllermanagerconfigv1alpha1.AddToScheme,
		componentbaseconfigv1alpha1.AddToScheme,
	)
	utilruntime.Must(schemeBuilder.AddToScheme(configScheme))
}

// ValidateControllerManagerConfiguration validates the given `ControllerManagerConfiguration`.
func ValidateControllerManagerConfiguration(conf *controllermanagerconfigv1alpha1.ControllerManagerConfiguration) field.ErrorList {
	allErrs := field.ErrorList{}

	clientConnectionPath := field.NewPath("gardenClientConnection")
	internalClientConnectionConfig := &componentbaseconfig.ClientConnectionConfiguration{}
	if err := configScheme.Convert(&conf.GardenClientConnection, internalClientConnectionConfig, nil); err != nil {
		allErrs = append(allErrs, field.InternalError(clientConnectionPath, err))
	} else {
		allErrs = append(allErrs, componentbaseconfigvalidation.ValidateClientConnectionConfiguration(internalClientConnectionConfig, clientConnectionPath)...)
	}

	if conf.LeaderElection != nil {
		leaderElectionPath := field.NewPath("leaderElection")
		internalLeaderElectionConfig := &componentbaseconfig.LeaderElectionConfiguration{}
		if err := configScheme.Convert(conf.LeaderElection, internalLeaderElectionConfig, nil); err != nil {
			allErrs = append(allErrs, field.InternalError(leaderElectionPath, err))
		} else {
			allErrs = append(allErrs, componentbaseconfigvalidation.ValidateLeaderElectionConfiguration(internalLeaderElectionConfig, leaderElectionPath)...)
		}
	}

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
