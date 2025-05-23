// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ Object = (*ContainerRuntime)(nil)

const (
	// ContainerRuntimeResource is a constant for the name of the Container Runtime Extension resource.
	ContainerRuntimeResource = "ContainerRuntime"
	// CRINameWorkerLabel is the name of the label describing the CRI name used in this node.
	CRINameWorkerLabel = "worker.gardener.cloud/cri-name"
	// ContainerRuntimeNameWorkerLabel is a label describing a Container Runtime which should be supported on the node.
	ContainerRuntimeNameWorkerLabel = "containerruntime.worker.gardener.cloud/%s"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:scope=Namespaced,path=containerruntimes,shortName=cr,singular=containerruntime
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name=Type,JSONPath=".spec.type",type=string,description="The type of the Container Runtime resource."
// +kubebuilder:printcolumn:name=Status,JSONPath=".status.lastOperation.state",type=string,description="status of the last operation, one of Aborted, Processing, Succeeded, Error, Failed"
// +kubebuilder:printcolumn:name=Age,JSONPath=".metadata.creationTimestamp",type=date,description="creation timestamp"

// ContainerRuntime is a specification for a container runtime resource.
type ContainerRuntime struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// Specification of the ContainerRuntime.
	// If the object's deletion timestamp is set, this field is immutable.
	Spec ContainerRuntimeSpec `json:"spec"`
	// +optional
	Status ContainerRuntimeStatus `json:"status"`
}

// GetExtensionSpec implements Object.
func (i *ContainerRuntime) GetExtensionSpec() Spec {
	return &i.Spec
}

// GetExtensionStatus implements Object.
func (i *ContainerRuntime) GetExtensionStatus() Status {
	return &i.Status
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ContainerRuntimeList is a list of ContainerRuntime resources.
type ContainerRuntimeList struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []ContainerRuntime `json:"items"`
}

// ContainerRuntimeSpec is the spec for a ContainerRuntime resource.
type ContainerRuntimeSpec struct {
	// BinaryPath is the Worker's machine path where container runtime extensions should copy the binaries to.
	BinaryPath string `json:"binaryPath"`
	// WorkerPool identifies the worker pool of the Shoot.
	// For each worker pool and type, Gardener deploys a ContainerRuntime CRD.
	WorkerPool ContainerRuntimeWorkerPool `json:"workerPool"`
	// DefaultSpec is a structure containing common fields used by all extension resources.
	DefaultSpec `json:",inline"`
}

// ContainerRuntimeWorkerPool identifies a Shoot worker pool by its name and selector.
type ContainerRuntimeWorkerPool struct {
	// Name specifies the name of the worker pool the container runtime should be available for.
	// This field is immutable.
	Name string `json:"name"`
	// Selector is the label selector used by the extension to match the nodes belonging to the worker pool.
	Selector metav1.LabelSelector `json:"selector"`
}

// ContainerRuntimeStatus is the status for a ContainerRuntime resource.
type ContainerRuntimeStatus struct {
	// DefaultStatus is a structure containing common fields used by all extension resources.
	DefaultStatus `json:",inline"`
}
