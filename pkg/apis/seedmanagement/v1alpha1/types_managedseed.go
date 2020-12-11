// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ManagedSeed represents a Shoot that is registered as Seed.
type ManagedSeed struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object metadata.
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	// Specification of the ManagedSeed.
	Spec ManagedSeedSpec `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
	// Most recently observed status of the ManagedSeed.
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

// ManagedSeedSpec is the specification of a ManagedSeed.
type ManagedSeedSpec struct {
	// Shoot is the Shoot that will be registered as Seed.
	// +optional
	Shoot *Shoot `json:"shoot,omitempty" protobuf:"bytes,1,opt,name=shoot"`
	// Seed describes the Seed that will be registered.
	// Either Seed or Gardenlet must be specified.
	// +optional
	Seed *SeedTemplateSpec `json:"seed,omitempty" protobuf:"bytes,2,opt,name=seed"`
	// Gardenlet specifies gardenlet deployment parameters and the GardenletConfiguration used to configure gardenlet.
	// +optional
	Gardenlet *Gardenlet `json:"gardenlet,omitempty" protobuf:"bytes,3,opt,name=gardenlet"`
}

// Shoot identifies the Shoot that will be registered as Seed.
type Shoot struct {
	// Name is the name of the Shoot that will be registered as Seed.
	Name string `json:"name" protobuf:"bytes,1,opt,name=name"`
	// APIServer specifies certain kube-apiserver parameters of the Shoot that will be registered as Seed.
	// +optional
	APIServer *APIServer `json:"apiServer,omitempty" protobuf:"bytes,2,opt,name=apiServer"`
}

// SeedTemplateSpec describes the data a Seed should have when created from a template.
type SeedTemplateSpec struct {
	// Standard object metadata.
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	// Specification of the desired behavior of the Seed.
	// +optional
	Spec gardencorev1beta1.SeedSpec `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
}

// Gardenlet specifies gardenlet deployment parameters and the GardenletConfiguration used to configure gardenlet.
type Gardenlet struct {
	// Deployment specifies certain gardenlet deployment parameters, such as the number of replicas,
	// the image, which bootstrap mechanism to use (bootstrap token / service account), etc.
	// +optional
	Deployment *GardenletDeployment `json:"deployment,omitempty" protobuf:"bytes,1,opt,name=deployment"`
	// Config is the GardenletConfiguration used to configure gardenlet.
	// +optional
	Config *gardenletconfigv1alpha1.GardenletConfiguration `json:"config,omitempty" protobuf:"bytes,2,opt,name=config"`
	// Bootstrap is the mechanism that should be used for bootstrapping gardenlet connection to the Garden cluster. One of ServiceAccount, Token.
	// If specified, a service account or a bootstrap token will be created in the garden cluster and used to compute the bootstrap kubeconfig.
	// If not specified, the gardenClientConnection.kubeconfig field will be used to connect to the Garden cluster.
	// +optional
	GardenConnectionBootstrap *GardenConnectionBootstrap `json:"gardenConnectionBootstrap,omitempty" protobuf:"bytes,3,opt,name=gardenConnectionBootstrap"`
	// SeedConnection is the mechanism for gardenlet connection to the Seed cluster. Must equal ServiceAccount if specified.
	// If not specified, the seedClientConnection.kubeconfig field will be used to connect to the Seed cluster.
	// +optional
	SeedConnection *SeedConnection `json:"seedConnection,omitempty" protobuf:"bytes,4,opt,name=seedConnection"`
	// MergeParentConfig specifies whether the deployment parameters and GardenletConfiguration of the parent gardenlet
	// should be merged with the specified deployment parameters and GardenletConfiguration. Defaults to false.
	// +optional
	MergeParentConfig bool `json:"mergeParentConfig,omitempty" protobuf:"varint,5,opt,name=mergeParentConfig"`
}

