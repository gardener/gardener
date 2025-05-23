// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/gardener/gardener/pkg/logger"
	nodeagentconfigv1alpha1 "github.com/gardener/gardener/pkg/nodeagent/apis/config/v1alpha1"
	validationutils "github.com/gardener/gardener/pkg/utils/validation"
	"github.com/gardener/gardener/pkg/utils/validation/kubernetesversion"
)

// ValidateNodeAgentConfiguration validates the given `NodeAgentConfiguration`.
func ValidateNodeAgentConfiguration(conf *nodeagentconfigv1alpha1.NodeAgentConfiguration) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, validationutils.ValidateClientConnectionConfiguration(&conf.ClientConnection, field.NewPath("clientConnection"))...)

	if conf.LogLevel != "" && !sets.New(logger.AllLogLevels...).Has(conf.LogLevel) {
		allErrs = append(allErrs, field.NotSupported(field.NewPath("logLevel"), conf.LogLevel, logger.AllLogLevels))
	}

	if conf.LogFormat != "" && !sets.New(logger.AllLogFormats...).Has(conf.LogFormat) {
		allErrs = append(allErrs, field.NotSupported(field.NewPath("logFormat"), conf.LogFormat, logger.AllLogFormats))
	}

	allErrs = append(allErrs, validateBootstrapConfiguration(conf.Bootstrap, field.NewPath("bootstrap"))...)
	allErrs = append(allErrs, validateControllerConfiguration(conf.Controllers, field.NewPath("controllers"))...)

	return allErrs
}

func validateBootstrapConfiguration(_ *nodeagentconfigv1alpha1.BootstrapConfiguration, _ *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	return allErrs
}

func validateControllerConfiguration(conf nodeagentconfigv1alpha1.ControllerConfiguration, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, validateOperatingSystemConfigControllerConfiguration(conf.OperatingSystemConfig, fldPath.Child("operatingSystemConfig"))...)
	allErrs = append(allErrs, validateTokenControllerConfiguration(conf.Token, fldPath.Child("token"))...)

	return allErrs
}

func validateOperatingSystemConfigControllerConfiguration(conf nodeagentconfigv1alpha1.OperatingSystemConfigControllerConfig, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if conf.SecretName == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("secretName"), "must provide the secret name for the operating system config"))
	}

	allErrs = append(allErrs, validateSyncPeriod(conf.SyncPeriod, fldPath)...)

	if conf.KubernetesVersion == nil {
		allErrs = append(allErrs, field.Required(fldPath.Child("kubernetesVersion"), "must provide a supported kubernetes version"))
	} else if err := kubernetesversion.CheckIfSupported(conf.KubernetesVersion.String()); err != nil {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("kubernetesVersion"), conf.KubernetesVersion, err.Error()))
	}

	return allErrs
}

func validateTokenControllerConfiguration(conf nodeagentconfigv1alpha1.TokenControllerConfig, fldPath *field.Path) field.ErrorList {
	var (
		allErrs = field.ErrorList{}
		paths   = sets.New[string]()
	)

	for i, cfg := range conf.SyncConfigs {
		idxPath := fldPath.Child("syncConfigs").Index(i)

		if cfg.SecretName == "" {
			allErrs = append(allErrs, field.Required(idxPath.Child("secretName"), "must provide the secret name for the access token"))
		}

		if cfg.Path == "" {
			allErrs = append(allErrs, field.Required(idxPath.Child("path"), "must provide the path where the token should be synced to"))
		} else {
			if paths.Has(cfg.Path) {
				allErrs = append(allErrs, field.Duplicate(idxPath.Child("path"), cfg.Path))
			}
			paths.Insert(cfg.Path)
		}
	}

	allErrs = append(allErrs, validateSyncPeriod(conf.SyncPeriod, fldPath)...)

	return allErrs
}

func validateSyncPeriod(val *metav1.Duration, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if val == nil || val.Duration < 15*time.Second {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("syncPeriod"), val, "must be at least 15s"))
	}

	return allErrs
}
