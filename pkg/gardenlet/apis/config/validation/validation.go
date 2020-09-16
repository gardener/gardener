// SPDX-FileCopyrightText: 2019 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"

	"k8s.io/apimachinery/pkg/util/validation/field"
)

// ValidateGardenletConfiguration validates a GardenletConfiguration object.
func ValidateGardenletConfiguration(cfg *config.GardenletConfiguration) field.ErrorList {
	allErrs := field.ErrorList{}

	if (cfg.SeedConfig == nil && cfg.SeedSelector == nil) || (cfg.SeedConfig != nil && cfg.SeedSelector != nil) {
		allErrs = append(allErrs, field.Invalid(field.NewPath("seedSelector/seedConfig"), cfg, "exactly one of `seedConfig` and `seedSelector` is required"))
	}

	serverPath := field.NewPath("server")
	if cfg.Server == nil {
		allErrs = append(allErrs, field.Required(serverPath, "require configuration for server"))
	} else {
		if len(cfg.Server.HTTPS.BindAddress) == 0 {
			allErrs = append(allErrs, field.Required(serverPath.Child("https", "bindAddress"), "bind address must be specified"))
		}
		if cfg.Server.HTTPS.Port == 0 {
			allErrs = append(allErrs, field.Required(serverPath.Child("https", "port"), "port must be specified"))
		}
	}

	return allErrs
}
