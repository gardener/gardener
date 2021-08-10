// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/gardener/gardener/pkg/logger"
	schedulerapi "github.com/gardener/gardener/pkg/scheduler/apis/config"
	"k8s.io/apimachinery/pkg/util/sets"
)

// ValidateConfiguration validates the configuration.
func ValidateConfiguration(config *schedulerapi.SchedulerConfiguration) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, validateStrategy(config.Schedulers.Shoot.Strategy, field.NewPath("schedulers", "shoot", "strategy"))...)

	if config.LogLevel != "" {
		if !sets.NewString(logger.AllLogLevels...).Has(config.LogLevel) {
			allErrs = append(allErrs, field.NotSupported(field.NewPath("logLevel"), config.LogLevel, logger.AllLogLevels))
		}
	}

	if config.LogFormat != "" {
		if !sets.NewString(logger.AllLogFormats...).Has(config.LogFormat) {
			allErrs = append(allErrs, field.NotSupported(field.NewPath("logFormat"), config.LogFormat, logger.AllLogFormats))
		}
	}

	return allErrs
}

func validateStrategy(strategy schedulerapi.CandidateDeterminationStrategy, fldPath *field.Path) field.ErrorList {
	var (
		allErrs             = field.ErrorList{}
		supportedStrategies []string
	)

	for _, s := range schedulerapi.Strategies {
		supportedStrategies = append(supportedStrategies, string(s))
		if s == strategy {
			return allErrs
		}
	}

	allErrs = append(allErrs, field.NotSupported(fldPath, strategy, supportedStrategies))

	return allErrs
}
