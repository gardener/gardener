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

package v1alpha1

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
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// Spec contains the specification of this registration.
	Spec ControllerRegistrationSpec `json:"spec,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ControllerRegistrationList is a collection of ControllerRegistrations.
type ControllerRegistrationList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard list object metadata.
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`
	// Items is the list of ControllerRegistrations.
	Items []ControllerRegistration `json:"items"`
}

// ControllerRegistrationSpec is the specification of a ControllerRegistration.
type ControllerRegistrationSpec struct {
	// Resources is a list of combinations of kinds (DNSProvider, Infrastructure, Generic, ...) and their actual types
	// (aws-route53, gcp, auditlog, ...).
	Resources []ControllerResource `json:"resources"`
	// Deployment contains information for how this controller is deployed.
	// +optional
	Deployment *ControllerDeployment `json:"deployment,omitempty"`
}

// ControllerResource is a combination of a kind (DNSProvider, Infrastructure, Generic, ...) and the actual type for this
// kind (aws-route53, gcp, auditlog, ...).
type ControllerResource struct {
	// Kind is the resource kind, for example "OperatingSystemConfig".
	Kind string `json:"kind"`
	// Type is the resource type, for example "coreos" or "ubuntu".
	Type string `json:"type"`
	// GloballyEnabled determines if this ControllerResource is required by all Shoot clusters.
	// +optional
	GloballyEnabled *bool `json:"globallyEnabled,omitempty"`
	// ReconcileTimeout defines how long Gardener should wait for the resource reconciliation.
	// +optional
	ReconcileTimeout *metav1.Duration `json:"reconcileTimeout,omitempty"`
}

// ControllerDeployment contains information for how this controller is deployed.
type ControllerDeployment struct {
	// Type is the deployment type.
	Type string `json:"type"`
	// ProviderConfig contains type-specific configuration.
	// +optional
	ProviderConfig *ProviderConfig `json:"providerConfig,omitempty"`
}
