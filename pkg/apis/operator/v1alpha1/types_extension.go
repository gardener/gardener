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
// +kubebuilder:resource:scope=Cluster,shortName="extop"
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Installed",type=string,JSONPath=`.status.conditions[?(@.type=="Installed")].status`,description="Indicates whether the extension has been reconciled successfully."
// +kubebuilder:printcolumn:name="Required Runtime",type=string,JSONPath=`.status.conditions[?(@.type=="RequiredRuntime")].status`,description="Indicates whether the extension is required in the runtime cluster."
// +kubebuilder:printcolumn:name="Required Virtual",type=string,JSONPath=`.status.conditions[?(@.type=="RequiredVirtual")].status`,description="Indicates whether the extension is required in the virtual cluster."
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
	// Deployment contains deployment configuration for an extension and it's admission controller.
	// +optional
	Deployment *Deployment `json:"deployment,omitempty"`
}

// Deployment specifies how an extension can be installed for a Gardener landscape. It includes the specification
// for installing an extension and/or an admission controller.
type Deployment struct {
	// ExtensionDeployment contains the deployment configuration an extension.
	// +optional
	ExtensionDeployment *ExtensionDeploymentSpec `json:"extension,omitempty"`
	// AdmissionDeployment contains the deployment configuration for an admission controller.
	// +optional
	AdmissionDeployment *AdmissionDeploymentSpec `json:"admission,omitempty"`
}

// ExtensionDeploymentSpec specifies how to install the extension in a gardener landscape. The installation is split into two parts:
// - installing the extension in the virtual garden cluster by creating the ControllerRegistration and ControllerDeployment
// - installing the extension in the runtime cluster (if necessary).
type ExtensionDeploymentSpec struct {
	// DeploymentSpec is the deployment configuration for the extension.
	// +optional
	DeploymentSpec `json:",inline"`
	// Values are the deployment values used in the creation of the ControllerDeployment in the virtual garden cluster.
	// +optional
	Values *apiextensionsv1.JSON `json:"values,omitempty"`
	// RuntimeClusterValues are the deployment values for the extension deployment running in the runtime garden cluster.
	// +optional
	RuntimeClusterValues *apiextensionsv1.JSON `json:"runtimeClusterValues,omitempty"`
	// Policy controls how the controller is deployed. It defaults to 'OnDemand'.
	// +optional
	Policy *gardencorev1beta1.ControllerDeploymentPolicy `json:"policy,omitempty"`
	// SeedSelector contains an optional label selector for seeds. Only if the labels match then this controller will be
	// considered for a deployment.
	// An empty list means that all seeds are selected.
	// +optional
	SeedSelector *metav1.LabelSelector `json:"seedSelector,omitempty"`
	// InjectGardenKubeconfig controls whether a kubeconfig to the garden cluster should be injected into workload
	// resources.
	// +optional
	InjectGardenKubeconfig *bool `json:"injectGardenKubeconfig,omitempty"`
}

// AdmissionDeploymentSpec contains the deployment specification for the admission controller of an extension.
type AdmissionDeploymentSpec struct {
	// RuntimeCluster is the deployment configuration for the admission in the runtime cluster. The runtime deployment
	// is responsible for creating the admission controller in the runtime cluster.
	// +optional
	RuntimeCluster *DeploymentSpec `json:"runtimeCluster,omitempty"`
	// VirtualCluster is the deployment configuration for the admission deployment in the garden cluster. The garden deployment
	// installs necessary resources in the virtual garden cluster e.g. RBAC that are necessary for the admission controller.
	// +optional
	VirtualCluster *DeploymentSpec `json:"virtualCluster,omitempty"`
	// Values are the deployment values. The values will be applied to both admission deployments.
	// +optional
	Values *apiextensionsv1.JSON `json:"values,omitempty"`
}

// DeploymentSpec is the specification for the deployment of a component.
type DeploymentSpec struct {
	// Helm contains the specification for a Helm deployment.
	Helm *ExtensionHelm `json:"helm,omitempty"`
}

// ExtensionHelm is the configuration for a helm deployment.
type ExtensionHelm struct {
	// OCIRepository defines where to pull the chart from.
	// +optional
	OCIRepository *gardencorev1.OCIRepository `json:"ociRepository,omitempty"`
}

// ExtensionStatus is the status of a Gardener extension.
type ExtensionStatus struct {
	// ObservedGeneration is the most recent generation observed for this resource.
	// +optional
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

const (
	// ExtensionInstalled is a condition type for indicating whether the extension has been installed.
	ExtensionInstalled gardencorev1beta1.ConditionType = "Installed"
	// ExtensionRequiredRuntime is a condition type for indicating whether the extension is required in the garden runtime cluster.
	ExtensionRequiredRuntime gardencorev1beta1.ConditionType = "RequiredRuntime"
	// ExtensionRequiredVirtual is a condition type for indicating whether the extension is required in the virtual garden cluster.
	ExtensionRequiredVirtual gardencorev1beta1.ConditionType = "RequiredVirtual"

	// ControllerInstallationsHealthy is a constant for a condition type indicating the health of the controller installations.
	ControllerInstallationsHealthy = "ControllerInstallationsHealthy"
	// ExtensionHealthy is a constant for a condition type indicating the extension's health.
	ExtensionHealthy gardencorev1beta1.ConditionType = "Healthy"
	// ExtensionAdmissionHealthy is a constant for a condition type indicating the runtime extension admission's health.
	ExtensionAdmissionHealthy gardencorev1beta1.ConditionType = "AdmissionHealthy"
)
