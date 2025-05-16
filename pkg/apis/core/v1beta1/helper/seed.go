// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package helper

import (
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/utils/ptr"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

// TaintsHave returns true if the given key is part of the taints list.
func TaintsHave(taints []gardencorev1beta1.SeedTaint, key string) bool {
	for _, taint := range taints {
		if taint.Key == key {
			return true
		}
	}
	return false
}

// TaintsAreTolerated returns true when all the given taints are tolerated by the given tolerations.
func TaintsAreTolerated(taints []gardencorev1beta1.SeedTaint, tolerations []gardencorev1beta1.Toleration) bool {
	if len(taints) == 0 {
		return true
	}
	if len(taints) > len(tolerations) {
		return false
	}

	tolerationKeyValues := make(map[string]string, len(tolerations))
	for _, toleration := range tolerations {
		v := ""
		if toleration.Value != nil {
			v = *toleration.Value
		}
		tolerationKeyValues[toleration.Key] = v
	}

	for _, taint := range taints {
		tolerationValue, ok := tolerationKeyValues[taint.Key]
		if !ok {
			return false
		}
		if taint.Value != nil && *taint.Value != tolerationValue {
			return false
		}
	}

	return true
}

// SeedSettingExcessCapacityReservationEnabled returns true if the 'excess capacity reservation' setting is enabled.
func SeedSettingExcessCapacityReservationEnabled(settings *gardencorev1beta1.SeedSettings) bool {
	return settings == nil || settings.ExcessCapacityReservation == nil || ptr.Deref(settings.ExcessCapacityReservation.Enabled, true)
}

// SeedSettingVerticalPodAutoscalerEnabled returns true if the 'verticalPodAutoscaler' setting is enabled.
func SeedSettingVerticalPodAutoscalerEnabled(settings *gardencorev1beta1.SeedSettings) bool {
	return settings == nil || settings.VerticalPodAutoscaler == nil || settings.VerticalPodAutoscaler.Enabled
}

// SeedSettingDependencyWatchdogWeederEnabled returns true if the dependency-watchdog-weeder is enabled.
func SeedSettingDependencyWatchdogWeederEnabled(settings *gardencorev1beta1.SeedSettings) bool {
	return settings == nil || settings.DependencyWatchdog == nil || settings.DependencyWatchdog.Weeder == nil || settings.DependencyWatchdog.Weeder.Enabled
}

// SeedSettingDependencyWatchdogProberEnabled returns true if the dependency-watchdog-prober is enabled.
func SeedSettingDependencyWatchdogProberEnabled(settings *gardencorev1beta1.SeedSettings) bool {
	return settings == nil || settings.DependencyWatchdog == nil || settings.DependencyWatchdog.Prober == nil || settings.DependencyWatchdog.Prober.Enabled
}

// SeedSettingTopologyAwareRoutingEnabled returns true if the topology-aware routing is enabled.
func SeedSettingTopologyAwareRoutingEnabled(settings *gardencorev1beta1.SeedSettings) bool {
	return settings != nil && settings.TopologyAwareRouting != nil && settings.TopologyAwareRouting.Enabled
}

// SeedBackupCredentialsRefEqual returns true when the credentials reference of the backup configuration is the same.
func SeedBackupCredentialsRefEqual(oldBackup, newBackup *gardencorev1beta1.Backup) bool {
	var (
		oldCredentialsRef *corev1.ObjectReference
		newCredentialsRef *corev1.ObjectReference
	)

	if oldBackup != nil {
		oldCredentialsRef = oldBackup.CredentialsRef
	}

	if newBackup != nil {
		newCredentialsRef = newBackup.CredentialsRef
	}

	return apiequality.Semantic.DeepEqual(oldCredentialsRef, newCredentialsRef)
}

// CalculateSeedUsage returns a map representing the number of shoots per seed from the given list of shoots.
// It takes both spec.seedName and status.seedName into account.
func CalculateSeedUsage(shootList []*gardencorev1beta1.Shoot) map[string]int {
	m := map[string]int{}

	for _, shoot := range shootList {
		var (
			specSeed   = ptr.Deref(shoot.Spec.SeedName, "")
			statusSeed = ptr.Deref(shoot.Status.SeedName, "")
		)

		if specSeed != "" {
			m[specSeed]++
		}
		if statusSeed != "" && specSeed != statusSeed {
			m[statusSeed]++
		}
	}

	return m
}
