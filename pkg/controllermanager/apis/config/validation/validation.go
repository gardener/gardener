// Copyright 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	metav1validation "k8s.io/apimachinery/pkg/apis/meta/v1/validation"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/gardener/gardener/pkg/controllermanager/apis/config"
	"github.com/gardener/gardener/pkg/logger"
)

// ValidateControllerManagerConfiguration validates the given `ControllerManagerConfiguration`.
func ValidateControllerManagerConfiguration(conf *config.ControllerManagerConfiguration) field.ErrorList {
	allErrs := field.ErrorList{}

	if conf.LogLevel != "" {
		if !sets.New[string](logger.AllLogLevels...).Has(conf.LogLevel) {
			allErrs = append(allErrs, field.NotSupported(field.NewPath("logLevel"), conf.LogLevel, logger.AllLogLevels))
		}
	}

	if conf.LogFormat != "" {
		if !sets.New[string](logger.AllLogFormats...).Has(conf.LogFormat) {
			allErrs = append(allErrs, field.NotSupported(field.NewPath("logFormat"), conf.LogFormat, logger.AllLogFormats))
		}
	}

	allErrs = append(allErrs, validateControllerManagerControllerConfiguration(conf.Controllers, field.NewPath("controllers"))...)
	return allErrs
}

func validateControllerManagerControllerConfiguration(conf config.ControllerManagerControllerConfiguration, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	bastionFldPath := fldPath.Child("bastion")
	if conf.Bastion != nil {
		allErrs = append(allErrs, validateBastionControllerConfiguration(conf.Bastion, bastionFldPath)...)
	}

	cloudProfileFldPath := fldPath.Child("cloudProfile")
	if conf.CloudProfile != nil {
		allErrs = append(allErrs, validateCloudProfileControllerConfiguration(conf.CloudProfile, cloudProfileFldPath)...)
	}

	controllerDeploymentFldPath := fldPath.Child("controllerDeployment")
	if conf.ControllerDeployment != nil {
		allErrs = append(allErrs, validateControllerDeploymentControllerConfiguration(conf.ControllerDeployment, controllerDeploymentFldPath)...)
	}

	controllerRegistrationFldPath := fldPath.Child("controllerRegistration")
	if conf.ControllerRegistration != nil {
		allErrs = append(allErrs, validateControllerRegistrationControllerConfiguration(conf.ControllerRegistration, controllerRegistrationFldPath)...)
	}

	eventFldPath := fldPath.Child("event")
	if conf.Event != nil {
		allErrs = append(allErrs, validateEventControllerConfiguration(conf.Event, eventFldPath)...)
	}

	exposureClassFldPath := fldPath.Child("exposureClass")
	if conf.ExposureClass != nil {
		allErrs = append(allErrs, validateExposureClassControllerConfiguration(conf.ExposureClass, exposureClassFldPath)...)
	}

	projectFldPath := fldPath.Child("project")
	if conf.Project != nil {
		allErrs = append(allErrs, validateProjectControllerConfiguration(conf.Project, projectFldPath)...)
	}

	quotaFldPath := fldPath.Child("quota")
	if conf.Quota != nil {
		allErrs = append(allErrs, validateQuotaControllerConfiguration(conf.Quota, quotaFldPath)...)
	}

	secretBindingFldPath := fldPath.Child("secretBinding")
	if conf.SecretBinding != nil {
		allErrs = append(allErrs, validateSecretBindingControllerConfiguration(conf.SecretBinding, secretBindingFldPath)...)
	}

	seedFldPath := fldPath.Child("seed")
	if conf.Seed != nil {
		allErrs = append(allErrs, validateSeedControllerConfiguration(conf.Seed, seedFldPath)...)
	}

	shootMaintenanceFldPath := fldPath.Child("shootMaintenance")
	allErrs = append(allErrs, validateShootMaintenanceControllerConfiguration(conf.ShootMaintenance, shootMaintenanceFldPath)...)

	shootQuotaFldPath := fldPath.Child("shootQuota")
	allErrs = append(allErrs, validateShootQuotaControllerConfiguration(conf.ShootQuota, shootQuotaFldPath)...)

	shootHibernationFldPath := fldPath.Child("shootHibernation")
	allErrs = append(allErrs, validateShootHibernationControllerConfiguration(conf.ShootHibernation, shootHibernationFldPath)...)

	shootReferenceFldPath := fldPath.Child("shootReference")
	if conf.ShootReference != nil {
		allErrs = append(allErrs, validateShootReferenceControllerConfiguration(conf.ShootReference, shootReferenceFldPath)...)
	}

	shootRetryFldPath := fldPath.Child("shootRetry")
	if conf.ShootRetry != nil {
		allErrs = append(allErrs, validateShootRetryControllerConfiguration(conf.ShootRetry, shootRetryFldPath)...)
	}

	shootConditionsFldPath := fldPath.Child("shootConditions")
	if conf.ShootConditions != nil {
		allErrs = append(allErrs, validateShootConditionsControllerConfiguration(conf.ShootConditions, shootConditionsFldPath)...)
	}

	shootStatusLabelFldPath := fldPath.Child("shootStatusLabel")
	if conf.ShootStatusLabel != nil {
		allErrs = append(allErrs, validateShootStatusLabelControllerConfiguration(conf.ShootStatusLabel, shootStatusLabelFldPath)...)
	}

	managedSeedSetFldPath := fldPath.Child("managedSeedSet")
	if conf.ManagedSeedSet != nil {
		allErrs = append(allErrs, validateManagedSeedSetControllerConfiguration(conf.ManagedSeedSet, managedSeedSetFldPath)...)
	}

	return allErrs
}

