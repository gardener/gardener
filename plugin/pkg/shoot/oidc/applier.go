// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package oidc

import (
	"github.com/gardener/gardener/pkg/apis/core"
	settingsv1alpha1 "github.com/gardener/gardener/pkg/apis/settings/v1alpha1"
)

// ApplyOIDCConfiguration applies preset OpenID Connect configuration to the shoot.
func ApplyOIDCConfiguration(shoot *core.Shoot, preset *settingsv1alpha1.OpenIDConnectPresetSpec) {
	if shoot == nil || preset == nil {
		return
	}

	oidc := &core.OIDCConfig{
		CABundle:       preset.Server.CABundle,
		ClientID:       &preset.Server.ClientID,
		GroupsClaim:    preset.Server.GroupsClaim,
		GroupsPrefix:   preset.Server.GroupsPrefix,
		IssuerURL:      &preset.Server.IssuerURL,
		SigningAlgs:    preset.Server.SigningAlgs,
		UsernameClaim:  preset.Server.UsernameClaim,
		UsernamePrefix: preset.Server.UsernamePrefix,
		RequiredClaims: preset.Server.RequiredClaims,
	}

	if shoot.Spec.Kubernetes.KubeAPIServer == nil {
		shoot.Spec.Kubernetes.KubeAPIServer = &core.KubeAPIServerConfig{}
	}
	shoot.Spec.Kubernetes.KubeAPIServer.OIDCConfig = oidc
}
