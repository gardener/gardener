// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package core

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ControllerRegistration represents a registration of an external controller.
type ControllerRegistration struct {
	metav1.TypeMeta
	// Standard object metadata.
	metav1.ObjectMeta

	// Spec contains the specification of this registration.
	// If the object's deletion timestamp is set, this field is immutable.
	Spec ControllerRegistrationSpec
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ControllerRegistrationList is a collection of ControllerRegistrations.
type ControllerRegistrationList struct {
	metav1.TypeMeta
	// Standard list object metadata.
	metav1.ListMeta

	// Items is the list of ControllerRegistrations.
	Items []ControllerRegistration
}

// ControllerRegistrationSpec is the specification of a ControllerRegistration.
type ControllerRegistrationSpec struct {
	// Resources is a list of combinations of kinds (Infrastructure, Generic, ...) and their actual types
	// (aws-route53, gcp, auditlog, ...).
	Resources []ControllerResource
	// Deployment contains information for how this controller is deployed.
	Deployment *ControllerRegistrationDeployment
}

// ClusterType defines the type of cluster.
type ClusterType string

const (
	// ClusterTypeShoot represents the shoot cluster type.
	ClusterTypeShoot ClusterType = "shoot"
	// ClusterTypeSeed represents the seed cluster type.
	ClusterTypeSeed ClusterType = "seed"
)

// ControllerResource is a combination of a kind (Infrastructure, Generic, ...) and the actual type for this
// kind (aws-route53, gcp, auditlog, ...).
type ControllerResource struct {
	// Kind is the resource kind.
	Kind string
	// Type is the resource type.
	Type string
	// ReconcileTimeout defines how long Gardener should wait for the resource reconciliation.
	// This field is defaulted to 3m0s when kind is "Extension".
	ReconcileTimeout *metav1.Duration
	// Primary determines if the controller backed by this ControllerRegistration is responsible for the extension
	// resource's lifecycle. This field defaults to true. There must be exactly one primary controller for this kind/type
	// combination. This field is immutable.
	Primary *bool
	// Lifecycle defines a strategy that determines when different operations on a ControllerResource should be performed.
	// This field is defaulted in the following way when kind is "Extension".
	//  Reconcile: "AfterKubeAPIServer"
	//  Delete: "BeforeKubeAPIServer"
	//  Migrate: "BeforeKubeAPIServer"
	Lifecycle *ControllerResourceLifecycle
	// WorkerlessSupported specifies whether this ControllerResource supports Workerless Shoot clusters.
	// This field is only relevant when kind is "Extension".
	WorkerlessSupported *bool
	// AutoEnable determines if this resource is automatically enabled for shoot or seed clusters, or both.
	// This field can only be set for resources of kind "Extension".
	AutoEnable []ClusterType
	// ClusterCompatibility defines the compatibility of this resource with different cluster types.
	// If compatibility is not specified, it will be defaulted to 'shoot'.
	// This field can only be set for resources of kind "Extension".
	ClusterCompatibility []ClusterType
}

// DeploymentRef contains information about `ControllerDeployment` references.
type DeploymentRef struct {
	// Name is the name of the `ControllerDeployment` that is being referred to.
	Name string
}

// ControllerRegistrationDeployment contains information for how this controller is deployed.
type ControllerRegistrationDeployment struct {
	// Policy controls how the controller is deployed. It defaults to 'OnDemand'.
	Policy *ControllerDeploymentPolicy
	// SeedSelector contains an optional label selector for seeds. Only if the labels match then this controller will be
	// considered for a deployment.
	// An empty list means that all seeds are selected.
	SeedSelector *metav1.LabelSelector
	// DeploymentRefs holds references to `ControllerDeployments`. Only one element is supported currently.
	DeploymentRefs []DeploymentRef
}

// ControllerDeploymentPolicy is a string alias.
type ControllerDeploymentPolicy string

const (
	// ControllerDeploymentPolicyOnDemand specifies that the controller shall be only deployed if required by another
	// resource. If nothing requires it then the controller shall not be deployed.
	ControllerDeploymentPolicyOnDemand ControllerDeploymentPolicy = "OnDemand"
	// ControllerDeploymentPolicyAlways specifies that the controller shall be deployed always, independent of whether
	// another resource requires it or the respective seed has shoots.
	ControllerDeploymentPolicyAlways ControllerDeploymentPolicy = "Always"
	// ControllerDeploymentPolicyAlwaysExceptNoShoots specifies that the controller shall be deployed always, independent of
	// whether another resource requires it, but only when the respective seed has at least one shoot.
	ControllerDeploymentPolicyAlwaysExceptNoShoots ControllerDeploymentPolicy = "AlwaysExceptNoShoots"
)

// ControllerResourceLifecycleStrategy is a string alias.
type ControllerResourceLifecycleStrategy string

const (
	// BeforeKubeAPIServer specifies that a resource should be handled before the kube-apiserver.
	BeforeKubeAPIServer ControllerResourceLifecycleStrategy = "BeforeKubeAPIServer"
	// AfterKubeAPIServer specifies that a resource should be handled after the kube-apiserver.
	AfterKubeAPIServer ControllerResourceLifecycleStrategy = "AfterKubeAPIServer"
	// AfterWorker specifies that a resource should be handled after workers. This is only available during reconcile.
	AfterWorker ControllerResourceLifecycleStrategy = "AfterWorker"
)

// ControllerResourceLifecycle defines the lifecycle of a controller resource.
type ControllerResourceLifecycle struct {
	// Reconcile defines the strategy during reconciliation.
	Reconcile *ControllerResourceLifecycleStrategy
	// Delete defines the strategy during deletion.
	Delete *ControllerResourceLifecycleStrategy
	// Migrate defines the strategy during migration.
	Migrate *ControllerResourceLifecycleStrategy
}
