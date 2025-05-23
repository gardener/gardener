// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	"fmt"
	"net"
	"time"

	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	metav1validation "k8s.io/apimachinery/pkg/apis/meta/v1/validation"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"

	gardencore "github.com/gardener/gardener/pkg/apis/core"
	gardencorehelper "github.com/gardener/gardener/pkg/apis/core/helper"
	gardencorevalidation "github.com/gardener/gardener/pkg/apis/core/validation"
	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/logger"
	validationutils "github.com/gardener/gardener/pkg/utils/validation"
)

// ValidateGardenletConfiguration validates a GardenletConfiguration object.
func ValidateGardenletConfiguration(cfg *gardenletconfigv1alpha1.GardenletConfiguration, fldPath *field.Path, inTemplate bool) field.ErrorList {
	allErrs := field.ErrorList{}

	if cfg.GardenClientConnection != nil {
		allErrs = append(allErrs, validateGardenClientConnection(cfg.GardenClientConnection, fldPath.Child("gardenClientConnection"))...)
	}

	if cfg.SeedClientConnection != nil {
		allErrs = append(allErrs, validateSeedClientConnection(cfg.SeedClientConnection, fldPath.Child("seedClientConnection"))...)
	}

	if cfg.ShootClientConnection != nil {
		allErrs = append(allErrs, validateShootClientConnection(cfg.ShootClientConnection, fldPath.Child("shootClientConnection"))...)
	}

	allErrs = append(allErrs, validationutils.ValidateLeaderElectionConfiguration(cfg.LeaderElection, field.NewPath("leaderElection"))...)

	if cfg.Controllers != nil {
		if cfg.Controllers.BackupEntry != nil {
			allErrs = append(allErrs, validateBackupEntryControllerConfiguration(cfg.Controllers.BackupEntry, fldPath.Child("controllers", "backupEntry"))...)
		}
		if cfg.Controllers.Bastion != nil {
			allErrs = append(allErrs, validateBastionControllerConfiguration(cfg.Controllers.Bastion, fldPath.Child("controllers", "bastion"))...)
		}
		if cfg.Controllers.Shoot != nil {
			allErrs = append(allErrs, validateShootControllerConfiguration(cfg.Controllers.Shoot, fldPath.Child("controllers", "shoot"))...)
		}
		if cfg.Controllers.ShootCare != nil {
			allErrs = append(allErrs, validateShootCareControllerConfiguration(cfg.Controllers.ShootCare, fldPath.Child("controllers", "shootCare"))...)
		}
		if cfg.Controllers.ManagedSeed != nil {
			allErrs = append(allErrs, validateManagedSeedControllerConfiguration(cfg.Controllers.ManagedSeed, fldPath.Child("controllers", "managedSeed"))...)
		}
		if cfg.Controllers.NetworkPolicy != nil {
			allErrs = append(allErrs, validateNetworkPolicyControllerConfiguration(cfg.Controllers.NetworkPolicy, fldPath.Child("controllers", "networkPolicy"))...)
		}
	}

	if cfg.LogLevel != "" {
		if !sets.New(logger.AllLogLevels...).Has(cfg.LogLevel) {
			allErrs = append(allErrs, field.NotSupported(field.NewPath("logLevel"), cfg.LogLevel, logger.AllLogLevels))
		}
	}

	if cfg.LogFormat != "" {
		if !sets.New(logger.AllLogFormats...).Has(cfg.LogFormat) {
			allErrs = append(allErrs, field.NotSupported(field.NewPath("logFormat"), cfg.LogFormat, logger.AllLogFormats))
		}
	}

	seedConfigPath := fldPath.Child("seedConfig")
	if !inTemplate && cfg.SeedConfig == nil {
		allErrs = append(allErrs, field.Invalid(seedConfigPath, cfg, "seed config must be set"))
	}

	if cfg.SeedConfig != nil {
		seedTemplate, err := gardencorehelper.ConvertSeedTemplate(&cfg.SeedConfig.SeedTemplate)
		if err != nil {
			allErrs = append(allErrs, field.Invalid(seedConfigPath, seedTemplate, fmt.Sprintf("could not convert gardenlet config: %v", err)))
			return allErrs
		}

		allErrs = append(allErrs, gardencorevalidation.ValidateSeedTemplate(seedTemplate, seedConfigPath)...)
	}

	resourcesPath := fldPath.Child("resources")
	if cfg.Resources != nil {
		for resourceName, quantity := range cfg.Resources.Capacity {
			if reservedQuantity, ok := cfg.Resources.Reserved[resourceName]; ok && reservedQuantity.Value() > quantity.Value() {
				allErrs = append(allErrs, field.Invalid(resourcesPath.Child("reserved", string(resourceName)), cfg.Resources.Reserved[resourceName], "reserved must be lower or equal to capacity"))
			}
		}
		for resourceName := range cfg.Resources.Reserved {
			if _, ok := cfg.Resources.Capacity[resourceName]; !ok {
				allErrs = append(allErrs, field.Invalid(resourcesPath.Child("reserved", string(resourceName)), cfg.Resources.Reserved[resourceName], "reserved without capacity"))
			}
		}
	}

	sniPath := fldPath.Child("sni", "ingress")
	if cfg.SNI != nil && cfg.SNI.Ingress != nil && cfg.SNI.Ingress.ServiceExternalIP != nil {
		if ip := net.ParseIP(*cfg.SNI.Ingress.ServiceExternalIP); ip == nil {
			allErrs = append(allErrs, field.Invalid(sniPath.Child("serviceExternalIP"), cfg.SNI.Ingress.ServiceExternalIP, "external service ip is invalid"))
		}
	}

	exposureClassHandlersPath := fldPath.Child("exposureClassHandlers")
	for i, handler := range cfg.ExposureClassHandlers {
		handlerPath := exposureClassHandlersPath.Index(i)

		for _, errorMessage := range validation.IsDNS1123Label(handler.Name) {
			allErrs = append(allErrs, field.Invalid(handlerPath.Child("name"), handler.Name, errorMessage))
		}

		if handler.SNI != nil && handler.SNI.Ingress != nil && handler.SNI.Ingress.ServiceExternalIP != nil {
			if ip := net.ParseIP(*handler.SNI.Ingress.ServiceExternalIP); ip == nil {
				allErrs = append(allErrs, field.Invalid(handlerPath.Child("sni", "ingress", "serviceExternalIP"), handler.SNI.Ingress.ServiceExternalIP, "external service ip is invalid"))
			}
		}
	}

	if nodeTolerationCfg := cfg.NodeToleration; nodeTolerationCfg != nil {
		nodeTolerationConfigPath := fldPath.Child("nodeToleration")

		allErrs = append(allErrs, apivalidation.ValidateNonnegativeField(ptr.Deref(nodeTolerationCfg.DefaultNotReadyTolerationSeconds, 0), nodeTolerationConfigPath.Child("defaultNotReadyTolerationSeconds"))...)
		allErrs = append(allErrs, apivalidation.ValidateNonnegativeField(ptr.Deref(nodeTolerationCfg.DefaultUnreachableTolerationSeconds, 0), nodeTolerationConfigPath.Child("defaultUnreachableTolerationSeconds"))...)
	}

	return allErrs
}

