// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package v1beta1

import (
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ControllerDeployment contains information about how this controller is deployed.
type ControllerDeployment struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object metadata.
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	// Type is the deployment type.
	Type string `json:"type" protobuf:"bytes,2,opt,name=type"`
	// ProviderConfig contains type-specific configuration. It contains assets that deploy the controller.
	ProviderConfig runtime.RawExtension `json:"providerConfig" protobuf:"bytes,3,opt,name=providerConfig"`
	// InjectGardenKubeconfig controls whether a kubeconfig to the garden cluster should be injected into workload
	// resources.
	// +optional
	InjectGardenKubeconfig *bool `json:"injectGardenKubeconfig,omitempty" protobuf:"varint,4,opt,name=injectGardenKubeconfig"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ControllerDeploymentList is a collection of ControllerDeployments.
type ControllerDeploymentList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard list object metadata.
	// +optional
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	// Items is the list of ControllerDeployments.
	Items []ControllerDeployment `json:"items" protobuf:"bytes,2,rep,name=items"`
}

const (
	// ControllerDeploymentTypeHelm is the type for instructing the extension controller deployment using helm.
	// The ControllerDeployment.ProviderConfig is expected to hold a HelmControllerDeployment object.
	ControllerDeploymentTypeHelm = "helm"
)

// HelmControllerDeployment configures how an extension controller is deployed using helm.
// This is the legacy structure that used to be defined in gardenlet's ControllerInstallation controller for
// ControllerDeployment's with type=helm.
// While this is not a proper API type, we need to define the structure in the API package so that we can convert it
// to the internal API version in the new representation.
type HelmControllerDeployment struct {
	// Chart is a Helm chart tarball.
	Chart []byte `json:"chart,omitempty" protobuf:"bytes,1,opt,name=chart"`
	// Values is a map of values for the given chart.
	Values *apiextensionsv1.JSON `json:"values,omitempty" protobuf:"bytes,2,opt,name=values"`
	// OCIRepository defines where to pull the chart.
	// +optional
	OCIRepository *OCIRepository `json:"ociRepository,omitempty" protobuf:"bytes,3,opt,name=ociRepository"`
}

// OCIRepository configures where to pull an OCI Artifact, that could contain for example a Helm Chart.
type OCIRepository struct {
	// Ref is the full artifact Ref and takes precedence over all other fields.
	// +optional
	Ref *string `json:"ref,omitempty" protobuf:"bytes,1,name=ref"`
	// Repository is a reference to an OCI artifact repository.
	// +optional
	Repository *string `json:"repository,omitempty" protobuf:"bytes,2,name=repository"`
	// Tag is the image tag to pull.
	// +optional
	Tag *string `json:"tag,omitempty" protobuf:"bytes,3,opt,name=tag"`
	// Digest of the image to pull, takes precedence over tag.
	// +optional
	Digest *string `json:"digest,omitempty" protobuf:"bytes,4,opt,name=digest"`
}
