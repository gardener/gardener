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

package oidc_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/pointer"

	"github.com/gardener/gardener/pkg/apis/core"
	"github.com/gardener/gardener/pkg/apis/settings/v1alpha1"
	"github.com/gardener/gardener/plugin/pkg/shoot/oidc"
)

var _ = Describe("Applier", func() {
	var (
		shoot *core.Shoot
		spec  *v1alpha1.OpenIDConnectPresetSpec
	)

	BeforeEach(func() {
		shoot = &core.Shoot{}
		spec = &v1alpha1.OpenIDConnectPresetSpec{
			Server: v1alpha1.KubeAPIServerOpenIDConnect{},
		}
	})

	It("no shoot is passed, no modifications", func() {
		shoot = nil
		specCpy := spec.DeepCopy()

		oidc.ApplyOIDCConfiguration(shoot, spec)
		Expect(spec).To(Equal(specCpy))
	})

	It("no spec is passed, no modifications", func() {
		spec = nil
		shootCpy := shoot.DeepCopy()

		oidc.ApplyOIDCConfiguration(shoot, spec)
		Expect(shoot).To(Equal(shootCpy))
	})

	It("full preset, empty shoot", func() {
		spec.Server = v1alpha1.KubeAPIServerOpenIDConnect{
			CABundle:     pointer.String("cert"),
			ClientID:     "client-id",
			IssuerURL:    "https://foo.bar",
			GroupsClaim:  pointer.String("groupz"),
			GroupsPrefix: pointer.String("group-prefix"),
			RequiredClaims: map[string]string{
				"claim-1": "value-1",
				"claim-2": "value-2",
			},
			SigningAlgs:    []string{"alg-1", "alg-2"},
			UsernameClaim:  pointer.String("user"),
			UsernamePrefix: pointer.String("user-prefix"),
		}
		spec.Client = &v1alpha1.OpenIDConnectClientAuthentication{
			Secret:      pointer.String("secret"),
			ExtraConfig: map[string]string{"foo": "bar", "baz": "dap"},
		}

		shoot.Spec.Kubernetes.Version = "v1.25.0"

		expectedShoot := shoot.DeepCopy()
		expectedShoot.Spec.Kubernetes.KubeAPIServer = &core.KubeAPIServerConfig{
			OIDCConfig: &core.OIDCConfig{
				CABundle:     pointer.String("cert"),
				ClientID:     pointer.String("client-id"),
				IssuerURL:    pointer.String("https://foo.bar"),
				GroupsClaim:  pointer.String("groupz"),
				GroupsPrefix: pointer.String("group-prefix"),
				RequiredClaims: map[string]string{
					"claim-1": "value-1",
					"claim-2": "value-2",
				},
				SigningAlgs:    []string{"alg-1", "alg-2"},
				UsernameClaim:  pointer.String("user"),
				UsernamePrefix: pointer.String("user-prefix"),

				ClientAuthentication: &core.OpenIDConnectClientAuthentication{
					Secret:      pointer.String("secret"),
					ExtraConfig: map[string]string{"foo": "bar", "baz": "dap"},
				},
			},
		}

		oidc.ApplyOIDCConfiguration(shoot, spec)

		Expect(shoot.Spec.Kubernetes.KubeAPIServer.OIDCConfig).To(Equal(expectedShoot.Spec.Kubernetes.KubeAPIServer.OIDCConfig))
		// just to be 100% sure that no other modification is happening.
		Expect(shoot).To(Equal(expectedShoot))
	})
})
