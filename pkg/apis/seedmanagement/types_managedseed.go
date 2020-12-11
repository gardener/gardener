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

package seedmanagement

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
	metav1.TypeMeta
	// Standard object metadata.
	metav1.ObjectMeta
	// Specification of the ManagedSeed.
	Spec ManagedSeedSpec
	// Most recently observed status of the ManagedSeed.
	Status ManagedSeedStatus
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ManagedSeedList is a list of ManagedSeed objects.
type ManagedSeedList struct {
	metav1.TypeMeta
	// Standard list object metadata.
	metav1.ListMeta
	// Items is the list of ManagedSeeds.
	Items []ManagedSeed
}

// ManagedSeedSpec is the specification of a ManagedSeed.
type ManagedSeedSpec struct {
	// Shoot is the Shoot that will be registered as Seed.
	Shoot *Shoot
	// Seed describes the Seed that will be registered.
	// Either Seed or Gardenlet must be specified.
	Seed *SeedTemplateSpec
	// Gardenlet specifies gardenlet deployment parameters and the GardenletConfiguration used to configure gardenlet.
	Gardenlet *Gardenlet
}

// Shoot identifies the Shoot that will be registered as Seed.
type Shoot struct {
	// Name is the name of the Shoot that will be registered as Seed.
	Name string
	// APIServer specifies certain kube-apiserver parameters of the Shoot that will be registered as Seed.
	APIServer *APIServer
}

// SeedTemplateSpec describes the data a Seed should have when created from a template.
type SeedTemplateSpec struct {
	// Standard object metadata.
	metav1.ObjectMeta
	// Specification of the desired behavior of the Seed.
	Spec gardencorev1beta1.SeedSpec
}

// Gardenlet specifies gardenlet deployment parameters and the GardenletConfiguration used to configure gardenlet.
type Gardenlet struct {
	// Deployment specifies certain gardenlet deployment parameters, such as the number of replicas,
	// the image, which bootstrap mechanism to use (bootstrap token / service account), etc.
	Deployment *GardenletDeployment
	// Config is the GardenletConfiguration used to configure gardenlet.
	Config *gardenletconfigv1alpha1.GardenletConfiguration
	// Bootstrap is the mechanism that should be used for bootstrapping gardenlet connection to the Garden cluster. One of ServiceAccount, Token.
	// If specified, a service account or a bootstrap token will be created in the garden cluster and used to compute the bootstrap kubeconfig.
	// If not specified, the gardenClientConnection.kubeconfig field will be used to connect to the Garden cluster.
	GardenConnectionBootstrap *GardenConnectionBootstrap
	// SeedConnection is the mechanism for gardenlet connection to the Seed cluster. Must equal ServiceAccount if specified.
	// If not specified, the seedClientConnection.kubeconfig field will be used to connect to the Seed cluster.
	SeedConnection *SeedConnection
	// MergeParentConfig specifies whether the deployment parameters and GardenletConfiguration of the parent gardenlet
	// should be merged with the specified deployment parameters and GardenletConfiguration. Defaults to false.
	MergeParentConfig bool
}

// GardenletDeployment specifies certain gardenlet deployment parameters, such as the number of replicas,
// the image, which bootstrap mechanism to use (bootstrap token / service account), etc.
type GardenletDeployment struct {
	// ReplicaCount is the number of gardenlet replicas. Defaults to 1.
	ReplicaCount *int32
	// RevisionHistoryLimit is the number of old gardenlet ReplicaSets to retain to allow rollback. Defaults to 10.
	RevisionHistoryLimit *int32
	// ServiceAccountName is the name of the ServiceAccount to use to run gardenlet pods.
	ServiceAccountName string
	// Image is the gardenlet container image.
	Image *Image
	// Resources are the compute resources required by the gardenlet container.
	Resources *corev1.ResourceRequirements
	// PodLabels are the labels on gardenlet pods.
	PodLabels map[string]string
	// PodAnnotations are the annotations on gardenlet pods.
	PodAnnotations map[string]string
	// AdditionalVolumes is the list of additional volumes that should be mounted by gardenlet containers.
	AdditionalVolumes []corev1.Volume
	// AdditionalVolumeMounts is the list of additional pod volumes to mount into the gardenlet container's filesystem.
	AdditionalVolumeMounts []corev1.VolumeMount
	// Env is the list of environment variables to set in the gardenlet container.
	Env []corev1.EnvVar
	// VPA specifies whether to enable VPA for gardenlet. Defaults to false.
	VPA *bool
	// ImageVectorOverwrite is the gardenlet image vector overwrite.
	// More info: https://github.com/gardener/gardener/blob/master/docs/deployment/image_vector.md.
	ImageVectorOverwrite string
	// ComponentImageVectorOverwrites is a list of image vector overwrites for components deployed by gardenlet.
	// More info: https://github.com/gardener/gardener/blob/master/docs/deployment/image_vector.md.
	ComponentImageVectorOverwrites string
}

// Image specifies container image parameters.
type Image struct {
	// Repository is the image repository.
	Repository string
	// Tag is the image tag.
	Tag string
	// PullPolicy is the image pull policy. One of Always, Never, IfNotPresent.
	PullPolicy corev1.PullPolicy
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
	Replicas *int32
	// Autoscaler specifies certain kube-apiserver autoscaler parameters, such as the minimum and maximum number of replicas.
	Autoscaler *APIServerAutoscaler
}

// APIServerAutoscaler specifies certain kube-apiserver autoscaler parameters of the Shoot that will be registered as Seed.
type APIServerAutoscaler struct {
	// MinReplicas is the minimum number of kube-apiserver replicas. Defaults to min(3, MaxReplicas).
	MinReplicas *int32
	// MaxReplicas is the maximum number of kube-apiserver replicas. Defaults to 3.
	MaxReplicas *int32
}

// ManagedSeedStatus is the status of a ManagedSeed.
type ManagedSeedStatus struct {
	// LastOperation holds information about the last operation on the ManagedSeed.
	LastOperation *gardencorev1beta1.LastOperation
	// LastError holds information about the last occurred error during an operation.
	LastError *gardencorev1beta1.LastError
	// ObservedGeneration is the most recent generation observed for this ManagedSeed. It corresponds to the
	// ManagedSeed's generation, which is updated on mutation by the API Server.
	ObservedGeneration int64
}
