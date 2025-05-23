// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

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
