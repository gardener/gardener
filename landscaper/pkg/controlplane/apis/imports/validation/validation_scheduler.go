// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"fmt"

	"github.com/gardener/gardener/landscaper/pkg/controlplane/apis/imports"
	confighelper "github.com/gardener/gardener/pkg/scheduler/apis/config/helper"
	configvalidation "github.com/gardener/gardener/pkg/scheduler/apis/config/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// ValidateScheduler validates the configuration of the Gardener Scheduler
func ValidateScheduler(config imports.GardenerScheduler, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if config.DeploymentConfiguration != nil {
		allErrs = append(allErrs, ValidateCommonDeployment(*config.DeploymentConfiguration, fldPath.Child("deploymentConfiguration"))...)
	}

	if config.ComponentConfiguration != nil {
		allErrs = append(allErrs, ValidateSchedulerComponentConfiguration(*config.ComponentConfiguration, fldPath.Child("componentConfiguration"))...)
	}

	return allErrs
}

// ValidateSchedulerComponentConfiguration validates the component configuration of the Gardener Scheduler
func ValidateSchedulerComponentConfiguration(config imports.SchedulerComponentConfiguration, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if config.Configuration != nil {
		// Convert the Gardener Scheduler config to an internal version
		componentConfig, err := confighelper.ConvertSchedulerConfiguration(config.Configuration.ComponentConfiguration)
		if err != nil {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("config"), config.Configuration.ComponentConfiguration, fmt.Sprintf("could not convert to Gardener Scheduler configuration: %v", err)))
			return allErrs
		}

		if errorList := configvalidation.ValidateConfiguration(componentConfig); len(errorList) > 0 {
			for _, err := range errorList {
				err.Field = fmt.Sprintf("%s.%s", fldPath.Child("config").String(), err.Field)
				allErrs = append(allErrs, err)
			}
		}
	}
	return allErrs
}
