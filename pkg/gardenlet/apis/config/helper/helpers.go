// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package helper

import (
	"errors"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	gardencore "github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
)

// SeedNameFromSeedConfig returns an empty string if the given seed config is nil, or the
// name inside the seed config.
func SeedNameFromSeedConfig(seedConfig *config.SeedConfig) string {
	if seedConfig == nil {
		return ""
	}
	return seedConfig.SeedTemplate.Name
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
	utilruntime.Must(gardenletconfigv1alpha1.AddToScheme(scheme))
}

// ConvertGardenletConfiguration converts the given object to an internal GardenletConfiguration version.
func ConvertGardenletConfiguration(obj runtime.Object) (*config.GardenletConfiguration, error) {
	obj, err := scheme.ConvertToVersion(obj, config.SchemeGroupVersion)
	if err != nil {
		return nil, err
	}
	result, ok := obj.(*config.GardenletConfiguration)
	if !ok {
		return nil, errors.New("could not convert GardenletConfiguration to internal version")
	}
	return result, nil
}

// ConvertGardenletConfigurationExternal converts the given object to an external  GardenletConfiguration version.
func ConvertGardenletConfigurationExternal(obj runtime.Object) (*gardenletconfigv1alpha1.GardenletConfiguration, error) {
	obj, err := scheme.ConvertToVersion(obj, gardenletconfigv1alpha1.SchemeGroupVersion)
	if err != nil {
		return nil, err
	}
	result, ok := obj.(*gardenletconfigv1alpha1.GardenletConfiguration)
	if !ok {
		return nil, errors.New("could not convert GardenletConfiguration to version " + gardenletconfigv1alpha1.SchemeGroupVersion.String())
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
