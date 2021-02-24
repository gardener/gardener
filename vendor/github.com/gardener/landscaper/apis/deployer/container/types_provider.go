// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors.
//
// SPDX-License-Identifier: Apache-2.0

package container

import (
	"encoding/json"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	lsv1alpha1 "github.com/gardener/landscaper/apis/core/v1alpha1"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ProviderConfiguration is the container deployer configuration that is expected in a DeployItem
type ProviderConfiguration struct {
	metav1.TypeMeta `json:",inline"`

	// Docker image name.
	// More info: https://kubernetes.io/docs/concepts/containers/images
	// The image will be defaulted by the container deployer to the configured default.
	// +optional
	Image string `json:"image,omitempty"`
	// Entrypoint array. Not executed within a shell.
	// The docker image's ENTRYPOINT is used if this is not provided.
	// Variable references $(VAR_NAME) are expanded using the container's environment. If a variable
	// cannot be resolved, the reference in the input string will be unchanged. The $(VAR_NAME) syntax
	// can be escaped with a double $$, ie: $$(VAR_NAME). Escaped references will never be expanded,
	// regardless of whether the variable exists or not.
	// Cannot be updated.
	// More info: https://kubernetes.io/docs/tasks/inject-data-application/define-command-argument-container/#running-a-command-in-a-shell
	// +optional
	Command []string `json:"command,omitempty"`
	// Arguments to the entrypoint.
	// The docker image's CMD is used if this is not provided.
	// Variable references $(VAR_NAME) are expanded using the container's environment. If a variable
	// cannot be resolved, the reference in the input string will be unchanged. The $(VAR_NAME) syntax
	// can be escaped with a double $$, ie: $$(VAR_NAME). Escaped references will never be expanded,
	// regardless of whether the variable exists or not.
	// Cannot be updated.
	// More info: https://kubernetes.io/docs/tasks/inject-data-application/define-command-argument-container/#running-a-command-in-a-shell
	// +optional
	Args []string `json:"args,omitempty"`
	// ImportValues contains the import values for the container.
	// +optional
	ImportValues json.RawMessage `json:"importValues,omitempty"`
	// Blueprint is the resolved reference to the Blueprint definition
	// +optional
	Blueprint *lsv1alpha1.BlueprintDefinition `json:"blueprint,omitempty"`
	// ComponentDescriptor is the resolved reference to the ComponentDescriptor defnition
	// +optional
	ComponentDescriptor *lsv1alpha1.ComponentDescriptorDefinition `json:"componentDescriptor,omitempty"`
	// RegistryPullSecrets defines a list of registry credentials that are used to
	// pull blueprints and component descriptors from the respective registry.
	// For more info see: https://kubernetes.io/docs/tasks/configure-pod-container/pull-image-private-registry/
	// Note that the type information is used to determine the secret key and the type of the secret.
	// +optional
	RegistryPullSecrets []lsv1alpha1.ObjectReference `json:"registryPullSecrets,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// ProviderStatus is the container provider specific status
type ProviderStatus struct {
	metav1.TypeMeta `json:",inline"`
	// LastOperation defines the last run operation of the pod.
	// The operation can be either reconcile or deletion.
	LastOperation string `json:"lastOperation"`
	// PodStatus indicated the status of the executed pod.
	PodStatus *PodStatus `json:"podStatus,omitempty"`
}

// PodStatus describes the status of a pod with its init, wait and main container
type PodStatus struct {
	// PodName is the name of the created pod.
	PodName string `json:"podName"`
	// LastRun is the time when the pod was executed the last time.
	LastRun *metav1.Time `json:"lastRun,omitempty"`
	// ContainerStatus contains the status of the pod that
	// executes the configured container.
	ContainerStatus ContainerStatus `json:"containerStatus"`
	// InitContainerStatus contains the status of the init container.
	InitContainerStatus ContainerStatus `json:"initContainerStatus"`
	// WaitContainerStatus contains the status of the wait container.
	WaitContainerStatus ContainerStatus `json:"waitContainerStatus"`
}

// ContainerStatus describes the status of a pod with its init, wait and main container.
type ContainerStatus struct {
	// A human readable message indicating details about why the pod is in this condition.
	// +optional
	Message string `json:"message,omitempty"`
	// A brief CamelCase message indicating details about why the pod is in this state.
	// e.g. 'Evicted'
	// +optional
	Reason string `json:"reason,omitempty"`
	// Details about the container's current condition.
	// +optional
	State corev1.ContainerState `json:"state,omitempty"`
	// The image the container is running.
	// More info: https://kubernetes.io/docs/concepts/containers/images
	Image string `json:"image"`
	// ImageID of the container's image.
	ImageID string `json:"imageID"`
	// ExitCode of the main container.
	// +optional
	ExitCode *int32 `json:"exitCode,omitempty"`
}
