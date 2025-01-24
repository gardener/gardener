// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package helper

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
)

// SeedNameFromSeedConfig returns an empty string if the given seed config is nil, or the
// name inside the seed config.
func SeedNameFromSeedConfig(seedConfig *gardenletconfigv1alpha1.SeedConfig) string {
	if seedConfig == nil {
		return ""
	}
	return seedConfig.SeedTemplate.Name
}

// StaleExtensionHealthChecksThreshold returns nil if the given config is nil or the check
// for stale health checks is not enabled. Otherwise it returns the threshold from the given config.
func StaleExtensionHealthChecksThreshold(c *gardenletconfigv1alpha1.StaleExtensionHealthChecks) *metav1.Duration {
	if c != nil && c.Enabled {
		return c.Threshold
	}

	return nil
}

// IsLoggingEnabled return true if the logging stack for clusters is enabled.
func IsLoggingEnabled(c *gardenletconfigv1alpha1.GardenletConfiguration) bool {
	if c != nil && c.Logging != nil &&
		c.Logging.Enabled != nil {
		return *c.Logging.Enabled
	}
	return false
}

// IsValiEnabled return true if the vali is enabled
func IsValiEnabled(c *gardenletconfigv1alpha1.GardenletConfiguration) bool {
	if c != nil && c.Logging != nil &&
		c.Logging.Vali != nil && c.Logging.Vali.Enabled != nil {
		return *c.Logging.Vali.Enabled
	}
	return true
}

// IsEventLoggingEnabled returns true if the event-logging is enabled.
func IsEventLoggingEnabled(c *gardenletconfigv1alpha1.GardenletConfiguration) bool {
	return c != nil && c.Logging != nil &&
		c.Logging.ShootEventLogging != nil &&
		c.Logging.ShootEventLogging.Enabled != nil &&
		*c.Logging.ShootEventLogging.Enabled
}

// IsMonitoringEnabled returns true if the monitoring stack for shoot clusters is enabled. Default is enabled.
func IsMonitoringEnabled(c *gardenletconfigv1alpha1.GardenletConfiguration) bool {
	if c != nil && c.Monitoring != nil && c.Monitoring.Shoot != nil &&
		c.Monitoring.Shoot.Enabled != nil {
		return *c.Monitoring.Shoot.Enabled
	}
	return true
}

// GetManagedResourceProgressingThreshold returns ManagedResourceProgressingThreshold if set otherwise it returns nil.
func GetManagedResourceProgressingThreshold(c *gardenletconfigv1alpha1.GardenletConfiguration) *metav1.Duration {
	if c != nil && c.Controllers != nil && c.Controllers.ShootCare != nil && c.Controllers.ShootCare.ManagedResourceProgressingThreshold != nil {
		return c.Controllers.ShootCare.ManagedResourceProgressingThreshold
	}
	return nil
}
