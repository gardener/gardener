// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
	componentbaseconfig "k8s.io/component-base/config"
	componentbaseconfigv1alpha1 "k8s.io/component-base/config/v1alpha1"
	componentbaseconfigvalidation "k8s.io/component-base/config/validation"

	"github.com/gardener/gardener/pkg/logger"
	schedulerconfigv1alpha1 "github.com/gardener/gardener/pkg/scheduler/apis/config/v1alpha1"
)

var configScheme = runtime.NewScheme()

func init() {
	schemeBuilder := runtime.NewSchemeBuilder(
		schedulerconfigv1alpha1.AddToScheme,
		componentbaseconfigv1alpha1.AddToScheme,
	)
	utilruntime.Must(schemeBuilder.AddToScheme(configScheme))
}

// ValidateConfiguration validates the configuration.
func ValidateConfiguration(config *schedulerconfigv1alpha1.SchedulerConfiguration) field.ErrorList {
	allErrs := field.ErrorList{}

	clientConnectionPath := field.NewPath("clientConnection")
	internalClientConnectionConfig := &componentbaseconfig.ClientConnectionConfiguration{}
	if err := configScheme.Convert(&config.ClientConnection, internalClientConnectionConfig, nil); err != nil {
		allErrs = append(allErrs, field.InternalError(clientConnectionPath, err))
	} else {
		allErrs = append(allErrs, componentbaseconfigvalidation.ValidateClientConnectionConfiguration(internalClientConnectionConfig, clientConnectionPath)...)
	}

	if config.LeaderElection != nil {
		leaderElectionPath := field.NewPath("leaderElection")
		internalLeaderElectionConfig := &componentbaseconfig.LeaderElectionConfiguration{}
		if err := configScheme.Convert(config.LeaderElection, internalLeaderElectionConfig, nil); err != nil {
			allErrs = append(allErrs, field.InternalError(leaderElectionPath, err))
		} else {
			allErrs = append(allErrs, componentbaseconfigvalidation.ValidateLeaderElectionConfiguration(internalLeaderElectionConfig, leaderElectionPath)...)
		}
	}

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
func validateSchedulerControllerConfiguration(schedulers schedulerconfigv1alpha1.SchedulerControllerConfiguration, fldPath *field.Path) field.ErrorList {
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

func validateStrategy(strategy schedulerconfigv1alpha1.CandidateDeterminationStrategy, fldPath *field.Path) field.ErrorList {
	var (
		allErrs             = field.ErrorList{}
		supportedStrategies []string
	)

	for _, s := range schedulerconfigv1alpha1.Strategies {
		supportedStrategies = append(supportedStrategies, string(s))

		if s == strategy {
			return allErrs
		}
	}

	allErrs = append(allErrs, field.NotSupported(fldPath, strategy, supportedStrategies))

	return allErrs
}
