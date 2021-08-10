// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"github.com/gardener/gardener/pkg/controllermanager/apis/config"
	"github.com/gardener/gardener/pkg/logger"
	metav1validation "k8s.io/apimachinery/pkg/apis/meta/v1/validation"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// ValidateControllerManagerConfiguration validates the given `ControllerManagerConfiguration`.
func ValidateControllerManagerConfiguration(conf *config.ControllerManagerConfiguration) field.ErrorList {
	allErrs := field.ErrorList{}

	if conf.LogLevel != "" {
		if !sets.NewString(logger.AllLogLevels...).Has(conf.LogLevel) {
			allErrs = append(allErrs, field.NotSupported(field.NewPath("logLevel"), conf.LogLevel, logger.AllLogLevels))
		}
	}

	if conf.LogFormat != "" {
		if !sets.NewString(logger.AllLogFormats...).Has(conf.LogFormat) {
			allErrs = append(allErrs, field.NotSupported(field.NewPath("logFormat"), conf.LogLevel, logger.AllLogFormats))
		}
	}

	allErrs = append(allErrs, validateControllerManagerControllerConfiguration(conf.Controllers, field.NewPath("controllers"))...)
	return allErrs
}

func validateControllerManagerControllerConfiguration(conf config.ControllerManagerControllerConfiguration, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	projectFldPath := fldPath.Child("project")
	if conf.Project != nil {
		allErrs = append(allErrs, validateProjectControllerConfiguration(conf.Project, projectFldPath)...)
	}
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

	allErrs = append(allErrs, metav1validation.ValidateLabelSelector(conf.ProjectSelector, fldPath.Child("projectSelector"))...)

	if conf.Config == nil {
		allErrs = append(allErrs, field.Required(fldPath.Child("config"), "must provide a quota config"))
	}

	return allErrs
}
