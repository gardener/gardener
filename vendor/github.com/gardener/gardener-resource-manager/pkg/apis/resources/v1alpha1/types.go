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

// ResourceManagerIgnoreAnnotation is an annotation that dictates whether a resources should be ignored during reconciliation.
const ResourceManagerIgnoreAnnotation = "resources.gardener.cloud/ignore"

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
	// Class holds the resource class used to control the responsibility for multiple resource manager instances
	// +optional
	Class *string `json:"class,omitempty"`
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
	// Equivalences specifies possible group/kind equivalences for objects.
	// +optional
	Equivalences [][]metav1.GroupKind `json:"equivalences,omitempty"`
	// DeletePersistentVolumeClaims specifies if PersistentVolumeClaims created by StatefulSets, which are managed by this
	// resource, should also be deleted when the corresponding StatefulSet is deleted (defaults to false).
	// +optional
	DeletePersistentVolumeClaims *bool `json:"deletePersistentVolumeClaims,omitempty"`
}

// ManagedResourceStatus is the status of a managed resource.
type ManagedResourceStatus struct {
	Conditions []ManagedResourceCondition `json:"conditions,omitempty"`
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

// ConditionType is the type of a condition.
type ConditionType string

const (
	// ResourcesApplied is a condition type that indicates whether all resources are applied to the target cluster.
	ResourcesApplied ConditionType = "ResourcesApplied"
	// ResourcesHealthy is a condition type that indicates whether all resources are present and healthy.
	ResourcesHealthy ConditionType = "ResourcesHealthy"
)

// ConditionStatus is the status of a condition.
type ConditionStatus string

// These are valid condition statuses.
const (
	// ConditionTrue means a resource is in the condition.
	ConditionTrue ConditionStatus = "True"
	// ConditionFalse means a resource is not in the condition.
	ConditionFalse ConditionStatus = "False"
	// ConditionUnknown means that the controller can't decide if a resource is in the condition or not
	ConditionUnknown ConditionStatus = "Unknown"
	// ConditionProgressing means that the controller is currently acting on the resource and the condition is therefore progressing.
	ConditionProgressing ConditionStatus = "Progressing"
)

// These are well-known reasons for ManagedResourceConditions.
const (
	// ConditionApplySucceeded indicates that the `ResourcesApplied` condition is `True`,
	// because all resources have been applied successfully.
	ConditionApplySucceeded = "ApplySucceeded"
	// ConditionApplyFailed indicates that the `ResourcesApplied` condition is `False`,
	// because applying the resources failed.
	ConditionApplyFailed = "ApplyFailed"
	// ConditionDecodingFailed indicates that the `ResourcesApplied` condition is `False`,
	// because decoding the resources of the ManagedResource failed.
	ConditionDecodingFailed = "DecodingFailed"
	// ConditionApplyProgressing indicates that the `ResourcesApplied` condition is `Progressing`,
	// because the resources are currently being reconciled.
	ConditionApplyProgressing = "ApplyProgressing"
	// ConditionDeletionFailed indicates that the `ResourcesApplied` condition is `False`,
	// because deleting the resources failed.
	ConditionDeletionFailed = "DeletionFailed"
	// ConditionDeletionPending indicates that the `ResourcesApplied` condition is `Progressing`,
	// because the deletion of some resources are still pending.
	ConditionDeletionPending = "DeletionPending"
	// ConditionHealthChecksPending indicates that the `ResourcesHealthy` condition is `Unknown`,
	// because the health checks have not been completely executed yet for the current set of resources.
	ConditionHealthChecksPending = "HealthChecksPending"
)

// ManagedResourceCondition describes the state of a deployment at a certain period.
type ManagedResourceCondition struct {
	// Type of the ManagedResource condition.
	Type ConditionType `json:"type"`
	// Status of the ManagedResource condition.
	Status ConditionStatus `json:"status"`
	// Last time the condition was updated.
	LastUpdateTime metav1.Time `json:"lastUpdateTime"`
	// Last time the condition transitioned from one status to another.
	LastTransitionTime metav1.Time `json:"lastTransitionTime"`
	// The reason for the condition's last transition.
	Reason string `json:"reason"`
	// A human readable message indicating details about the transition.
	Message string `json:"message"`
}