// ValidateGardenletConfigurationUpdate validates a GardenletConfiguration object before an update.
func ValidateGardenletConfigurationUpdate(newCfg, oldCfg *gardenletconfigv1alpha1.GardenletConfiguration, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if newCfg.SeedConfig != nil && oldCfg.SeedConfig != nil {
		seedConfigPath := fldPath.Child("seedConfig")
		newSeedTemplate, err := gardencorehelper.ConvertSeedTemplate(&newCfg.SeedConfig.SeedTemplate)
		if err != nil {
			allErrs = append(allErrs, field.Invalid(seedConfigPath, newSeedTemplate, fmt.Sprintf("could not convert gardenlet config: %v", err)))
			return allErrs
		}

		oldSeedTemplate, err := gardencorehelper.ConvertSeedTemplate(&oldCfg.SeedConfig.SeedTemplate)
		if err != nil {
			allErrs = append(allErrs, field.Invalid(seedConfigPath, oldSeedTemplate, fmt.Sprintf("could not convert gardenlet config: %v", err)))
			return allErrs
		}

		allErrs = append(allErrs, gardencorevalidation.ValidateSeedTemplateUpdate(newSeedTemplate, oldSeedTemplate, seedConfigPath)...)
	}

	return allErrs
}

func validateGardenClientConnection(conf *gardenletconfigv1alpha1.GardenClientConnection, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, validationutils.ValidateClientConnectionConfiguration(&conf.ClientConnectionConfiguration, fldPath)...)

	if conf.KubeconfigValidity != nil {
		allErrs = append(allErrs, validateKubeconfigValidity(conf.KubeconfigValidity, fldPath.Child("kubeconfigValidity"))...)
	}

	return allErrs
}

