// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package v1beta1

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/utils/pointer"
)

// SetDefaults_Seed sets default values for Seed objects.
func SetDefaults_Seed(obj *Seed) {
	if obj.Spec.Settings == nil {
		obj.Spec.Settings = &SeedSettings{}
	}
}

// SetDefaults_SeedSettings sets default values for SeedSettings objects.
func SetDefaults_SeedSettings(obj *SeedSettings) {
	if obj.ExcessCapacityReservation == nil {
		obj.ExcessCapacityReservation = &SeedSettingExcessCapacityReservation{}
		setDefaults_ExcessCapacityReservationConfig(obj.ExcessCapacityReservation)
	}

	if pointer.BoolDeref(obj.ExcessCapacityReservation.Enabled, true) && len(obj.ExcessCapacityReservation.Configs) == 0 {
		setDefaults_ExcessCapacityReservationConfig(obj.ExcessCapacityReservation)
	}

	if obj.Scheduling == nil {
		obj.Scheduling = &SeedSettingScheduling{Visible: true}
	}

	if obj.VerticalPodAutoscaler == nil {
		obj.VerticalPodAutoscaler = &SeedSettingVerticalPodAutoscaler{Enabled: true}
	}

	if obj.DependencyWatchdog == nil {
		obj.DependencyWatchdog = &SeedSettingDependencyWatchdog{}
	}

	if obj.TopologyAwareRouting == nil {
		obj.TopologyAwareRouting = &SeedSettingTopologyAwareRouting{Enabled: false}
	}
}

// SetDefaults_SeedNetworks sets default values for SeedNetworks objects.
func SetDefaults_SeedNetworks(obj *SeedNetworks) {
	if len(obj.IPFamilies) == 0 {
		obj.IPFamilies = []IPFamily{IPFamilyIPv4}
	}
}

// SetDefaults_SeedSettingDependencyWatchdog sets defaults for SeedSettingDependencyWatchdog objects.
func SetDefaults_SeedSettingDependencyWatchdog(obj *SeedSettingDependencyWatchdog) {
	if obj.Weeder == nil {
		obj.Weeder = &SeedSettingDependencyWatchdogWeeder{Enabled: true}
	}
	if obj.Prober == nil {
		obj.Prober = &SeedSettingDependencyWatchdogProber{Enabled: true}
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
