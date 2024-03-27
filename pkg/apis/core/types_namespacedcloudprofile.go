// Copyright 2024 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package core

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// NamespacedCloudProfile represents certain properties about a provider environment.
type NamespacedCloudProfile struct {
	metav1.TypeMeta
	// Standard object metadata.
	metav1.ObjectMeta
	// Spec defines the provider environment properties.
	Spec NamespacedCloudProfileSpec
	// Most recently observed status of the NamespacedCloudProfile.
	Status NamespacedCloudProfileStatus
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// NamespacedCloudProfileList is a collection of NamespacedCloudProfiles.
type NamespacedCloudProfileList struct {
	metav1.TypeMeta
	// Standard list object metadata.
	metav1.ListMeta
	// Items is the list of NamespacedCloudProfiles.
	Items []NamespacedCloudProfile
}

// NamespacedCloudProfileSpec is the specification of a NamespacedCloudProfile.
type NamespacedCloudProfileSpec struct {
	// CABundle is a certificate bundle which will be installed onto every host machine of shoot cluster targeting this profile.
	CABundle *string
	// Kubernetes contains constraints regarding allowed values of the 'kubernetes' block in the Shoot specification.
	Kubernetes *KubernetesSettings
	// MachineImages contains constraints regarding allowed values for machine images in the Shoot specification.
	MachineImages []MachineImage
	// MachineTypes contains constraints regarding allowed values for machine types in the 'workers' block in the Shoot specification.
	MachineTypes []MachineType
	// Regions contains constraints regarding allowed values for regions and zones.
	Regions []Region
	// VolumeTypes contains constraints regarding allowed values for volume types in the 'workers' block in the Shoot specification.
	VolumeTypes []VolumeType
	// Parent contains a reference to a CloudProfile it inherits from.
	Parent CloudProfileReference
}

// NamespacedCloudProfileStatus holds the most recently observed status of the NamespacedCloudProfile.
type NamespacedCloudProfileStatus struct {
	// CloudProfileSpec is the most recently generated CloudProfileSpec of the NamespacedCloudProfile.
	CloudProfileSpec CloudProfileSpec
	// ObservedGeneration is the most recent generation observed for this NamespacedCloudProfile.
	ObservedGeneration int64
}

// CloudProfileReference holds the information about the parent of the NamespacedCloudProfile.
type CloudProfileReference struct {
	// Kind contains a CloudProfile kind.
	Kind string
	// Name contains the name of the referenced CloudProfile.
	Name string
}
