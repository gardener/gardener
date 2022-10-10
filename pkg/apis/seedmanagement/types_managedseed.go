// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"k8s.io/apimachinery/pkg/runtime"

	gardencore "github.com/gardener/gardener/pkg/apis/core"
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

// ManagedSeedTemplate is a template for creating a ManagedSeed object.
type ManagedSeedTemplate struct {
	// Standard object metadata.
	metav1.ObjectMeta
	// Specification of the desired behavior of the ManagedSeed.
	Spec ManagedSeedSpec
}

// ManagedSeedSpec is the specification of a ManagedSeed.
type ManagedSeedSpec struct {
	// Shoot references a Shoot that should be registered as Seed.
	// This field is immutable.
	Shoot *Shoot
	// SeedTemplate is a template for a Seed object, that should be used to register a given cluster as a Seed.
	// Either SeedTemplate or Gardenlet must be specified. When Seed is specified, the ManagedSeed controller will not deploy a gardenlet into the cluster
	// and an existing gardenlet reconciling the new Seed is required.
	SeedTemplate *gardencore.SeedTemplate
	// Gardenlet specifies that the ManagedSeed controller should deploy a gardenlet into the cluster
	// with the given deployment parameters and GardenletConfiguration.
	Gardenlet *Gardenlet
}

// Shoot identifies the Shoot that should be registered as Seed.
type Shoot struct {
	// Name is the name of the Shoot that will be registered as Seed.
	Name string
}

// Gardenlet specifies gardenlet deployment parameters and the GardenletConfiguration used to configure gardenlet.
type Gardenlet struct {
	// Deployment specifies certain gardenlet deployment parameters, such as the number of replicas,
	// the image, etc.
	Deployment *GardenletDeployment
	// Config is the GardenletConfiguration used to configure gardenlet.
	Config runtime.Object
	// Bootstrap is the mechanism that should be used for bootstrapping gardenlet connection to the Garden cluster. One of ServiceAccount, BootstrapToken, None.
	// If set to ServiceAccount or BootstrapToken, a service account or a bootstrap token will be created in the garden cluster and used to compute the bootstrap kubeconfig.
	// If set to None, the gardenClientConnection.kubeconfig field will be used to connect to the Garden cluster. Defaults to BootstrapToken.
	// This field is immutable.
	Bootstrap *Bootstrap
	// MergeWithParent specifies whether the GardenletConfiguration of the parent gardenlet
	// should be merged with the specified GardenletConfiguration. Defaults to true. This field is immutable.
	MergeWithParent *bool
}

// GardenletDeployment specifies certain gardenlet deployment parameters, such as the number of replicas,
// the image, etc.
type GardenletDeployment struct {
	// ReplicaCount is the number of gardenlet replicas. Defaults to 1.
	ReplicaCount *int32
	// RevisionHistoryLimit is the number of old gardenlet ReplicaSets to retain to allow rollback. Defaults to 10.
	RevisionHistoryLimit *int32
	// ServiceAccountName is the name of the ServiceAccount to use to run gardenlet pods.
	ServiceAccountName *string
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
	// VPA specifies whether to enable VPA for gardenlet. Defaults to true.
	VPA *bool
	// FailureToleranceType determines how gardenlet replicas are spread across the failure domains, possible values are either `node` or `zone`.
	// Please make sure to adjust the replicaCount accordingly if you intend to run an HA setup for gardenlet.
	FailureToleranceType *gardencore.FailureToleranceType
}

// Image specifies container image parameters.
type Image struct {
	// Repository is the image repository.
	Repository *string
	// Tag is the image tag.
	Tag *string
	// PullPolicy is the image pull policy. One of Always, Never, IfNotPresent.
	// Defaults to Always if latest tag is specified, or IfNotPresent otherwise.
	PullPolicy *corev1.PullPolicy
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
	Conditions []gardencore.Condition
	// ObservedGeneration is the most recent generation observed for this ManagedSeed. It corresponds to the
	// ManagedSeed's generation, which is updated on mutation by the API Server.
	ObservedGeneration int64
}

const (
	// ManagedSeedShootReconciled is a condition type for indicating whether the ManagedSeed's shoot has been reconciled.
	ManagedSeedShootReconciled gardencore.ConditionType = "ShootReconciled"
	// ManagedSeedSeedRegistered is a condition type for indicating whether the ManagedSeed's seed has been registered,
	// either directly or by deploying gardenlet into the shoot.
	ManagedSeedSeedRegistered gardencore.ConditionType = "SeedRegistered"
)
