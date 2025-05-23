// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package settings

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ClusterOpenIDConnectPreset is a OpenID Connect configuration that is applied
// to a Shoot objects cluster-wide.
//
// Deprecated: This resource is deprecated and will be removed after support for Kubernetes 1.31 is dropped.
// Please configure and use structured authentication instead of oidc flags.
// For more information check https://github.com/gardener/gardener/issues/9858
// TODO(AleksandarSavchev): Remove this resource after support for Kubernetes 1.31 is dropped.
type ClusterOpenIDConnectPreset struct {
	metav1.TypeMeta
	// Standard object metadata.
	metav1.ObjectMeta
	// Spec is the specification of this OpenIDConnect preset.
	Spec ClusterOpenIDConnectPresetSpec
}

// ClusterOpenIDConnectPresetSpec contains the OpenIDConnect specification and
// project selector matching Shoots in Projects.
type ClusterOpenIDConnectPresetSpec struct {
	OpenIDConnectPresetSpec

	// Project decides whether to apply the configuration if the
	// Shoot is in a specific Project matching the label selector.
	// Use the selector only if the OIDC Preset is opt-in, because end
	// users may skip the admission by setting the labels.
	// Defaults to the empty LabelSelector, which matches everything.
	ProjectSelector *metav1.LabelSelector
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ClusterOpenIDConnectPresetList is a collection of ClusterOpenIDConnectPresets.
type ClusterOpenIDConnectPresetList struct {
	metav1.TypeMeta
	// Standard list object metadata.
	metav1.ListMeta
	// Items is the list of ClusterOpenIDConnectPresets.
	Items []ClusterOpenIDConnectPreset
}

var _ Preset = &ClusterOpenIDConnectPreset{}

// GetPresetSpec returns a pointer to the OpenIDConnect specification.
func (o *ClusterOpenIDConnectPreset) GetPresetSpec() *OpenIDConnectPresetSpec {
	return &o.Spec.OpenIDConnectPresetSpec
}

// SetPresetSpec sets the OpenIDConnect specification.
func (o *ClusterOpenIDConnectPreset) SetPresetSpec(s *OpenIDConnectPresetSpec) {
	o.Spec.OpenIDConnectPresetSpec = *s
}
