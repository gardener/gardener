// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ControllerRegistration represents a registration of an external controller.
type ControllerRegistration struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object metadata.
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	// Spec contains the specification of this registration.
	// If the object's deletion timestamp is set, this field is immutable.
	Spec ControllerRegistrationSpec `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ControllerRegistrationList is a collection of ControllerRegistrations.
type ControllerRegistrationList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard list object metadata.
	// +optional
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	// Items is the list of ControllerRegistrations.
	Items []ControllerRegistration `json:"items" protobuf:"bytes,2,rep,name=items"`
}

// ControllerRegistrationSpec is the specification of a ControllerRegistration.
type ControllerRegistrationSpec struct {
	// Resources is a list of combinations of kinds (DNSProvider, Infrastructure, Generic, ...) and their actual types
	// (aws-route53, gcp, auditlog, ...).
	// +optional
	Resources []ControllerResource `json:"resources,omitempty" protobuf:"bytes,1,opt,name=resources"`
	// Deployment contains information for how this controller is deployed.
	// +optional
	Deployment *ControllerRegistrationDeployment `json:"deployment,omitempty" protobuf:"bytes,2,opt,name=deployment"`
}

// AutoEnableMode defines the mode for automatically enabling a resource.
// It specifies whether the resource is enabled for all clusters, only shoot clusters, only seed clusters, or none.
type AutoEnableMode string

const (
	// AutoEnableModeShoot enables the resource only for shoot clusters.
	AutoEnableModeShoot AutoEnableMode = "shoot"
	// AutoEnableModeSeed enables the resource only for seed clusters.
	AutoEnableModeSeed AutoEnableMode = "seed"
)

// ControllerResource is a combination of a kind (DNSProvider, Infrastructure, Generic, ...) and the actual type for this
// kind (aws-route53, gcp, auditlog, ...).
type ControllerResource struct {
	// Kind is the resource kind, for example "OperatingSystemConfig".
	Kind string `json:"kind" protobuf:"bytes,1,opt,name=kind"`
	// Type is the resource type, for example "coreos" or "ubuntu".
	Type string `json:"type" protobuf:"bytes,2,opt,name=type"`
	// GloballyEnabled determines if this ControllerResource is required by all Shoot clusters.
	// Deprecated: This field is deprecated and will be removed in Gardener version v.122. Please use AutoEnable instead.
	// +optional
	GloballyEnabled *bool `json:"globallyEnabled,omitempty" protobuf:"varint,3,opt,name=globallyEnabled"`
	// ReconcileTimeout defines how long Gardener should wait for the resource reconciliation.
	// This field is defaulted to 3m0s when kind is "Extension".
	// +optional
	ReconcileTimeout *metav1.Duration `json:"reconcileTimeout,omitempty" protobuf:"bytes,4,opt,name=reconcileTimeout"`
	// Primary determines if the controller backed by this ControllerRegistration is responsible for the extension
	// resource's lifecycle. This field defaults to true. There must be exactly one primary controller for this kind/type
	// combination. This field is immutable.
	// +optional
	Primary *bool `json:"primary,omitempty" protobuf:"varint,5,opt,name=primary"`
	// Lifecycle defines a strategy that determines when different operations on a ControllerResource should be performed.
	// This field is defaulted in the following way when kind is "Extension".
	//  Reconcile: "AfterKubeAPIServer"
	//  Delete: "BeforeKubeAPIServer"
	//  Migrate: "BeforeKubeAPIServer"
	// +optional
	Lifecycle *ControllerResourceLifecycle `json:"lifecycle,omitempty" protobuf:"bytes,6,opt,name=lifecycle"`
	// WorkerlessSupported specifies whether this ControllerResource supports Workerless Shoot clusters.
	// This field is only relevant when kind is "Extension".
	// +optional
	WorkerlessSupported *bool `json:"workerlessSupported,omitempty" protobuf:"varint,7,opt,name=workerlessSupported"`
	// AutoEnable determines if this resource is automatically enabled for shoot or seed clusters, or both.
	// Valid values are "shoot" and "seed".
	// This field can only be set for resources of kind "Extension".
	// +optional
	AutoEnable []AutoEnableMode `json:"autoEnable,omitempty" protobuf:"bytes,8,rep,name=autoEnable,casttype=AutoEnableMode"`
}

// DeploymentRef contains information about `ControllerDeployment` references.
type DeploymentRef struct {
	// Name is the name of the `ControllerDeployment` that is being referred to.
	Name string `json:"name" protobuf:"bytes,1,opt,name=name"`
}

// ControllerRegistrationDeployment contains information for how this controller is deployed.
type ControllerRegistrationDeployment struct {
	// Policy controls how the controller is deployed. It defaults to 'OnDemand'.
	// +optional
	Policy *ControllerDeploymentPolicy `json:"policy,omitempty" protobuf:"bytes,3,opt,name=policy"`
	// SeedSelector contains an optional label selector for seeds. Only if the labels match then this controller will be
	// considered for a deployment.
	// An empty list means that all seeds are selected.
	// +optional
	SeedSelector *metav1.LabelSelector `json:"seedSelector,omitempty" protobuf:"bytes,4,opt,name=seedSelector"`
	// DeploymentRefs holds references to `ControllerDeployments`. Only one element is supported currently.
	// +optional
	DeploymentRefs []DeploymentRef `json:"deploymentRefs,omitempty" protobuf:"bytes,5,opt,name=deploymentRefs"`
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
	// +optional
	Reconcile *ControllerResourceLifecycleStrategy `json:"reconcile,omitempty" protobuf:"bytes,1,opt,name=reconcile,casttype=ControllerResourceLifecycleStrategy"`
	// Delete defines the strategy during deletion.
	// +optional
	Delete *ControllerResourceLifecycleStrategy `json:"delete,omitempty" protobuf:"bytes,2,opt,name=delete,casttype=ControllerResourceLifecycleStrategy"`
	// Migrate defines the strategy during migration.
	// +optional
	Migrate *ControllerResourceLifecycleStrategy `json:"migrate,omitempty" protobuf:"bytes,3,opt,name=migrate,casttype=ControllerResourceLifecycleStrategy"`
}
