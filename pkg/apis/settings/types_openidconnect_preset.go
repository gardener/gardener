// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

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
