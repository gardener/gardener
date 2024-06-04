// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	gardencorev1 "github.com/gardener/gardener/pkg/apis/core/v1"
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
	// ExtensionDeployment contains deployment configuration for the admission and extension concept.
	// +optional
	ExtensionDeployment *ExtensionDeploymentSpec `json:"extensionDeployment,omitempty"`
	// AdmissionDeployment contains deployment configuration for the admission and extension concept.
	// +optional
	AdmissionDeployment *AdmissionDeploymentSpec `json:"admissionDeployment,omitempty"`
}

// ExtensionDeploymentSpec contains the deployment specification for an extension.
type ExtensionDeploymentSpec struct {
	// RuntimeDeployment is the deployment configuration for the extension in the runtime cluster.
	// The deployment controls the extension behavior for the purpose of managing infrastructure resources
	// of the runtime cluster.
	// +optional
	RuntimeDeployment *DeploymentSpec `json:"runtimeDeployment,omitempty"`
	// GardenDeployment is the deployment configuration for the extension deployment in the garden cluster.
	// It controls the creation of the ControllerDeployment created in the garden virtual cluster and control how the
	// extensions operate in a seed cluster.
	// +optional
	GardenDeployment *DeploymentSpec `json:"gardenDeployment,omitempty"`
	// Policy controls how the controller is deployed. It defaults to 'OnDemand'.
	// +optional
	Policy *gardencorev1beta1.ControllerDeploymentPolicy `json:"policy,omitempty"`
}

// AdmissionDeploymentSpec contains the deployment specification for the admission controller of an extension.
type AdmissionDeploymentSpec struct {
	// RuntimeDeployment is the deployment configuration for the admission in the runtime cluster. The runtime deployment
	// is responsible for creating the admission controller in the runtime cluster.
	// +optional
	RuntimeDeployment *DeploymentSpec `json:"runtimeDeployment,omitempty"`
	// GardenDeployment is the deployment configuration for the admission deployment in the garden cluster. The garden deployment
	// installs necessary resources in the virtual garden cluster e.g. RBAC that are necessary for the admission controller.
	// +optional
	GardenDeployment *DeploymentSpec `json:"gardenDeployment,omitempty"`
}

// DeploymentSpec is the specification for the deployment of a component.
type DeploymentSpec struct {
	// Helm contains the specification for a Helm deployment.
	Helm *Helm `json:"helm,omitempty"`
}

// Helm is the Helm deployment configuration.
type Helm struct {
	// OCIRepository defines where to pull the chart.
	// +optional
	OCIRepository *gardencorev1.OCIRepository `json:"ociRepository,omitempty"`
	// Values are the chart values.
	// +optional
	Values *apiextensionsv1.JSON `json:"values,omitempty"`
}

// ExtensionStatus is the status of a Gardener extension.
type ExtensionStatus struct {
	// ObservedGeneration is the most recent generation observed for this resource.
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// Conditions represents the latest available observations of an Extension's current state.
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +optional
	Conditions []gardencorev1beta1.Condition `json:"conditions,omitempty"`
	// ProviderStatus contains type-specific status.
	// +optional
	ProviderStatus *runtime.RawExtension `json:"providerStatus,omitempty"`
}
