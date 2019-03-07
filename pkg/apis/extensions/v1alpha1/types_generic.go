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

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Generic is a specification for a Generic resource.
type Generic struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GenericSpec   `json:"spec"`
	Status GenericStatus `json:"status"`
}

// GetExtensionType returns the type of this Generic resource.
func (g *Generic) GetExtensionType() string {
	return g.Spec.Type
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// GenericList is a list of Generic resources.
type GenericList struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []Generic `json:"items"`
}

// GenericSpec is the spec for a Generic resource.
type GenericSpec struct {
	// DefaultSpec is a structure containing common fields used by all extension resources.
	DefaultSpec `json:",inline"`
	// ProviderConfig is the configuration for the respective extension controller.
	// +optional
	ProviderConfig *runtime.RawExtension `json:"providerConfig,omitempty"`
}

// GenericStatus is the status for a Generic resource.
type GenericStatus struct {
	// DefaultStatus is a structure containing common fields used by all extension resources.
	DefaultStatus `json:",inline"`
}
