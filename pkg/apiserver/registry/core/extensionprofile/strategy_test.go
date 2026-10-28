// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package extensionprofile_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apiserver/pkg/registry/rest"

	"github.com/gardener/gardener/pkg/apis/core"
	"github.com/gardener/gardener/pkg/apiserver/registry/core/extensionprofile"
)

var _ = Describe("Strategy", func() {
	var (
		ctx      = context.Background()
		strategy rest.RESTCreateUpdateStrategy
	)

	BeforeEach(func() {
		strategy = extensionprofile.Strategy
	})

	It("should be cluster-scoped", func() {
		Expect(strategy.NamespaceScoped()).To(BeFalse())
	})

	Describe("#PrepareForCreate", func() {
		It("should clear the status on create", func() {
			extensionProfile := &core.ExtensionProfile{
				Status: core.ExtensionProfileStatus{
					Components: []core.ExtensionProfileComponentStatus{{Name: "traefik"}},
				},
			}

			strategy.PrepareForCreate(ctx, extensionProfile)

			Expect(extensionProfile.Status).To(Equal(core.ExtensionProfileStatus{}))
		})
	})

	Describe("#PrepareForUpdate", func() {
		It("should preserve the old status (status is changed only via the subresource)", func() {
			oldProfile := &core.ExtensionProfile{
				Status: core.ExtensionProfileStatus{
					Components: []core.ExtensionProfileComponentStatus{{Name: "traefik"}},
				},
			}
			newProfile := &core.ExtensionProfile{}

			strategy.PrepareForUpdate(ctx, newProfile, oldProfile)

			Expect(newProfile.Status).To(Equal(oldProfile.Status))
		})
	})

	Describe("StatusStrategy #PrepareForUpdate", func() {
		It("should preserve the old spec (spec is not changed via the status subresource)", func() {
			oldProfile := &core.ExtensionProfile{
				Spec: core.ExtensionProfileSpec{
					Components: []core.ExtensionProfileComponent{{Name: "traefik"}},
				},
			}
			newProfile := &core.ExtensionProfile{}

			extensionprofile.StatusStrategy.PrepareForUpdate(ctx, newProfile, oldProfile)

			Expect(newProfile.Spec).To(Equal(oldProfile.Spec))
		})
	})
})
