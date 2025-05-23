// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package seedmanagement

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	gardencore "github.com/gardener/gardener/pkg/apis/core"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Gardenlet represents a Gardenlet configuration for an unmanaged seed.
type Gardenlet struct {
	metav1.TypeMeta
	// Standard object metadata.
	metav1.ObjectMeta
	// Specification of the Gardenlet.
	Spec GardenletSpec
	// Most recently observed status of the Gardenlet.
	Status GardenletStatus
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// GardenletList is a list of Gardenlet objects.
type GardenletList struct {
	metav1.TypeMeta
	// Standard list object metadata.
	metav1.ListMeta
	// Items is the list of Gardenlets.
	Items []Gardenlet
}

// GardenletSpec specifies gardenlet deployment parameters and the configuration used to configure gardenlet.
type GardenletSpec struct {
	// Deployment specifies certain gardenlet deployment parameters, such as the number of replicas,
	// the image, etc.
	Deployment GardenletSelfDeployment
	// Config is the GardenletConfiguration used to configure gardenlet.
	Config runtime.Object
	// KubeconfigSecretRef is a reference to a secret containing a kubeconfig for the cluster to which gardenlet should
	// be deployed. This is only used by gardener-operator for a very first gardenlet deployment. After that, gardenlet
	// will continuously upgrade itself. If this field is empty, gardener-operator deploys it into its own runtime
	// cluster.
	KubeconfigSecretRef *corev1.LocalObjectReference
}

// GardenletSelfDeployment specifies certain gardenlet deployment parameters, such as the number of replicas,
// the image, etc.
type GardenletSelfDeployment struct {
	GardenletDeployment
	// Helm is the Helm deployment configuration for gardenlet.
	Helm GardenletHelm
	// ImageVectorOverwrite is the image vector overwrite for the components deployed by this gardenlet.
	ImageVectorOverwrite *string
	// ComponentImageVectorOverwrite is the component image vector overwrite for the components deployed by this
	// gardenlet.
	ComponentImageVectorOverwrite *string
}

// GardenletHelm is the Helm deployment configuration for gardenlet.
type GardenletHelm struct {
	// OCIRepository defines where to pull the chart.
	OCIRepository gardencore.OCIRepository
}

// GardenletStatus is the status of a Gardenlet.
type GardenletStatus struct {
	// Conditions represents the latest available observations of a Gardenlet's current state.
	// +patchMergeKey=type
	// +patchStrategy=merge
	Conditions []gardencore.Condition
	// ObservedGeneration is the most recent generation observed for this Gardenlet. It corresponds to the
	// Gardenlet's generation, which is updated on mutation by the API Server.
	ObservedGeneration int64
}

const (
	// GardenletReconciled is a condition type for indicating whether the Gardenlet has been reconciled.
	GardenletReconciled gardencore.ConditionType = "GardenletReconciled"
)
