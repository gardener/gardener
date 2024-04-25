// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

// Code generated by applyconfiguration-gen. DO NOT EDIT.

package applyconfiguration

import (
	v1alpha1 "github.com/gardener/gardener/pkg/apis/settings/v1alpha1"
	settingsv1alpha1 "github.com/gardener/gardener/pkg/client/settings/applyconfiguration/settings/v1alpha1"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
)

// ForKind returns an apply configuration type for the given GroupVersionKind, or nil if no
// apply configuration type exists for the given GroupVersionKind.
func ForKind(kind schema.GroupVersionKind) interface{} {
	switch kind {
	// Group=settings.gardener.cloud, Version=v1alpha1
	case v1alpha1.SchemeGroupVersion.WithKind("ClusterOpenIDConnectPreset"):
		return &settingsv1alpha1.ClusterOpenIDConnectPresetApplyConfiguration{}
	case v1alpha1.SchemeGroupVersion.WithKind("ClusterOpenIDConnectPresetSpec"):
		return &settingsv1alpha1.ClusterOpenIDConnectPresetSpecApplyConfiguration{}
	case v1alpha1.SchemeGroupVersion.WithKind("KubeAPIServerOpenIDConnect"):
		return &settingsv1alpha1.KubeAPIServerOpenIDConnectApplyConfiguration{}
	case v1alpha1.SchemeGroupVersion.WithKind("OpenIDConnectClientAuthentication"):
		return &settingsv1alpha1.OpenIDConnectClientAuthenticationApplyConfiguration{}
	case v1alpha1.SchemeGroupVersion.WithKind("OpenIDConnectPreset"):
		return &settingsv1alpha1.OpenIDConnectPresetApplyConfiguration{}
	case v1alpha1.SchemeGroupVersion.WithKind("OpenIDConnectPresetSpec"):
		return &settingsv1alpha1.OpenIDConnectPresetSpecApplyConfiguration{}

	}
	return nil
}
