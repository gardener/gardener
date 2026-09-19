// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package controlplane_test

import (
	"github.com/coreos/go-systemd/v22/unit"
	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	extensionscontextwebhook "github.com/gardener/gardener/extensions/pkg/webhook/context"
	"github.com/gardener/gardener/extensions/pkg/webhook/controlplane/genericmutator"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/gardener/gardener/pkg/provider-local/webhook/controlplane"
)

var _ = Describe("Ensurer", func() {
	var (
		ensurer genericmutator.Ensurer
		shoot   *gardencorev1beta1.Shoot
		gctx    extensionscontextwebhook.GardenContext
	)

	BeforeEach(func() {
		ensurer = NewEnsurer(logr.Discard())
		shoot = &gardencorev1beta1.Shoot{}
		gctx = extensionscontextwebhook.NewInternalGardenContext(&extensionscontroller.Cluster{Shoot: shoot})
	})

	Describe("#EnsureKubeletServiceUnitOptions", func() {
		var unitOptions []*unit.UnitOption

		BeforeEach(func() {
			unitOptions = []*unit.UnitOption{
				{Section: "Service", Name: "ExecStart", Value: `/opt/bin/kubelet \
    --config=/var/lib/kubelet/config/kubelet`},
			}
		})

		It("should add the external cloud provider flag for shoots with managed infrastructure", func(ctx SpecContext) {
			shoot.Spec.CredentialsBindingName = new("local")

			Expect(ensurer.EnsureKubeletServiceUnitOptions(ctx, gctx, nil, unitOptions, nil)).To(Equal([]*unit.UnitOption{
				{Section: "Service", Name: "ExecStart", Value: `/opt/bin/kubelet \
    --config=/var/lib/kubelet/config/kubelet \
    --cloud-provider=external`},
			}))
		})

		It("should not add the external cloud provider flag for shoots with unmanaged infrastructure", func(ctx SpecContext) {
			Expect(ensurer.EnsureKubeletServiceUnitOptions(ctx, gctx, nil, unitOptions, nil)).To(Equal([]*unit.UnitOption{
				{Section: "Service", Name: "ExecStart", Value: `/opt/bin/kubelet \
    --config=/var/lib/kubelet/config/kubelet`},
			}))
		})
	})
})
