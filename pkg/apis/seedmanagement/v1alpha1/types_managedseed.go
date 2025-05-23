// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ManagedSeed represents a Shoot that is registered as Seed.
type ManagedSeed struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object metadata.
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	// Specification of the ManagedSeed.
	// +optional
	Spec ManagedSeedSpec `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
	// Most recently observed status of the ManagedSeed.
	// +optional
	Status ManagedSeedStatus `json:"status,omitempty" protobuf:"bytes,3,opt,name=status"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ManagedSeedList is a list of ManagedSeed objects.
type ManagedSeedList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard list object metadata.
	// +optional
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	// Items is the list of ManagedSeeds.
	Items []ManagedSeed `json:"items" protobuf:"bytes,2,rep,name=items"`
}

// ManagedSeedTemplate is a template for creating a ManagedSeed object.
type ManagedSeedTemplate struct {
	// Standard object metadata.
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	// Specification of the desired behavior of the ManagedSeed.
	// +optional
	Spec ManagedSeedSpec `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
}

// ManagedSeedSpec is the specification of a ManagedSeed.
type ManagedSeedSpec struct {
	// Shoot references a Shoot that should be registered as Seed.
	// This field is immutable.
	// +optional
	Shoot *Shoot `json:"shoot,omitempty" protobuf:"bytes,1,opt,name=shoot"`

	// SeedTemplate is tombstoned to show why 2 is reserved protobuf tag.
	// SeedTemplate *gardencorev1beta1.SeedTemplate `json:"seedTemplate,omitempty" protobuf:"bytes,2,opt,name=seedTemplate"`

	// Gardenlet specifies that the ManagedSeed controller should deploy a gardenlet into the cluster
	// with the given deployment parameters and GardenletConfiguration.
	Gardenlet GardenletConfig `json:"gardenlet" protobuf:"bytes,3,opt,name=gardenlet"`
}

// Shoot identifies the Shoot that should be registered as Seed.
type Shoot struct {
	// Name is the name of the Shoot that will be registered as Seed.
	Name string `json:"name" protobuf:"bytes,1,opt,name=name"`
}

// GardenletConfig specifies gardenlet deployment parameters and the GardenletConfiguration used to configure gardenlet.
type GardenletConfig struct {
	// Deployment specifies certain gardenlet deployment parameters, such as the number of replicas,
	// the image, etc.
	// +optional
	Deployment *GardenletDeployment `json:"deployment,omitempty" protobuf:"bytes,1,opt,name=deployment"`
	// Config is the GardenletConfiguration used to configure gardenlet.
	// +optional
	Config runtime.RawExtension `json:"config,omitempty" protobuf:"bytes,2,opt,name=config"`
	// Bootstrap is the mechanism that should be used for bootstrapping gardenlet connection to the Garden cluster. One of ServiceAccount, BootstrapToken, None.
	// If set to ServiceAccount or BootstrapToken, a service account or a bootstrap token will be created in the garden cluster and used to compute the bootstrap kubeconfig.
	// If set to None, the gardenClientConnection.kubeconfig field will be used to connect to the Garden cluster. Defaults to BootstrapToken.
	// This field is immutable.
	// +optional
	Bootstrap *Bootstrap `json:"bootstrap,omitempty" protobuf:"bytes,3,opt,name=bootstrap"`
	// MergeWithParent specifies whether the GardenletConfiguration of the parent gardenlet
	// should be merged with the specified GardenletConfiguration. Defaults to true. This field is immutable.
	// +optional
	MergeWithParent *bool `json:"mergeWithParent,omitempty" protobuf:"varint,4,opt,name=mergeWithParent"`
}

// GardenletDeployment specifies certain gardenlet deployment parameters, such as the number of replicas,
// the image, etc.
type GardenletDeployment struct {
	// ReplicaCount is the number of gardenlet replicas. Defaults to 2.
	// +optional
	ReplicaCount *int32 `json:"replicaCount,omitempty" protobuf:"varint,1,opt,name=replicaCount"`
	// RevisionHistoryLimit is the number of old gardenlet ReplicaSets to retain to allow rollback. Defaults to 2.
	// +optional
	RevisionHistoryLimit *int32 `json:"revisionHistoryLimit,omitempty" protobuf:"varint,2,opt,name=revisionHistoryLimit"`
	// ServiceAccountName is the name of the ServiceAccount to use to run gardenlet pods.
	// +optional
	ServiceAccountName *string `json:"serviceAccountName,omitempty" protobuf:"bytes,3,opt,name=serviceAccountName"`
	// Image is the gardenlet container image.
	// +optional
	Image *Image `json:"image,omitempty" protobuf:"bytes,4,opt,name=image"`
	// Resources are the compute resources required by the gardenlet container.
	// +optional
	Resources *corev1.ResourceRequirements `json:"resources,omitempty" protobuf:"bytes,5,opt,name=resources"`
	// PodLabels are the labels on gardenlet pods.
	// +optional
	PodLabels map[string]string `json:"podLabels,omitempty" protobuf:"bytes,6,opt,name=podLabels"`
	// PodAnnotations are the annotations on gardenlet pods.
	// +optional
	PodAnnotations map[string]string `json:"podAnnotations,omitempty" protobuf:"bytes,7,opt,name=podAnnotations"`
	// AdditionalVolumes is the list of additional volumes that should be mounted by gardenlet containers.
	// +optional
	AdditionalVolumes []corev1.Volume `json:"additionalVolumes,omitempty" protobuf:"bytes,8,rep,name=additionalVolumes"`
	// AdditionalVolumeMounts is the list of additional pod volumes to mount into the gardenlet container's filesystem.
	// +optional
	AdditionalVolumeMounts []corev1.VolumeMount `json:"additionalVolumeMounts,omitempty" protobuf:"bytes,9,rep,name=additionalVolumeMounts"`
	// Env is the list of environment variables to set in the gardenlet container.
	// +optional
	Env []corev1.EnvVar `json:"env,omitempty" protobuf:"bytes,10,rep,name=env"`
}

// Image specifies container image parameters.
type Image struct {
	// Repository is the image repository.
	// +optional
	Repository *string `json:"repository,omitempty" protobuf:"bytes,1,opt,name=repository"`
	// Tag is the image tag.
	// +optional
	Tag *string `json:"tag,omitempty" protobuf:"bytes,2,opt,name=tag"`
	// PullPolicy is the image pull policy. One of Always, Never, IfNotPresent.
	// Defaults to Always if latest tag is specified, or IfNotPresent otherwise.
	// +optional
	PullPolicy *corev1.PullPolicy `json:"pullPolicy,omitempty" protobuf:"bytes,3,opt,name=pullPolicy"`
}

// Bootstrap describes a mechanism for bootstrapping gardenlet connection to the Garden cluster.
type Bootstrap string

const (
	// BootstrapServiceAccount means that a temporary service account should be used for bootstrapping gardenlet connection to the Garden cluster.
	BootstrapServiceAccount Bootstrap = "ServiceAccount"
	// BootstrapToken means that a bootstrap token should be used for bootstrapping gardenlet connection to the Garden cluster.
	BootstrapToken Bootstrap = "BootstrapToken"
	// BootstrapNone means that gardenlet connection to the Garden cluster should not be bootstrapped
	// and the gardenClientConnection.kubeconfig field should be used to connect to the Garden cluster.
	BootstrapNone Bootstrap = "None"
)

// ManagedSeedStatus is the status of a ManagedSeed.
type ManagedSeedStatus struct {
	// Conditions represents the latest available observations of a ManagedSeed's current state.
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +optional
	Conditions []gardencorev1beta1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
	// ObservedGeneration is the most recent generation observed for this ManagedSeed. It corresponds to the
	// ManagedSeed's generation, which is updated on mutation by the API Server.
	ObservedGeneration int64 `json:"observedGeneration,omitempty" protobuf:"varint,2,opt,name=observedGeneration"`
}

const (
	// ManagedSeedShootReconciled is a condition type for indicating whether the ManagedSeed's shoot has been reconciled.
	ManagedSeedShootReconciled gardencorev1beta1.ConditionType = "ShootReconciled"
	// SeedRegistered is a condition type for indicating whether the seed has been registered by gardenlet.
	SeedRegistered gardencorev1beta1.ConditionType = "SeedRegistered"
)