func validateKubeconfigValidity(conf *gardenletconfigv1alpha1.KubeconfigValidity, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if v := conf.Validity; v != nil && v.Duration < 10*time.Minute {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("validity"), *v, "validity must be at least 10m"))
	}

	if v := conf.AutoRotationJitterPercentageMin; v != nil && *v < 1 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("autoRotationJitterPercentageMin"), *v, "minimum percentage must be at least 1"))
	}
	if v := conf.AutoRotationJitterPercentageMax; v != nil && *v > 100 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("autoRotationJitterPercentageMax"), *v, "maximum percentage must be at most 100"))
	}
	if conf.AutoRotationJitterPercentageMin != nil &&
		conf.AutoRotationJitterPercentageMax != nil &&
		*conf.AutoRotationJitterPercentageMin >= *conf.AutoRotationJitterPercentageMax {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("autoRotationJitterPercentageMin"), *conf.AutoRotationJitterPercentageMin, "minimum percentage must be less than maximum percentage"))
	}

	return allErrs
}

func validateSeedClientConnection(conf *gardenletconfigv1alpha1.SeedClientConnection, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, validationutils.ValidateClientConnectionConfiguration(&conf.ClientConnectionConfiguration, fldPath)...)

	return allErrs
}

func validateShootClientConnection(conf *gardenletconfigv1alpha1.ShootClientConnection, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, validationutils.ValidateClientConnectionConfiguration(&conf.ClientConnectionConfiguration, fldPath)...)

	return allErrs
}

func validateShootControllerConfiguration(cfg *gardenletconfigv1alpha1.ShootControllerConfiguration, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if cfg.ConcurrentSyncs != nil {
		allErrs = append(allErrs, apivalidation.ValidateNonnegativeField(int64(*cfg.ConcurrentSyncs), fldPath.Child("concurrentSyncs"))...)
	}

	if cfg.ProgressReportPeriod != nil {
		allErrs = append(allErrs, apivalidation.ValidateNonnegativeField(int64(cfg.ProgressReportPeriod.Duration), fldPath.Child("progressReporterPeriod"))...)
	}

	if cfg.RetryDuration != nil {
		allErrs = append(allErrs, apivalidation.ValidateNonnegativeField(int64(cfg.RetryDuration.Duration), fldPath.Child("retryDuration"))...)
	}

	if cfg.SyncPeriod != nil {
		allErrs = append(allErrs, apivalidation.ValidateNonnegativeField(int64(cfg.SyncPeriod.Duration), fldPath.Child("syncPeriod"))...)
	}

	if cfg.DNSEntryTTLSeconds != nil {
		const (
			dnsEntryTTLSecondsMin = 30
			dnsEntryTTLSecondsMax = 600
		)

		if *cfg.DNSEntryTTLSeconds < dnsEntryTTLSecondsMin || *cfg.DNSEntryTTLSeconds > dnsEntryTTLSecondsMax {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("dnsEntryTTLSeconds"), *cfg.DNSEntryTTLSeconds, fmt.Sprintf("must be within [%d,%d]", dnsEntryTTLSecondsMin, dnsEntryTTLSecondsMax)))
		}
	}

	return allErrs
}

