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

package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime"
)

func addDefaultingFuncs(scheme *runtime.Scheme) error {
	return RegisterDefaults(scheme)
}

// SetDefaults_Seed sets default values for Seed objects.
func SetDefaults_Seed(obj *Seed) {
	if obj.Spec.Settings == nil {
		obj.Spec.Settings = &SeedSettings{}
	}

	if obj.Spec.Settings.ExcessCapacityReservation == nil {
		obj.Spec.Settings.ExcessCapacityReservation = &SeedSettingExcessCapacityReservation{Enabled: true}
	}

	if obj.Spec.Settings.Scheduling == nil {
		obj.Spec.Settings.Scheduling = &SeedSettingScheduling{Visible: true}
	}

	if obj.Spec.Settings.VerticalPodAutoscaler == nil {
		obj.Spec.Settings.VerticalPodAutoscaler = &SeedSettingVerticalPodAutoscaler{Enabled: true}
	}

	if obj.Spec.Settings.OwnerChecks == nil {
		obj.Spec.Settings.OwnerChecks = &SeedSettingOwnerChecks{Enabled: false}
	}

	if obj.Spec.Settings.DependencyWatchdog == nil {
		obj.Spec.Settings.DependencyWatchdog = &SeedSettingDependencyWatchdog{}
	}

	if obj.Spec.Settings.TopologyAwareRouting == nil {
		obj.Spec.Settings.TopologyAwareRouting = &SeedSettingTopologyAwareRouting{Enabled: false}
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
}
