// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package v1beta1

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/utils/ptr"
)

// SetDefaults_Seed sets default values for Seed objects.
func SetDefaults_Seed(obj *Seed) {
	// Use an explicit, non-generated defaulter for the SeedSpec so it can be reused in the gardenlet.
	DefaultSeedSpec(&obj.Spec)
}

// DefaultSeedSpec sets default values for SeedSpec objects.
func DefaultSeedSpec(obj *SeedSpec) {
	if obj.Settings == nil {
		obj.Settings = &SeedSettings{}
	}
	setDefaults_SeedNetworks(&obj.Networks)
	setDefaults_SeedSettings(obj.Settings)
}

// setDefaults_SeedSettings sets default values for SeedSettings objects.
func setDefaults_SeedSettings(obj *SeedSettings) {
	if obj.ExcessCapacityReservation == nil {
		obj.ExcessCapacityReservation = &SeedSettingExcessCapacityReservation{}
		setDefaults_ExcessCapacityReservationConfig(obj.ExcessCapacityReservation)
	}

	if ptr.Deref(obj.ExcessCapacityReservation.Enabled, true) && len(obj.ExcessCapacityReservation.Configs) == 0 {
		setDefaults_ExcessCapacityReservationConfig(obj.ExcessCapacityReservation)
	}

	if obj.Scheduling == nil {
		obj.Scheduling = &SeedSettingScheduling{Visible: true}
	}

	if obj.LoadBalancerServices == nil {
		obj.LoadBalancerServices = &SeedSettingLoadBalancerServices{}
	}
	setDefaults_LoadBalancerServices(obj.LoadBalancerServices)

	if obj.VerticalPodAutoscaler == nil {
		obj.VerticalPodAutoscaler = &SeedSettingVerticalPodAutoscaler{Enabled: true}
	}

	if obj.PersistentVolumeClaimAutoscaler == nil {
		obj.PersistentVolumeClaimAutoscaler = &SeedSettingPersistentVolumeClaimAutoscaler{Enabled: false}
	}

	if obj.DependencyWatchdog == nil {
		obj.DependencyWatchdog = &SeedSettingDependencyWatchdog{}
	}
	setDefaults_DependencyWatchdog(obj.DependencyWatchdog)

	if obj.TopologyAwareRouting == nil {
		obj.TopologyAwareRouting = &SeedSettingTopologyAwareRouting{Enabled: false}
	}
}

// setDefaults_SeedNetworks sets default values for SeedNetworks objects.
func setDefaults_SeedNetworks(obj *SeedNetworks) {
	if len(obj.IPFamilies) == 0 {
		obj.IPFamilies = []IPFamily{IPFamilyIPv4}
	}
}

// setDefaults_SeedSettingDependencyWatchdog sets defaults for SeedSettingDependencyWatchdog objects.
func setDefaults_DependencyWatchdog(obj *SeedSettingDependencyWatchdog) {
	if obj.Weeder == nil {
		obj.Weeder = &SeedSettingDependencyWatchdogWeeder{Enabled: true}
	}

	if obj.Prober == nil {
		obj.Prober = &SeedSettingDependencyWatchdogProber{Enabled: true}
	}
}

// setDefaults_SeedSettingLoadBalancerServices sets defaults for SeedSettingLoadBalancerServices objects.
func setDefaults_LoadBalancerServices(obj *SeedSettingLoadBalancerServices) {
	if obj.ZonalIngress == nil {
		obj.ZonalIngress = &SeedSettingLoadBalancerServicesZonalIngress{Enabled: new(true)}
	}
}

func setDefaults_ExcessCapacityReservationConfig(excessCapacityReservation *SeedSettingExcessCapacityReservation) {
	excessCapacityReservation.Configs = []SeedSettingExcessCapacityReservationConfig{
		// This roughly corresponds to a single, moderately large control-plane.
		{
			Resources: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("2"),
				corev1.ResourceMemory: resource.MustParse("6Gi"),
			},
		},
	}
}
