// Copyright 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

	var client *core.OpenIDConnectClientAuthentication
	if preset.Client != nil {
		client = &core.OpenIDConnectClientAuthentication{
			Secret:      preset.Client.Secret,
			ExtraConfig: preset.Client.ExtraConfig,
		}
	}
	oidc := &core.OIDCConfig{
		CABundle:             preset.Server.CABundle,
		ClientID:             &preset.Server.ClientID,
		GroupsClaim:          preset.Server.GroupsClaim,
		GroupsPrefix:         preset.Server.GroupsPrefix,
		IssuerURL:            &preset.Server.IssuerURL,
		SigningAlgs:          preset.Server.SigningAlgs,
		UsernameClaim:        preset.Server.UsernameClaim,
		UsernamePrefix:       preset.Server.UsernamePrefix,
		RequiredClaims:       preset.Server.RequiredClaims,
		ClientAuthentication: client,
	}

	if shoot.Spec.Kubernetes.KubeAPIServer == nil {
		shoot.Spec.Kubernetes.KubeAPIServer = &core.KubeAPIServerConfig{}
	}
	shoot.Spec.Kubernetes.KubeAPIServer.OIDCConfig = oidc
}
