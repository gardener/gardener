// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors.
//
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/selection"
)

// DeployItemValidationCondition is the Conditions type to indicate the deploy items configuration validation status.
const DeployItemValidationCondition ConditionType = "DeployItemValidation"

// DeployItemType defines the type of the deploy item
type DeployItemType string

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// DeployItemList contains a list of DeployItems
type DeployItemList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DeployItem `json:"items"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// DeployItem defines a resource that should be processed by a external deployer
// +kubebuilder:resource:path="deployitems",scope="Namespaced",shortName="di",singular="deployitem"
// +kubebuilder:printcolumn:JSONPath=".spec.type",name=Type,type=string
// +kubebuilder:printcolumn:JSONPath=".status.phase",name=Phase,type=string
// +kubebuilder:printcolumn:JSONPath=".status.exportRef.name",name=ExportRef,type=string
// +kubebuilder:printcolumn:JSONPath=".metadata.creationTimestamp",name=Age,type=date
// +kubebuilder:subresource:status
type DeployItem struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec DeployItemSpec `json:"spec"`

	// +optional
	Status DeployItemStatus `json:"status"`
}

// DeployItemSpec contains the definition of a deploy item.
type DeployItemSpec struct {
	// Type is the type of the deployer that should handle the item.
	Type DeployItemType `json:"type"`
	// Target specifies an optional target of the deploy item.
	// In most cases it contains the secrets to access a evironment.
	// It is also used by the deployers to determine the ownernship.
	// +optional
	Target *ObjectReference `json:"target,omitempty"`
	// Configuration contains the deployer type specific configuration.
	// +kubebuilder:validation:XEmbeddedResource
	// +kubebuilder:validation:XPreserveUnknownFields
	Configuration *runtime.RawExtension `json:"config,omitempty"`
}

// DeployItemStatus contains the status of a deploy item.
// todo: add operation
type DeployItemStatus struct {
	// Phase is the current phase of the DeployItem
	Phase ExecutionPhase `json:"phase,omitempty"`

	// ObservedGeneration is the most recent generation observed for this DeployItem.
	// It corresponds to the DeployItem generation, which is updated on mutation by the landscaper.
	ObservedGeneration int64 `json:"observedGeneration"`

	// Conditions contains the actual condition of a deploy item
	// +optional
	Conditions []Condition `json:"conditions,omitempty"`

	// LastError describes the last error that occurred.
	LastError *Error `json:"lastError,omitempty"`

	// ProviderStatus contains the provider specific status
	// +optional
	// +kubebuilder:validation:XEmbeddedResource
	// +kubebuilder:validation:XPreserveUnknownFields
	ProviderStatus *runtime.RawExtension `json:"providerStatus,omitempty"`

	// ExportReference is the reference to the object that contains the exported values.
	// +optional
	ExportReference *ObjectReference `json:"exportRef,omitempty"`
}

// TargetSelector describes a selector that matches specific targets.
// +k8s:deepcopy-gen=true
type TargetSelector struct {
	// Annotations matches a target based on annotations.
	// +optional
	Annotations []Requirement `json:"annotations,omitempty"`
}

// Requirement contains values, a key, and an operator that relates the key and values.
// The zero value of Requirement is invalid.
// Requirement implements both set based match and exact match
// Requirement should be initialized via NewRequirement constructor for creating a valid Requirement.
// +k8s:deepcopy-gen=true
type Requirement struct {
	Key      string             `json:"key"`
	Operator selection.Operator `json:"operator"`
	// In huge majority of cases we have at most one value here.
	// It is generally faster to operate on a single-element slice
	// than on a single-element map, so we have a slice here.
	// +optional
	Values []string `json:"values,omitempty"`
}
