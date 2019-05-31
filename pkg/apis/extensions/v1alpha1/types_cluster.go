// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

// ClusterResource is a constant for the name of the Cluster resource.
const ClusterResource = "Cluster"

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Cluster is a specification for a Cluster resource.
type Cluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ClusterSpec `json:"spec"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ClusterList is a list of Cluster resources.
type ClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	// Items is the list of Cluster.
	Items []Cluster `json:"items"`
}

// ClusterSpec is the spec for a Cluster resource.
type ClusterSpec struct {
	// CloudProfile is a raw extension field that contains the cloudprofile resource referenced
	// by the shoot that has to be reconciled.
	CloudProfile runtime.RawExtension `json:"cloudProfile"`
	// Seed is a raw extension field that contains the seed resource referenced by the shoot that
	// has to be reconciled.
	Seed runtime.RawExtension `json:"seed"`
	// Shoot is a raw extension field that contains the shoot resource that has to be reconciled.
	Shoot runtime.RawExtension `json:"shoot"`
}
