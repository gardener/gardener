// Copyright 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package helper

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	gardencore "github.com/gardener/gardener/pkg/apis/core"
	gardencorehelper "github.com/gardener/gardener/pkg/apis/core/helper"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	gardenletv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
)

// SeedNameFromSeedConfig returns an empty string if the given seed config is nil, or the
// name inside the seed config.
func SeedNameFromSeedConfig(seedConfig *config.SeedConfig) string {
	if seedConfig == nil {
		return ""
	}
	return seedConfig.SeedTemplate.Name
}

// OwnerChecksEnabledInSeedConfig returns false if the given seed config is nil or the 'ownerChecks' setting is enabled.
func OwnerChecksEnabledInSeedConfig(seedConfig *config.SeedConfig) bool {
	if seedConfig == nil {
		return false
	}
	return gardencorehelper.SeedSettingOwnerChecksEnabled(seedConfig.Spec.Settings)
}

// StaleExtensionHealthChecksThreshold returns nil if the given config is nil or the check
// for stale health checks is not enabled. Otherwise it returns the threshold from the given config.
func StaleExtensionHealthChecksThreshold(c *config.StaleExtensionHealthChecks) *metav1.Duration {
	if c != nil && c.Enabled {
		return c.Threshold
	}

	return nil
}

var scheme *runtime.Scheme

func init() {
	scheme = runtime.NewScheme()
	// core schemes are needed here to properly convert the embedded SeedTemplate objects
	utilruntime.Must(gardencore.AddToScheme(scheme))
	utilruntime.Must(gardencorev1beta1.AddToScheme(scheme))
	utilruntime.Must(config.AddToScheme(scheme))
	utilruntime.Must(gardenletv1alpha1.AddToScheme(scheme))
}

// ConvertGardenletConfiguration converts the given object to an internal GardenletConfiguration version.
func ConvertGardenletConfiguration(obj runtime.Object) (*config.GardenletConfiguration, error) {
	obj, err := scheme.ConvertToVersion(obj, config.SchemeGroupVersion)
	if err != nil {
		return nil, err
	}
	result, ok := obj.(*config.GardenletConfiguration)
	if !ok {
		return nil, fmt.Errorf("could not convert GardenletConfiguration to internal version")
	}
	return result, nil
}

// ConvertGardenletConfigurationExternal converts the given object to an external  GardenletConfiguration version.
func ConvertGardenletConfigurationExternal(obj runtime.Object) (*gardenletv1alpha1.GardenletConfiguration, error) {
	obj, err := scheme.ConvertToVersion(obj, gardenletv1alpha1.SchemeGroupVersion)
	if err != nil {
		return nil, err
	}
	result, ok := obj.(*gardenletv1alpha1.GardenletConfiguration)
	if !ok {
		return nil, fmt.Errorf("could not convert GardenletConfiguration to version %s", gardenletv1alpha1.SchemeGroupVersion.String())
	}
	return result, nil
}

// IsLoggingEnabled return true if the logging stack for clusters is enabled.
func IsLoggingEnabled(c *config.GardenletConfiguration) bool {
	if c != nil && c.Logging != nil &&
		c.Logging.Enabled != nil {
		return *c.Logging.Enabled
	}
	return false
}

// IsValiEnabled return true if the vali is enabled
func IsValiEnabled(c *config.GardenletConfiguration) bool {
	if c != nil && c.Logging != nil &&
		c.Logging.Vali != nil && c.Logging.Vali.Enabled != nil {
		return *c.Logging.Vali.Enabled
	}
	return true
}

// IsEventLoggingEnabled returns true if the event-logging is enabled.
func IsEventLoggingEnabled(c *config.GardenletConfiguration) bool {
	return c != nil && c.Logging != nil &&
		c.Logging.ShootEventLogging != nil &&
		c.Logging.ShootEventLogging.Enabled != nil &&
		*c.Logging.ShootEventLogging.Enabled
}

// IsMonitoringEnabled returns true if the monitoring stack for shoot clusters is enabled. Default is enabled.
func IsMonitoringEnabled(c *config.GardenletConfiguration) bool {
	if c != nil && c.Monitoring != nil && c.Monitoring.Shoot != nil &&
		c.Monitoring.Shoot.Enabled != nil {
		return *c.Monitoring.Shoot.Enabled
	}
	return true
}

// GetManagedResourceProgressingThreshold returns ManagedResourceProgressingThreshold if set otherwise it returns nil.
func GetManagedResourceProgressingThreshold(c *config.GardenletConfiguration) *metav1.Duration {
	if c != nil && c.Controllers != nil && c.Controllers.ShootCare != nil && c.Controllers.ShootCare.ManagedResourceProgressingThreshold != nil {
		return c.Controllers.ShootCare.ManagedResourceProgressingThreshold
	}
	return nil
}
