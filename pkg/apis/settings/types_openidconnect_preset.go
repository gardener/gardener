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

package settings

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// OpenIDConnectPreset is a OpenID Connect configuration that is applied
// to a Shoot in a namespace.
type OpenIDConnectPreset struct {
	metav1.TypeMeta
	// Standard object metadata.
	metav1.ObjectMeta

	Spec OpenIDConnectPresetSpec
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// OpenIDConnectPresetList is a collection of OpenIDConnectPresets.
type OpenIDConnectPresetList struct {
	metav1.TypeMeta
	// Standard list object metadata.
	metav1.ListMeta
	// Items is the list of OpenIDConnectPresets.
	Items []OpenIDConnectPreset
}

var _ Preset = &OpenIDConnectPreset{}

// GetPresetSpec returns a pointer to the OpenIDConnect specification.
func (o *OpenIDConnectPreset) GetPresetSpec() *OpenIDConnectPresetSpec {
	return &o.Spec
}

// SetPresetSpec sets the OpenIDConnect specification.
func (o *OpenIDConnectPreset) SetPresetSpec(s *OpenIDConnectPresetSpec) {
	o.Spec = *s
}
