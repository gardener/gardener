// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"k8s.io/utils/ptr"
)

// SetDefaults_ManagedSeedSet sets default values for ManagedSeed objects.
func SetDefaults_ManagedSeedSet(obj *ManagedSeedSet) {
	// Set default replicas
	if obj.Spec.Replicas == nil {
		obj.Spec.Replicas = ptr.To[int32](1)
	}

	// Set update strategy defaults
	if obj.Spec.UpdateStrategy == nil {
		obj.Spec.UpdateStrategy = &UpdateStrategy{}
	}

	// Set default revision history limit
	if obj.Spec.RevisionHistoryLimit == nil {
		obj.Spec.RevisionHistoryLimit = ptr.To[int32](10)
	}
}

// SetDefaults_UpdateStrategy sets default values for UpdateStrategy objects.
func SetDefaults_UpdateStrategy(obj *UpdateStrategy) {
	// Set default type
	if obj.Type == nil {
		t := RollingUpdateStrategyType
		obj.Type = &t
	}
}

// SetDefaults_RollingUpdateStrategy sets default values for RollingUpdateStrategy objects.
func SetDefaults_RollingUpdateStrategy(obj *RollingUpdateStrategy) {
	// Set default partition
	if obj.Partition == nil {
		obj.Partition = ptr.To[int32](0)
	}
}
