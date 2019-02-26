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
	"k8s.io/apimachinery/pkg/runtime"
)

// ProviderConfig is a workaround for missing OpenAPI functions on runtime.RawExtension struct.
// https://github.com/kubernetes/kubernetes/issues/55890
// https://github.com/kubernetes-sigs/cluster-api/issues/137
type ProviderConfig struct {
	runtime.RawExtension `json:",inline"`
}

// OpenAPISchemaType is used by the kube-openapi generator when constructing
// the OpenAPI spec of this type.
// See: https://github.com/kubernetes/kube-openapi/tree/master/pkg/generators
func (ProviderConfig) OpenAPISchemaType() []string { return []string{"object"} }

// OpenAPISchemaFormat is used by the kube-openapi generator when constructing
// the OpenAPI spec of this type.
func (ProviderConfig) OpenAPISchemaFormat() string { return "" }

// ConditionStatus is the status of a condition.
type ConditionStatus string

// ConditionType is a string alias.
type ConditionType string

// Condition holds the information about the state of a resource.
type Condition struct {
	// Type of the Shoot condition.
	Type ConditionType `json:"type"`
	// Status of the condition, one of True, False, Unknown.
	Status ConditionStatus `json:"status"`
	// Last time the condition transitioned from one status to another.
	LastTransitionTime metav1.Time `json:"lastTransitionTime"`
	// Last time the condition was updated.
	LastUpdateTime metav1.Time `json:"lastUpdateTime"`
	// The reason for the condition's last transition.
	Reason string `json:"reason"`
	// A human readable message indicating details about the transition.
	Message string `json:"message"`
}

const (
	// ConditionTrue means a resource is in the condition.
	ConditionTrue ConditionStatus = "True"
	// ConditionFalse means a resource is not in the condition.
	ConditionFalse ConditionStatus = "False"
	// ConditionUnknown" means Gardener can't decide if a resource is in the condition or not.
	ConditionUnknown ConditionStatus = "Unknown"
	// ConditionProgressing means the condition was seen true, failed but stayed within a predefined failure threshold.
	// In the future, we could add other intermediate conditions, e.g. ConditionDegraded.
	ConditionProgressing ConditionStatus = "Progressing"

	// ConditionCheckError is a constant for a reason in condition.
	ConditionCheckError = "ConditionCheckError"
)

// CIDR is a string alias.
type CIDR string

// K8SNetworks contains CIDRs for the pod, service and node networks of a Kubernetes cluster.
type K8SNetworks struct {
	// Nodes is the CIDR of the node network.
	// +optional
	Nodes *CIDR `json:"nodes,omitempty"`
	// Pods is the CIDR of the pod network.
	// +optional
	Pods *CIDR `json:"pods,omitempty"`
	// Services is the CIDR of the service network.
	// +optional
	Services *CIDR `json:"services,omitempty"`
}

const (
	// DefaultPodNetworkCIDR is a constant for the default pod network CIDR of a Shoot cluster.
	DefaultPodNetworkCIDR = CIDR("100.96.0.0/11")
	// DefaultServiceNetworkCIDR is a constant for the default service network CIDR of a Shoot cluster.
	DefaultServiceNetworkCIDR = CIDR("100.64.0.0/13")
)