func validateShootCareControllerConfiguration(cfg *gardenletconfigv1alpha1.ShootCareControllerConfiguration, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if cfg.ConcurrentSyncs != nil {
		allErrs = append(allErrs, apivalidation.ValidateNonnegativeField(int64(*cfg.ConcurrentSyncs), fldPath.Child("concurrentSyncs"))...)
	}

	if cfg.SyncPeriod != nil {
		allErrs = append(allErrs, apivalidation.ValidateNonnegativeField(int64(cfg.SyncPeriod.Duration), fldPath.Child("syncPeriod"))...)
	}

	if cfg.StaleExtensionHealthChecks != nil {
		allErrs = append(allErrs, apivalidation.ValidateNonnegativeField(int64(cfg.StaleExtensionHealthChecks.Threshold.Duration), fldPath.Child("staleExtensionHealthChecks", "threshold"))...)
	}

	if cfg.ManagedResourceProgressingThreshold != nil {
		allErrs = append(allErrs, apivalidation.ValidateNonnegativeField(int64(cfg.ManagedResourceProgressingThreshold.Duration), fldPath.Child("managedResourceProgressingThreshold"))...)
	}

	for i := range cfg.ConditionThresholds {
		allErrs = append(allErrs, apivalidation.ValidateNonnegativeField(int64(cfg.ConditionThresholds[i].Duration.Duration), fldPath.Child("conditionThresholds").Index(i).Child("duration"))...)
	}

	return allErrs
}

func validateManagedSeedControllerConfiguration(cfg *gardenletconfigv1alpha1.ManagedSeedControllerConfiguration, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if cfg.ConcurrentSyncs != nil {
		allErrs = append(allErrs, apivalidation.ValidateNonnegativeField(int64(*cfg.ConcurrentSyncs), fldPath.Child("concurrentSyncs"))...)
	}
	if cfg.SyncPeriod != nil {
		allErrs = append(allErrs, apivalidation.ValidateNonnegativeField(int64(cfg.SyncPeriod.Duration), fldPath.Child("syncPeriod"))...)
	}
	if cfg.WaitSyncPeriod != nil {
		allErrs = append(allErrs, apivalidation.ValidateNonnegativeField(int64(cfg.WaitSyncPeriod.Duration), fldPath.Child("waitSyncPeriod"))...)
	}
	if cfg.SyncJitterPeriod != nil {
		allErrs = append(allErrs, apivalidation.ValidateNonnegativeField(int64(cfg.SyncJitterPeriod.Duration), fldPath.Child("syncJitterPeriod"))...)
	}

	return allErrs
}

func validateNetworkPolicyControllerConfiguration(cfg *gardenletconfigv1alpha1.NetworkPolicyControllerConfiguration, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if cfg.ConcurrentSyncs != nil {
		allErrs = append(allErrs, apivalidation.ValidateNonnegativeField(int64(*cfg.ConcurrentSyncs), fldPath.Child("concurrentSyncs"))...)
	}

	for i, l := range cfg.AdditionalNamespaceSelectors {
		labelSelector := l
		allErrs = append(allErrs, metav1validation.ValidateLabelSelector(&labelSelector, metav1validation.LabelSelectorValidationOptions{}, fldPath.Child("additionalNamespaceSelectors").Index(i))...)
	}

	return allErrs
}

var availableShootPurposes = sets.New(
	string(gardencore.ShootPurposeEvaluation),
	string(gardencore.ShootPurposeTesting),
	string(gardencore.ShootPurposeDevelopment),
	string(gardencore.ShootPurposeInfrastructure),
	string(gardencore.ShootPurposeProduction),
)

func validateBackupEntryControllerConfiguration(cfg *gardenletconfigv1alpha1.BackupEntryControllerConfiguration, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(cfg.DeletionGracePeriodShootPurposes) > 0 && (cfg.DeletionGracePeriodHours == nil || *cfg.DeletionGracePeriodHours <= 0) {
		allErrs = append(allErrs, field.Forbidden(fldPath.Child("deletionGracePeriodShootPurposes"), "must specify grace period hours > 0 when specifying purposes"))
	}

	for i, purpose := range cfg.DeletionGracePeriodShootPurposes {
		if !availableShootPurposes.Has(string(purpose)) {
			allErrs = append(allErrs, field.NotSupported(fldPath.Child("deletionGracePeriodShootPurposes").Index(i), purpose, sets.List(availableShootPurposes)))
		}
	}

	return allErrs
}

func validateBastionControllerConfiguration(cfg *gardenletconfigv1alpha1.BastionControllerConfiguration, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if cfg.ConcurrentSyncs != nil {
		allErrs = append(allErrs, apivalidation.ValidateNonnegativeField(int64(*cfg.ConcurrentSyncs), fldPath.Child("concurrentSyncs"))...)
	}

	return allErrs
}
