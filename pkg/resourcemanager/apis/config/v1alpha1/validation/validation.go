// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	"time"

	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener/pkg/logger"
	resourcemanagerconfigv1alpha1 "github.com/gardener/gardener/pkg/resourcemanager/apis/config/v1alpha1"
	validationutils "github.com/gardener/gardener/pkg/utils/validation"
	kubernetescorevalidation "github.com/gardener/gardener/pkg/utils/validation/kubernetes/core"
)

// ValidateResourceManagerConfiguration validates the given `ResourceManagerConfiguration`.
func ValidateResourceManagerConfiguration(conf *resourcemanagerconfigv1alpha1.ResourceManagerConfiguration) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, validateClientConnection(conf.SourceClientConnection, field.NewPath("sourceClientConnection"))...)
	if conf.TargetClientConnection != nil {
		allErrs = append(allErrs, validateClientConnection(*conf.TargetClientConnection, field.NewPath("targetClientConnection"))...)
	}

	allErrs = append(allErrs, validationutils.ValidateLeaderElectionConfiguration(&conf.LeaderElection, field.NewPath("leaderElection"))...)
	allErrs = append(allErrs, validateServerConfiguration(conf.Server, field.NewPath("server"))...)

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

func validateClientConnection(conf resourcemanagerconfigv1alpha1.ClientConnection, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if conf.CacheResyncPeriod != nil && conf.CacheResyncPeriod.Duration < 10*time.Second {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("cacheResyncPeriod"), conf.CacheResyncPeriod.Duration, "must be at least 10s"))
	}

	allErrs = append(allErrs, validationutils.ValidateClientConnectionConfiguration(&conf.ClientConnectionConfiguration, fldPath)...)

	return allErrs
}

func validateServerConfiguration(conf resourcemanagerconfigv1alpha1.ServerConfiguration, fldPath *field.Path) field.ErrorList {
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

func validateResourceManagerControllerConfiguration(conf resourcemanagerconfigv1alpha1.ResourceManagerControllerConfiguration, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if conf.ClusterID == nil {
		allErrs = append(allErrs, field.Required(fldPath.Child("clusterID"), "cluster id must be non-nil"))
	}

	if len(ptr.Deref(conf.ResourceClass, "")) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("resourceClass"), "must provide a resource class"))
	}

	if conf.CSRApprover.Enabled {
		allErrs = append(allErrs, validateConcurrentSyncs(conf.CSRApprover.ConcurrentSyncs, fldPath.Child("csrApprover"))...)
	}

	if conf.GarbageCollector.Enabled {
		allErrs = append(allErrs, validateSyncPeriod(conf.GarbageCollector.SyncPeriod, fldPath.Child("garbageCollector"))...)
	}

	allErrs = append(allErrs, validateConcurrentSyncs(conf.Health.ConcurrentSyncs, fldPath.Child("health"))...)
	allErrs = append(allErrs, validateSyncPeriod(conf.Health.SyncPeriod, fldPath.Child("health"))...)

	allErrs = append(allErrs, validateManagedResourceControllerConfiguration(conf.ManagedResource, fldPath.Child("managedResources"))...)

	if conf.TokenRequestor.Enabled {
		allErrs = append(allErrs, validateConcurrentSyncs(conf.TokenRequestor.ConcurrentSyncs, fldPath.Child("tokenRequestor"))...)
	}

	if conf.NodeAgentReconciliationDelay.Enabled {
		allErrs = append(allErrs, validateNodeAgentReconciliationDelayControllerConfiguration(conf.NodeAgentReconciliationDelay, fldPath.Child("nodeAgentReconciliationDelay"))...)
	}

	return allErrs
}

func validateManagedResourceControllerConfiguration(conf resourcemanagerconfigv1alpha1.ManagedResourceControllerConfig, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, validateConcurrentSyncs(conf.ConcurrentSyncs, fldPath)...)
	allErrs = append(allErrs, validateSyncPeriod(conf.SyncPeriod, fldPath)...)

	if len(ptr.Deref(conf.ManagedByLabelValue, "")) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("managedByLabelValue"), "must specify value of managed-by label"))
	}

	return allErrs
}

