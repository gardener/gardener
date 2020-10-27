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
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ClusterOpenIDConnectPreset is a OpenID Connect configuration that is applied
// to a Shoot objects cluster-wide.
type ClusterOpenIDConnectPreset struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object metadata.
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	// Spec is the specification of this OpenIDConnect preset.
	Spec ClusterOpenIDConnectPresetSpec `json:"spec" protobuf:"bytes,2,opt,name=spec"`
}

// ClusterOpenIDConnectPresetSpec contains the OpenIDConnect specification and
// project selector matching Shoots in Projects.
type ClusterOpenIDConnectPresetSpec struct {
	OpenIDConnectPresetSpec `json:",inline" protobuf:"bytes,1,opt,name=openIDConnectPresetSpec"`

	// Project decides whether to apply the configuration if the
	// Shoot is in a specific Project matching the label selector.
	// Use the selector only if the OIDC Preset is opt-in, because end
	// users may skip the admission by setting the labels.
	// Defaults to the empty LabelSelector, which matches everything.
	// +optional
	ProjectSelector *metav1.LabelSelector `json:"projectSelector,omitempty" protobuf:"bytes,2,opt,name=projectSelector"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ClusterOpenIDConnectPresetList is a collection of ClusterOpenIDConnectPresets.
type ClusterOpenIDConnectPresetList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard list object metadata.
	// +optional
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	// Items is the list of ClusterOpenIDConnectPresets.
	Items []ClusterOpenIDConnectPreset `json:"items" protobuf:"bytes,2,rep,name=items"`
}
