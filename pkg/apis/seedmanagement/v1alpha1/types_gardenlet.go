// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	gardencorev1 "github.com/gardener/gardener/pkg/apis/core/v1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Gardenlet represents a Gardenlet configuration for an unmanaged seed.
type Gardenlet struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object metadata.
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	// Specification of the Gardenlet.
	// +optional
	Spec GardenletSpec `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
	// Most recently observed status of the Gardenlet.
	// +optional
	Status GardenletStatus `json:"status,omitempty" protobuf:"bytes,3,opt,name=status"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// GardenletList is a list of Gardenlet objects.
type GardenletList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard list object metadata.
	// +optional
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	// Items is the list of Gardenlets.
	Items []Gardenlet `json:"items" protobuf:"bytes,2,rep,name=items"`
}

// GardenletSpec specifies gardenlet deployment parameters and the configuration used to configure gardenlet.
type GardenletSpec struct {
	// Deployment specifies certain gardenlet deployment parameters, such as the number of replicas,
	// the image, etc.
	Deployment GardenletSelfDeployment `json:"deployment" protobuf:"bytes,1,opt,name=deployment"`
	// Config is the GardenletConfiguration used to configure gardenlet.
	// +optional
	Config runtime.RawExtension `json:"config,omitempty" protobuf:"bytes,2,opt,name=config"`
	// KubeconfigSecretRef is a reference to a secret containing a kubeconfig for the cluster to which gardenlet should
	// be deployed. This is only used by gardener-operator for a very first gardenlet deployment. After that, gardenlet
	// will continuously upgrade itself. If this field is empty, gardener-operator deploys it into its own runtime
	// cluster.
	// +optional
	KubeconfigSecretRef *corev1.LocalObjectReference `json:"kubeconfigSecretRef,omitempty" protobuf:"bytes,3,opt,name=kubeconfigSecretRef"`
}

// GardenletSelfDeployment specifies certain gardenlet deployment parameters, such as the number of replicas,
// the image, etc.
type GardenletSelfDeployment struct {
	// GardenletDeployment specifies common gardenlet deployment parameters.
	// +optional
	GardenletDeployment `json:",inline" protobuf:"bytes,1,opt,name=gardenletDeployment"`
	// Helm is the Helm deployment configuration.
	Helm GardenletHelm `json:"helm" protobuf:"bytes,2,opt,name=helm"`
	// ImageVectorOverwrite is the image vector overwrite for the components deployed by this gardenlet.
	// +optional
	ImageVectorOverwrite *string `json:"imageVectorOverwrite,omitempty" protobuf:"bytes,3,opt,name=imageVectorOverwrite"`
	// ComponentImageVectorOverwrite is the component image vector overwrite for the components deployed by this
	// gardenlet.
	// +optional
	ComponentImageVectorOverwrite *string `json:"componentImageVectorOverwrite,omitempty" protobuf:"bytes,4,opt,name=componentImageVectorOverwrite"`
}

// GardenletHelm is the Helm deployment configuration for gardenlet.
type GardenletHelm struct {
	// OCIRepository defines where to pull the chart.
	OCIRepository gardencorev1.OCIRepository `json:"ociRepository" protobuf:"bytes,1,opt,name=ociRepository"`
}

// GardenletStatus is the status of a Gardenlet.
type GardenletStatus struct {
	// Conditions represents the latest available observations of a Gardenlet's current state.
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +optional
	Conditions []gardencorev1beta1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
	// ObservedGeneration is the most recent generation observed for this Gardenlet. It corresponds to the Gardenlet's
	// generation, which is updated on mutation by the API Server.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty" protobuf:"varint,2,opt,name=observedGeneration"`
}

const (
	// GardenletReconciled is a condition type for indicating whether the Gardenlet has been reconciled.
	GardenletReconciled gardencorev1beta1.ConditionType = "GardenletReconciled"
)