func validateBastionControllerConfiguration(conf *config.BastionControllerConfiguration, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	return allErrs
}

func validateCloudProfileControllerConfiguration(conf *config.CloudProfileControllerConfiguration, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	return allErrs
}

func validateControllerDeploymentControllerConfiguration(conf *config.ControllerDeploymentControllerConfiguration, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	return allErrs
}

func validateControllerRegistrationControllerConfiguration(conf *config.ControllerRegistrationControllerConfiguration, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	return allErrs
}

func validateEventControllerConfiguration(conf *config.EventControllerConfiguration, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	return allErrs
}

func validateExposureClassControllerConfiguration(conf *config.ExposureClassControllerConfiguration, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	return allErrs
}

func validateProjectControllerConfiguration(conf *config.ProjectControllerConfiguration, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	for i, quotaConfig := range conf.Quotas {
		allErrs = append(allErrs, validateProjectQuotaConfiguration(quotaConfig, fldPath.Child("quotas").Index(i))...)
	}
	return allErrs
}

func validateProjectQuotaConfiguration(conf config.QuotaConfiguration, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, metav1validation.ValidateLabelSelector(conf.ProjectSelector, metav1validation.LabelSelectorValidationOptions{AllowInvalidLabelValueInSelector: true}, fldPath.Child("projectSelector"))...)

	if conf.Config == nil {
		allErrs = append(allErrs, field.Required(fldPath.Child("config"), "must provide a quota config"))
	}

	return allErrs
}

func validateQuotaControllerConfiguration(conf *config.QuotaControllerConfiguration, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	return allErrs
}

func validateSecretBindingControllerConfiguration(conf *config.SecretBindingControllerConfiguration, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	return allErrs
}

func validateSeedControllerConfiguration(conf *config.SeedControllerConfiguration, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	return allErrs
}

func validateShootMaintenanceControllerConfiguration(conf config.ShootMaintenanceControllerConfiguration, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	return allErrs
}

func validateShootQuotaControllerConfiguration(conf config.ShootQuotaControllerConfiguration, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	return allErrs
}

func validateShootHibernationControllerConfiguration(conf config.ShootHibernationControllerConfiguration, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	return allErrs
}

func validateShootReferenceControllerConfiguration(conf *config.ShootReferenceControllerConfiguration, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	return allErrs
}

func validateShootRetryControllerConfiguration(conf *config.ShootRetryControllerConfiguration, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	return allErrs
}

func validateShootConditionsControllerConfiguration(conf *config.ShootConditionsControllerConfiguration, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	return allErrs
}

func validateShootStatusLabelControllerConfiguration(conf *config.ShootStatusLabelControllerConfiguration, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	return allErrs
}

func validateManagedSeedSetControllerConfiguration(conf *config.ManagedSeedSetControllerConfiguration, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	return allErrs
}
