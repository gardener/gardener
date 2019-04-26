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
	// Resources is a list of combinations of kinds (DNSProvider, Infrastructure, Generic, ...) and their actual types
	// (aws-route53, gcp, auditlog, ...).
	Resources []ControllerResource
	// Deployment contains information for how this controller is deployed.
	Deployment *ControllerDeployment
}

// ControllerResource is a combination of a kind (DNSProvider, Infrastructure, Generic, ...) and the actual type for this
// kind (aws-route53, gcp, auditlog, ...).
type ControllerResource struct {
	// Kind is the resource kind.
	Kind string
	// Type is the resource type.
	Type string
	// GloballyEnabled determines if this resource is required by all Shoot clusters.
	GloballyEnabled *bool
	// ReconcileTimeout defines how long Gardener should wait for the resource reconciliation.
	ReconcileTimeout *metav1.Duration
}

// ControllerDeployment contains information for how this controller is deployed.
type ControllerDeployment struct {
	// Type is the deployment type.
	Type string
	// ProviderConfig contains type-specific configuration.
	ProviderConfig *ProviderConfig
}
