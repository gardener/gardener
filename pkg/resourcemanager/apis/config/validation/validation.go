// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"time"

	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
	componentbaseconfigvalidation "k8s.io/component-base/config/validation"
	"k8s.io/utils/pointer"

	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/resourcemanager/apis/config"
	corevalidation "github.com/gardener/gardener/pkg/utils/validation/kubernetes/core"
)

// ValidateResourceManagerConfiguration validates the given `ResourceManagerConfiguration`.
func ValidateResourceManagerConfiguration(conf *config.ResourceManagerConfiguration) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, validateSourceClientConnection(conf.SourceClientConnection, field.NewPath("sourceClientConnection"))...)
	if conf.TargetClientConnection != nil {
		allErrs = append(allErrs, validateTargetClientConnection(*conf.TargetClientConnection, field.NewPath("targetClientConnection"))...)
	}
	allErrs = append(allErrs, validateServerConfiguration(conf.Server, field.NewPath("server"))...)
	allErrs = append(allErrs, componentbaseconfigvalidation.ValidateLeaderElectionConfiguration(&conf.LeaderElection, field.NewPath("leaderElection"))...)

	if !sets.New(logger.AllLogLevels...).Has(conf.LogLevel) {
		allErrs = append(allErrs, field.NotSupported(field.NewPath("logLevel"), conf.LogLevel, logger.AllLogLevels))
	}
	if !sets.New(logger.AllLogFormats...).Has(conf.LogFormat) {
		allErrs = append(allErrs, field.NotSupported(field.NewPath("logFormat"), conf.LogFormat, logger.AllLogFormats))
	}

	allErrs = append(allErrs, validateResourceManagerControllerConfiguration(conf.Controllers, field.NewPath("controllers"))...)
	allErrs = append(allErrs, validateResourceManagerWebhookConfiguration(conf.Webhooks, field.NewPath("webhooks"))...)

	if conf.Controllers.TokenInvalidator.Enabled != conf.Webhooks.TokenInvalidator.Enabled {
		allErrs = append(allErrs, field.Forbidden(field.NewPath("controllers", "tokenInvalidator"), "controller and webhook for TokenInvalidator must either be both disabled or enabled"))
	}

	return allErrs
}

func validateSourceClientConnection(conf config.SourceClientConnection, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if conf.CacheResyncPeriod != nil && conf.CacheResyncPeriod.Duration < 10*time.Second {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("cacheResyncPeriod"), conf.CacheResyncPeriod.Duration, "must be at least 10s"))
	}

	allErrs = append(allErrs, componentbaseconfigvalidation.ValidateClientConnectionConfiguration(&conf.ClientConnectionConfiguration, fldPath)...)

	return allErrs
}

func validateTargetClientConnection(conf config.TargetClientConnection, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if conf.CacheResyncPeriod != nil && conf.CacheResyncPeriod.Duration < 10*time.Second {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("cacheResyncPeriod"), conf.CacheResyncPeriod.Duration, "must be at least 10s"))
	}

	allErrs = append(allErrs, componentbaseconfigvalidation.ValidateClientConnectionConfiguration(&conf.ClientConnectionConfiguration, fldPath)...)

	return allErrs
}

func validateServerConfiguration(conf config.ServerConfiguration, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if conf.HealthProbes == nil {
		allErrs = append(allErrs, field.Required(fldPath.Child("healthProbes"), "must provide health probes server configuration"))
	} else {
		allErrs = append(allErrs, apivalidation.ValidateNonnegativeField(int64(conf.HealthProbes.Port), fldPath.Child("healthProbes", "port"))...)
	}

	if conf.Metrics == nil {
		allErrs = append(allErrs, field.Required(fldPath.Child("metrics"), "must provide metrics server configuration"))
	} else {
		allErrs = append(allErrs, apivalidation.ValidateNonnegativeField(int64(conf.Metrics.Port), fldPath.Child("metrics", "port"))...)
	}

	return allErrs
}

