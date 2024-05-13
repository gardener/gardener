// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:scope=Cluster,shortName="ext"
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`,description="creation timestamp"

// Extension describes a Gardener extension.
type Extension struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object metadata.
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// Spec contains the specification of this extension.
	Spec ExtensionSpec `json:"spec,omitempty"`
	// Status contains the status of this extension.
	Status ExtensionStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ExtensionList is a list of Extension resources.
type ExtensionList struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`

	// Items is the list of Extension.
	Items []Extension `json:"items"`
}

// ExtensionSpec contains the specification of a Gardener extension.
type ExtensionSpec struct {
	// Resources is a list of combinations of kinds (DNSRecord, Backupbucket, ...) and their actual types
	// (aws-route53, gcp).
	// +optional
	Resources []gardencorev1beta1.ControllerResource `json:"resources,omitempty"`
	// Deployment contains deployment configuration for the admission and extension concept.
	// +optional
	Deployment *DeploymentSpec `json:"deployment,omitempty"`
}

// Deployment contains deployment configuration for the admission and extension concept.
type Deployment struct {
	// Admission contains the deployment specification for the extension admission controller.
	// +optional
	Admission *DeploymentSpec `json:"admission,omitempty"`
	// Extension contains the deployment specification for the extension.
	// +optional
	Extension *ExtensionDeploymentSpec `json:"extension,omitempty"`
}

// DeploymentSpec is the specification for the deployment of a component.
type DeploymentSpec struct {
	// Helm is the Helm deployment configuration.
	Helm *Helm `json:"helm,omitempty"`
}

// Helm is the Helm deployment configuration.
type Helm struct {
	// OCIRepository is the configuration of to the OCI repository.
	OCIRepository string `json:"ociRepository"`
	// RawChart is the base64-encoded, gzip'ed, tar'ed Helm chart.
	// +optional
	RawChart []byte `json:"rawChart,omitempty"`
	// Values are the chart values.
	// +optional
	Values *runtime.RawExtension `json:"values,omitempty"`
}

// ExtensionDeploymentSpec contains the deployment specification for an extension.
type ExtensionDeploymentSpec struct {
	DeploymentSpec `json:",inline"`
	// Policy controls how the controller is deployed. It defaults to 'OnDemand'.
	// +optional
	Policy *gardencorev1beta1.ControllerDeploymentPolicy `json:"policy,omitempty"`
}

// ExtensionStatus is the status of a Gardener extension.
type ExtensionStatus struct {
	// ObservedGeneration is the most recent generation observed for this resource.
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// Conditions represents the latest available observations of an Extension's current state.
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +optional
	Conditions []gardencorev1beta1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
	// ProviderStatus contains type-specific status.
	// +optional
	ProviderStatus *runtime.RawExtension `json:"providerStatus,omitempty"`
}
