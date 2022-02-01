// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

// ControllerResource is a combination of a kind (DNSProvider, Infrastructure, Generic, ...) and the actual type for this
// kind (aws-route53, gcp, auditlog, ...).
type ControllerResource struct {
	// Kind is the resource kind, for example "OperatingSystemConfig".
	Kind string `json:"kind" protobuf:"bytes,1,opt,name=kind"`
	// Type is the resource type, for example "coreos" or "ubuntu".
	Type string `json:"type" protobuf:"bytes,2,opt,name=type"`
	// GloballyEnabled determines if this ControllerResource is required by all Shoot clusters.
	// +optional
	GloballyEnabled *bool `json:"globallyEnabled,omitempty" protobuf:"varint,3,opt,name=globallyEnabled"`
	// ReconcileTimeout defines how long Gardener should wait for the resource reconciliation.
	// +optional
	ReconcileTimeout *metav1.Duration `json:"reconcileTimeout,omitempty" protobuf:"bytes,4,opt,name=reconcileTimeout"`
	// Primary determines if the controller backed by this ControllerRegistration is responsible for the extension
	// resource's lifecycle. This field defaults to true. There must be exactly one primary controller for this kind/type
	// combination. This field is immutable.
	// +optional
	Primary *bool `json:"primary,omitempty" protobuf:"varint,5,opt,name=primary"`
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
	// DeploymentRefs holds references to `ControllerDeployments`. Only one element is support now.
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