func validateResourceManagerControllerConfiguration(conf config.ResourceManagerControllerConfiguration, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if conf.ClusterID == nil {
		allErrs = append(allErrs, field.Required(fldPath.Child("clusterID"), "cluster id must be non-nil"))
	}
	if len(pointer.StringDeref(conf.ResourceClass, "")) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("resourceClass"), "must provide a resource class"))
	}

	if conf.KubeletCSRApprover.Enabled {
		allErrs = append(allErrs, validateConcurrentSyncs(conf.KubeletCSRApprover.ConcurrentSyncs, fldPath.Child("kubeletCSRApprover"))...)
	}
	if conf.GarbageCollector.Enabled {
		allErrs = append(allErrs, validateSyncPeriod(conf.GarbageCollector.SyncPeriod, fldPath.Child("garbageCollector"))...)
	}

	allErrs = append(allErrs, validateConcurrentSyncs(conf.Health.ConcurrentSyncs, fldPath.Child("health"))...)
	allErrs = append(allErrs, validateSyncPeriod(conf.Health.SyncPeriod, fldPath.Child("health"))...)

	allErrs = append(allErrs, validateManagedResourceControllerConfiguration(conf.ManagedResource, fldPath.Child("managedResources"))...)

	allErrs = append(allErrs, validateConcurrentSyncs(conf.Secret.ConcurrentSyncs, fldPath.Child("secret"))...)

	if conf.TokenRequestor.Enabled {
		allErrs = append(allErrs, validateConcurrentSyncs(conf.TokenRequestor.ConcurrentSyncs, fldPath.Child("tokenRequestor"))...)
	}

	return allErrs
}

func validateManagedResourceControllerConfiguration(conf config.ManagedResourceControllerConfig, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, validateConcurrentSyncs(conf.ConcurrentSyncs, fldPath)...)
	allErrs = append(allErrs, validateSyncPeriod(conf.SyncPeriod, fldPath)...)

	if len(pointer.StringDeref(conf.ManagedByLabelValue, "")) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("managedByLabelValue"), "must specify value of managed-by label"))
	}

	return allErrs
}

func validateResourceManagerWebhookConfiguration(conf config.ResourceManagerWebhookConfiguration, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, validatePodSchedulerNameWebhookConfiguration(conf.PodSchedulerName, fldPath.Child("podSchedulerName"))...)
	allErrs = append(allErrs, validateProjectedTokenMountWebhookConfiguration(conf.ProjectedTokenMount, fldPath.Child("projectedTokenMount"))...)
	allErrs = append(allErrs, validateHighAvailabilityConfigWebhookConfiguration(conf.HighAvailabilityConfig, fldPath.Child("highAvailabilityConfig"))...)
	allErrs = append(allErrs, validateSystemComponentsConfigWebhookConfig(&conf.SystemComponentsConfig, fldPath.Child("systemComponentsConfig"))...)

	return allErrs
}

func validatePodSchedulerNameWebhookConfiguration(conf config.PodSchedulerNameWebhookConfig, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if conf.Enabled && len(pointer.StringDeref(conf.SchedulerName, "")) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("schedulerName"), "must specify schedulerName when webhook is enabled"))
	}

	return allErrs
}

func validateProjectedTokenMountWebhookConfiguration(conf config.ProjectedTokenMountWebhookConfig, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if conf.Enabled && pointer.Int64Deref(conf.ExpirationSeconds, 0) < 600 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("expirationSeconds"), pointer.Int64Deref(conf.ExpirationSeconds, 0), "must be at least 600"))
	}

	return allErrs
}

func validateHighAvailabilityConfigWebhookConfiguration(conf config.HighAvailabilityConfigWebhookConfig, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateNonnegativeField(pointer.Int64Deref(conf.DefaultNotReadyTolerationSeconds, 0), fldPath.Child("defaultNotReadyTolerationSeconds"))...)
	allErrs = append(allErrs, apivalidation.ValidateNonnegativeField(pointer.Int64Deref(conf.DefaultUnreachableTolerationSeconds, 0), fldPath.Child("defaultUnreachableTolerationSeconds"))...)

	return allErrs
}

func validateConcurrentSyncs(val *int, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if pointer.IntDeref(val, 0) <= 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("concurrentSyncs"), val, "must be at least 1"))
	}

	return allErrs
}

func validateSyncPeriod(val *metav1.Duration, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if val == nil || val.Duration < 15*time.Second {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("syncPeriod"), val, "must be at least 15s"))
	}

	return allErrs
}

func validateSystemComponentsConfigWebhookConfig(conf *config.SystemComponentsConfigWebhookConfig, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, corevalidation.ValidateTolerations(conf.PodTolerations, fldPath.Child("podTolerations"))...)

	return allErrs
}