// GardenletDeployment specifies certain gardenlet deployment parameters, such as the number of replicas,
// the image, which bootstrap mechanism to use (bootstrap token / service account), etc.
type GardenletDeployment struct {
	// ReplicaCount is the number of gardenlet replicas. Defaults to 1.
	// +optional
	ReplicaCount *int32 `json:"replicaCount,omitempty" protobuf:"varint,1,opt,name=replicaCount"`
	// RevisionHistoryLimit is the number of old gardenlet ReplicaSets to retain to allow rollback. Defaults to 10.
	// +optional
	RevisionHistoryLimit *int32 `json:"revisionHistoryLimit,omitempty" protobuf:"varint,2,opt,name=revisionHistoryLimit"`
	// ServiceAccountName is the name of the ServiceAccount to use to run gardenlet pods.
	// +optional
	ServiceAccountName string `json:"serviceAccountName,omitempty" protobuf:"bytes,3,opt,name=serviceAccountName"`
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
	// VPA specifies whether to enable VPA for gardenlet. Defaults to false.
	// +optional
	VPA *bool `json:"vpa,omitempty" protobuf:"bytes,11,rep,name=vpa"`
	// ImageVectorOverwrite is the gardenlet image vector overwrite.
	// More info: https://github.com/gardener/gardener/blob/master/docs/deployment/image_vector.md.
	// +optional
	ImageVectorOverwrite string `json:"imageVectorOverwrite,omitempty" protobuf:"bytes,12,rep,name=imageVectorOverwrite"`
	// ComponentImageVectorOverwrites is a list of image vector overwrites for components deployed by gardenlet.
	// More info: https://github.com/gardener/gardener/blob/master/docs/deployment/image_vector.md.
	// +optional
	ComponentImageVectorOverwrites string `json:"componentImageVectorOverwrites,omitempty" protobuf:"bytes,13,rep,name=componentImageVectorOverwrites"`
}

// Image specifies container image parameters.
type Image struct {
	// Repository is the image repository.
	// +optional
	Repository string `json:"repository,omitempty" protobuf:"bytes,1,opt,name=repository"`
	// Tag is the image tag.
	// +optional
	Tag string `json:"tag,omitempty" protobuf:"bytes,2,opt,name=tag"`
	// PullPolicy is the image pull policy. One of Always, Never, IfNotPresent.
	// +optional
	PullPolicy corev1.PullPolicy `json:"pullPolicy,omitempty" protobuf:"bytes,3,opt,name=pullPolicy"`
}

// GardenConnectionBootstrap describes a mechanism for bootstrapping gardenlet connection to the Garden cluster.
type GardenConnectionBootstrap string

const (
	// GardenConnectionBootstrapServiceAccount means that a service account should be used for bootstrapping gardenlet connection to the Garden cluster.
	GardenConnectionBootstrapServiceAccount GardenConnectionBootstrap = "ServiceAccount"
	// GardenConnectionBootstrapToken means that a bootstrap token should be used for bootstrapping gardenlet connection to the Garden cluster.
	GardenConnectionBootstrapToken GardenConnectionBootstrap = "BootstrapToken"
)

// SeedConnection describes a mechanism for gardenlet connection to the Seed cluster.
type SeedConnection string

const (
	// SeedConnectionServiceAccount means that a service account should be used for gardenlet connection to the Seed cluster.
	SeedConnectionServiceAccount SeedConnection = "ServiceAccount"
)

// APIServer specifies certain kube-apiserver parameters of the Shoot that will be registered as Seed.
type APIServer struct {
	// Replicas is the number of kube-apiserver replicas. Defaults to 3.
	// +optional
	Replicas *int32 `json:"replicas,omitempty" protobuf:"varint,1,opt,name=replicas"`
	// Autoscaler specifies certain kube-apiserver autoscaler parameters, such as the minimum and maximum number of replicas.
	// +optional
	Autoscaler *APIServerAutoscaler `json:"autoscaler,omitempty" protobuf:"bytes,1,opt,name=autoscaler"`
}

// APIServerAutoscaler specifies certain kube-apiserver autoscaler parameters of the Shoot that will be registered as Seed.
type APIServerAutoscaler struct {
	// MinReplicas is the minimum number of kube-apiserver replicas. Defaults to min(3, MaxReplicas).
	// +optional
	MinReplicas *int32 `json:"minReplicas,omitempty" protobuf:"varint,1,opt,name=minReplicas"`
	// MaxReplicas is the maximum number of kube-apiserver replicas. Defaults to 3.
	// +optional
	MaxReplicas *int32 `json:"maxReplicas,omitempty" protobuf:"varint,2,opt,name=maxReplicas"`
}

// ManagedSeedStatus is the status of a ManagedSeed.
type ManagedSeedStatus struct {
	// LastOperation holds information about the last operation on the ManagedSeed.
	// +optional
	LastOperation *gardencorev1beta1.LastOperation `json:"lastOperation,omitempty" protobuf:"bytes,1,opt,name=lastOperation"`
	// LastError holds information about the last occurred error during an operation.
	// +optional
	LastError *gardencorev1beta1.LastError `json:"lastError,omitempty" protobuf:"bytes,2,opt,name=lastError"`
	// ObservedGeneration is the most recent generation observed for this ManagedSeed. It corresponds to the
	// ManagedSeed's generation, which is updated on mutation by the API Server.
	ObservedGeneration int64 `json:"observedGeneration,omitempty" protobuf:"varint,3,opt,name=observedGeneration"`
}
