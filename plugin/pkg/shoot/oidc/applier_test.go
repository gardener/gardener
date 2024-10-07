// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package oidc_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"

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
			CABundle:     ptr.To("cert"),
			ClientID:     "client-id",
			IssuerURL:    "https://foo.bar",
			GroupsClaim:  ptr.To("groupz"),
			GroupsPrefix: ptr.To("group-prefix"),
			RequiredClaims: map[string]string{
				"claim-1": "value-1",
				"claim-2": "value-2",
			},
			SigningAlgs:    []string{"alg-1", "alg-2"},
			UsernameClaim:  ptr.To("user"),
			UsernamePrefix: ptr.To("user-prefix"),
		}
		spec.Client = &v1alpha1.OpenIDConnectClientAuthentication{
			Secret:      ptr.To("secret"),
			ExtraConfig: map[string]string{"foo": "bar", "baz": "dap"},
		}

		shoot.Spec.Kubernetes.Version = "v1.31.0"

		expectedShoot := shoot.DeepCopy()
		expectedShoot.Spec.Kubernetes.KubeAPIServer = &core.KubeAPIServerConfig{
			OIDCConfig: &core.OIDCConfig{
				CABundle:     ptr.To("cert"),
				ClientID:     ptr.To("client-id"),
				IssuerURL:    ptr.To("https://foo.bar"),
				GroupsClaim:  ptr.To("groupz"),
				GroupsPrefix: ptr.To("group-prefix"),
				RequiredClaims: map[string]string{
					"claim-1": "value-1",
					"claim-2": "value-2",
				},
				SigningAlgs:    []string{"alg-1", "alg-2"},
				UsernameClaim:  ptr.To("user"),
				UsernamePrefix: ptr.To("user-prefix"),

				ClientAuthentication: &core.OpenIDConnectClientAuthentication{
					Secret:      ptr.To("secret"),
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
