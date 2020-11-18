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
	"fmt"

	"github.com/gardener/gardener/pkg/gardenlet/apis/config"

	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// ValidateGardenletConfiguration validates a GardenletConfiguration object.
func ValidateGardenletConfiguration(cfg *config.GardenletConfiguration) field.ErrorList {
	allErrs := field.ErrorList{}

	if cfg.Controllers != nil {
		if cfg.Controllers.Shoot != nil {
			allErrs = append(allErrs, ValidateShootControllerConfiguration(cfg.Controllers.Shoot, field.NewPath("controllers", "shoot"))...)
		}
	}

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

	sniPath := field.NewPath("sni")

	if cfg.SNI == nil {
		allErrs = append(allErrs, field.Required(sniPath, "required configuration for SNI"))
	} else {
		allErrs = append(allErrs, validateSNI(sniPath, cfg.SNI)...)
	}

	resourcesPath := field.NewPath("resources")
	if cfg.Resources != nil {
		for resourceName, quantity := range cfg.Resources.Capacity {
			if reservedQuantity, ok := cfg.Resources.Reserved[resourceName]; ok && reservedQuantity.Value() > quantity.Value() {
				allErrs = append(allErrs, field.Invalid(resourcesPath.Child("reserved", string(resourceName)), cfg.Resources.Reserved[resourceName], "must be lower or equal to capacity"))
			}
		}
		for resourceName := range cfg.Resources.Reserved {
			if _, ok := cfg.Resources.Capacity[resourceName]; !ok {
				allErrs = append(allErrs, field.Invalid(resourcesPath.Child("reserved", string(resourceName)), cfg.Resources.Reserved[resourceName], "reserved without capacity"))
			}
		}
	}

	return allErrs
}

func validateSNI(sniPath *field.Path, sni *config.SNI) field.ErrorList {
	allErrs := field.ErrorList{}

	ingressPath := sniPath.Child("ingress")

	if sni.Ingress == nil {
		allErrs = append(allErrs, field.Required(ingressPath, "required configuration for SNI ingress"))
	} else {
		if len(sni.Ingress.Labels) == 0 {
			allErrs = append(allErrs, field.Required(ingressPath.Child("labels"), "must specify ingress gateway labels"))
		}
		if sni.Ingress.Namespace == nil || *sni.Ingress.Namespace == "" {
			allErrs = append(allErrs, field.Required(ingressPath.Child("namespace"), "must specify ingress gateway namespace"))
		}
		if sni.Ingress.ServiceName == nil || *sni.Ingress.ServiceName == "" {
			allErrs = append(allErrs, field.Required(ingressPath.Child("serviceName"), "must specify ingress gateway service name"))
		}
	}

	return allErrs
}

// ValidateShootControllerConfiguration validates the shoot controller configuration.
func ValidateShootControllerConfiguration(cfg *config.ShootControllerConfiguration, fldPath *field.Path) field.ErrorList {
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
