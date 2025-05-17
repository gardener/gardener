// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controllerregistration

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener/pkg/apis/core"
)

var _ = Describe("ControllerRegistration Strategy", func() {
	var ctx context.Context

	BeforeEach(func() {
		ctx = context.Background()
	})

	Describe("#PrepareForUpdate", func() {
		var (
			oldControllerRegistration *core.ControllerRegistration
			newControllerRegistration *core.ControllerRegistration
		)

		BeforeEach(func() {
			oldControllerRegistration = &core.ControllerRegistration{
				Spec: core.ControllerRegistrationSpec{
					Resources: []core.ControllerResource{
						{
							Kind: "Extension",
							Type: "test",
						},
					},
				},
			}
			newControllerRegistration = oldControllerRegistration.DeepCopy()
		})

		Describe("Controller Resources", func() {
			It("should remove the shoot cluster type when globallyEnabled is updated to false", func() {
				newControllerRegistration.Spec.Resources[0].AutoEnable = []core.ClusterType{"shoot", "seed"}
				newControllerRegistration.Spec.Resources[0].GloballyEnabled = ptr.To(false)
				oldControllerRegistration.Spec.Resources[0].GloballyEnabled = ptr.To(true)

				Strategy.PrepareForUpdate(ctx, newControllerRegistration, oldControllerRegistration)

				Expect(newControllerRegistration.Spec.Resources[0].AutoEnable).To(ConsistOf(core.ClusterType("seed")))
			})

			It("should not remove shoot cluster type when globallyEnabled was not set before", func() {
				newControllerRegistration.Spec.Resources[0].AutoEnable = []core.ClusterType{"shoot"}
				newControllerRegistration.Spec.Resources[0].GloballyEnabled = ptr.To(false)
				oldControllerRegistration.Spec.Resources[0].GloballyEnabled = nil

				Strategy.PrepareForUpdate(ctx, newControllerRegistration, oldControllerRegistration)

				Expect(newControllerRegistration.Spec.Resources[0].AutoEnable).To(ConsistOf(core.ClusterType("shoot")))
			})
		})
	})
})
