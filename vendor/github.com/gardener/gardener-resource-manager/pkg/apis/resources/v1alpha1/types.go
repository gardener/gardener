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

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ManagedResource describes a list of managed resources.
type ManagedResource struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object metadata.
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// Spec contains the specification of this managed resource.
	Spec ManagedResourceSpec `json:"spec,omitempty"`
	// Status contains the status of this managed resource.
	Status ManagedResourceStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ManagedResourceList is a list of ManagedResource resources.
type ManagedResourceList struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`

	// Items is the list of ManagedResource.
	Items []ManagedResource `json:"items"`
}

type ManagedResourceSpec struct {
	// SecretRefs is a list of secret references.
	SecretRefs []corev1.LocalObjectReference `json:"secretRefs"`
	// InjectLabels injects the provided labels into every resource that is part of the referenced secrets.
	// +optional
	InjectLabels map[string]string `json:"injectLabels,omitempty"`
	// ForceOverwriteLabels specifies that all existing labels should be overwritten. Defaults to false.
	// +optional
	ForceOverwriteLabels *bool `json:"forceOverwriteLabels,omitempty"`
	// ForceOverwriteAnnotations specifies that all existing annotations should be overwritten. Defaults to false.
	// +optional
	ForceOverwriteAnnotations *bool `json:"forceOverwriteAnnotations,omitempty"`
	// KeepObjects specifies whether the objects should be kept although the managed resource has already been deleted.
	// Defaults to false.
	// +optional
	KeepObjects *bool `json:"keepObjects,omitempty"`
}

// ManagedResourceStatus is the status of a managed resource.
type ManagedResourceStatus struct {
	// ObservedGeneration is the most recent generation observed for this resource.
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// Resources is a list of objects that have been created.
	// +optional
	Resources []ObjectReference `json:"resources,omitempty"`
}

type ObjectReference struct {
	corev1.ObjectReference `json:",inline"`
	// Labels is a map of labels that were used during last update of the resource.
	Labels map[string]string `json:"labels,omitempty"`
	// Annotations is a map of annotations that were used during last update of the resource.
	Annotations map[string]string `json:"annotations,omitempty"`
}
