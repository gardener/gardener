// SPDX-FileCopyrightText: 2019 SAP SE or an SAP affiliate company and Gardener contributors
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/gardener/gardener/pkg/logger"
	schedulerconfig "github.com/gardener/gardener/pkg/scheduler/apis/config"
)

// ValidateConfiguration validates the configuration.
func ValidateConfiguration(config *schedulerconfig.SchedulerConfiguration) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, validateSchedulerControllerConfiguration(config.Schedulers, field.NewPath("schedulers"))...)

	if config.LogLevel != "" {
		if !sets.New(logger.AllLogLevels...).Has(config.LogLevel) {
			allErrs = append(allErrs, field.NotSupported(field.NewPath("logLevel"), config.LogLevel, logger.AllLogLevels))
		}
	}

	if config.LogFormat != "" {
		if !sets.New(logger.AllLogFormats...).Has(config.LogFormat) {
			allErrs = append(allErrs, field.NotSupported(field.NewPath("logFormat"), config.LogFormat, logger.AllLogFormats))
		}
	}

	return allErrs
}

// validateSchedulerControllerConfiguration validates the scheduler controller configuration.
func validateSchedulerControllerConfiguration(schedulers schedulerconfig.SchedulerControllerConfiguration, fldPath *field.Path) field.ErrorList {
	var (
		allErrs = field.ErrorList{}
	)

	if schedulers.BackupBucket != nil {
		allErrs = append(allErrs, apivalidation.ValidateNonnegativeField(int64(schedulers.BackupBucket.ConcurrentSyncs), fldPath.Child("backupBucket", "concurrentSyncs"))...)
	}

	if schedulers.Shoot != nil {
		allErrs = append(allErrs, apivalidation.ValidateNonnegativeField(int64(schedulers.Shoot.ConcurrentSyncs), fldPath.Child("shoot", "concurrentSyncs"))...)
		allErrs = append(allErrs, validateStrategy(schedulers.Shoot.Strategy, fldPath.Child("shoot", "strategy"))...)
	}

	return allErrs
}

func validateStrategy(strategy schedulerconfig.CandidateDeterminationStrategy, fldPath *field.Path) field.ErrorList {
	var (
		allErrs             = field.ErrorList{}
		supportedStrategies []string
	)

	for _, s := range schedulerconfig.Strategies {
		supportedStrategies = append(supportedStrategies, string(s))
		if s == strategy {
			return allErrs
		}
	}

	allErrs = append(allErrs, field.NotSupported(fldPath, strategy, supportedStrategies))

	return allErrs
}