func validateNodeAgentReconciliationDelayControllerConfiguration(conf resourcemanagerconfigv1alpha1.NodeAgentReconciliationDelayControllerConfig, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if conf.MinDelay != nil && conf.MinDelay.Seconds() < 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("minDelay"), conf.MinDelay.Duration.String(), "must be non-negative"))
	}
	if conf.MaxDelay != nil && conf.MaxDelay.Seconds() < 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("maxDelay"), conf.MaxDelay.Duration.String(), "must be non-negative"))
	}

	if conf.MinDelay != nil && conf.MaxDelay != nil && conf.MinDelay.Duration > conf.MaxDelay.Duration {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("minDelay"), conf.MinDelay.Duration.String(), "minimum delay must not be higher than maximum delay"))
	}

	return allErrs
}

func validateResourceManagerWebhookConfiguration(conf resourcemanagerconfigv1alpha1.ResourceManagerWebhookConfiguration, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, validatePodSchedulerNameWebhookConfiguration(conf.PodSchedulerName, fldPath.Child("podSchedulerName"))...)
	allErrs = append(allErrs, validateProjectedTokenMountWebhookConfiguration(conf.ProjectedTokenMount, fldPath.Child("projectedTokenMount"))...)
	allErrs = append(allErrs, validateHighAvailabilityConfigWebhookConfiguration(conf.HighAvailabilityConfig, fldPath.Child("highAvailabilityConfig"))...)
	allErrs = append(allErrs, validateSystemComponentsConfigWebhookConfig(&conf.SystemComponentsConfig, fldPath.Child("systemComponentsConfig"))...)
	allErrs = append(allErrs, validateNodeAgentAuthorizerWebhookConfiguration(conf.NodeAgentAuthorizer, fldPath.Child("nodeAgentAuthorizer"))...)

	return allErrs
}

func validatePodSchedulerNameWebhookConfiguration(conf resourcemanagerconfigv1alpha1.PodSchedulerNameWebhookConfig, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if conf.Enabled && len(ptr.Deref(conf.SchedulerName, "")) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("schedulerName"), "must specify schedulerName when webhook is enabled"))
	}

	return allErrs
}

func validateProjectedTokenMountWebhookConfiguration(conf resourcemanagerconfigv1alpha1.ProjectedTokenMountWebhookConfig, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if conf.Enabled && ptr.Deref(conf.ExpirationSeconds, 0) < 600 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("expirationSeconds"), ptr.Deref(conf.ExpirationSeconds, 0), "must be at least 600"))
	}

	return allErrs
}

func validateNodeAgentAuthorizerWebhookConfiguration(conf resourcemanagerconfigv1alpha1.NodeAgentAuthorizerWebhookConfig, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if conf.Enabled && conf.MachineNamespace == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("machineNamespace"), "machine namespace must not be empty"))
	}

	return allErrs
}

func validateHighAvailabilityConfigWebhookConfiguration(conf resourcemanagerconfigv1alpha1.HighAvailabilityConfigWebhookConfig, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateNonnegativeField(ptr.Deref(conf.DefaultNotReadyTolerationSeconds, 0), fldPath.Child("defaultNotReadyTolerationSeconds"))...)
	allErrs = append(allErrs, apivalidation.ValidateNonnegativeField(ptr.Deref(conf.DefaultUnreachableTolerationSeconds, 0), fldPath.Child("defaultUnreachableTolerationSeconds"))...)

	return allErrs
}

func validateConcurrentSyncs(val *int, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if ptr.Deref(val, 0) <= 0 {
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

func validateSystemComponentsConfigWebhookConfig(conf *resourcemanagerconfigv1alpha1.SystemComponentsConfigWebhookConfig, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, kubernetescorevalidation.ValidateTolerations(conf.PodTolerations, fldPath.Child("podTolerations"))...)

	return allErrs
}
