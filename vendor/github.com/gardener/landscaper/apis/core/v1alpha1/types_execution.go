// SPDX-FileCopyrightText: 2019 SAP SE or an SAP affiliate company and Gardener contributors.
//
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// ExecutionManagedByLabel is the label of a deploy item that contains the name of the managed execution.
// This label is used by the extension controller to identify its managed deploy items
// todo: add conversion
const ExecutionManagedByLabel = "execution.landscaper.gardener.cloud/managed-by"

// ExecutionManagedNameLabel is the unique identifier of the deploy item managed by a execution.
// It corresponds to the execution item name.
// todo: add conversion
const ExecutionManagedNameLabel = "execution.landscaper.gardener.cloud/name"

// ExecutionDependsOnAnnotation is name of the annotation that holds the dependsOn data
// defined in the execution.
// This annotation is mainly to correctly cleanup orphaned deploy items that are not part of the execution anymore.
// todo: add conversion
const ExecutionDependsOnAnnotation = "execution.landscaper.gardener.cloud/dependsOn"

// ReconcileDeployItemsCondition is the Conditions type to indicate the deploy items status.
const ReconcileDeployItemsCondition ConditionType = "ReconcileDeployItems"

type ExecutionPhase string

const (
	ExecutionPhaseInit        = ExecutionPhase(ComponentPhaseInit)
	ExecutionPhaseProgressing = ExecutionPhase(ComponentPhaseProgressing)
	ExecutionPhaseDeleting    = ExecutionPhase(ComponentPhaseDeleting)
	ExecutionPhaseSucceeded   = ExecutionPhase(ComponentPhaseSucceeded)
	ExecutionPhaseFailed      = ExecutionPhase(ComponentPhaseFailed)
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ExecutionList contains a list of Executions‚
type ExecutionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Execution `json:"items"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Execution contains the configuration of a execution and deploy item
// +kubebuilder:resource:path="executions",scope="Namespaced",shortName="exec",singular="execution"
// +kubebuilder:printcolumn:JSONPath=".status.phase",name=Phase,type=string
// +kubebuilder:printcolumn:JSONPath=".status.exportRef.name",name=ExportRef,type=string
// +kubebuilder:printcolumn:JSONPath=".metadata.creationTimestamp",name=Age,type=date
// +kubebuilder:subresource:status
type Execution struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// Spec defines a execution and its items
	Spec ExecutionSpec `json:"spec"`
	// Status contains the current status of the execution.
	// +optional
	Status ExecutionStatus `json:"status"`
}

// ExecutionSpec defines a execution plan.
type ExecutionSpec struct {
	// DeployItems defines all execution items that need to be scheduled.
	DeployItems DeployItemTemplateList `json:"deployItems,omitempty"`
}

// ExecutionStatus contains the current status of a execution.
type ExecutionStatus struct {
	// Phase is the current phase of the execution .
	Phase ExecutionPhase `json:"phase,omitempty"`

	// ObservedGeneration is the most recent generation observed for this Execution.
	// It corresponds to the Execution generation, which is updated on mutation by the landscaper.
	ObservedGeneration int64 `json:"observedGeneration"`

	// Conditions contains the actual condition of a execution
	Conditions []Condition `json:"conditions,omitempty"`

	// ExportReference references the object that contains the exported values.
	// only used for operation purpose.
	// +optional
	ExportReference *ObjectReference `json:"exportRef,omitempty"`

	// DeployItemReferences contain the state of all deploy items.
	// The observed generation is here the generation of the Execution not the DeployItem.
	DeployItemReferences []VersionedNamedObjectReference `json:"deployItemRefs,omitempty"`
}

// DeployItemTemplateList is a list of deploy item templates
type DeployItemTemplateList []DeployItemTemplate

// DeployItemTemplate defines a execution element that is translated into a deploy item.
// +k8s:deepcopy-gen=true
type DeployItemTemplate struct {
	// Name is the unique name of the execution.
	Name string `json:"name"`

	// DataType is the DeployItem type of the execution.
	Type DeployItemType `json:"type"`

	// Target is the object reference to the target that the deploy item should deploy to.
	// +optional
	Target *ObjectReference `json:"target,omitempty"`

	// Labels is the map of labels to be added to the deploy item.
	// +optional
	Labels map[string]string `json:"labels,omitempty"`

	// ProviderConfiguration contains the type specific configuration for the execution.
	// +kubebuilder:validation:XEmbeddedResource
	// +kubebuilder:validation:XPreserveUnknownFields
	Configuration *runtime.RawExtension `json:"config"`

	// DependsOn lists deploy items that need to be executed before this one
	DependsOn []string `json:"dependsOn,omitempty"`
}
