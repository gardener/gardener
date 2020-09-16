// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package seed_test

import (
	"context"

	"github.com/gardener/gardener/pkg/apis/core"
	. "github.com/gardener/gardener/pkg/registry/core/seed"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/utils/pointer"
)

var _ = Describe("Strategy", func() {
	var (
		ctx      = context.TODO()
		strategy = Strategy{}
	)

	Describe("#PrepareForUpdate", func() {
		var oldSeed, newSeed *core.Seed

		BeforeEach(func() {
			oldSeed = &core.Seed{}
			newSeed = &core.Seed{}
		})

		It("should preserve the status", func() {
			newSeed.Status = core.SeedStatus{KubernetesVersion: pointer.StringPtr("1.2.3")}
			strategy.PrepareForUpdate(ctx, newSeed, oldSeed)
			Expect(newSeed.Status).To(Equal(oldSeed.Status))
		})

		It("should bump the generation if the spec changes", func() {
			newSeed.Spec.Provider.Type = "foo"
			strategy.PrepareForUpdate(ctx, newSeed, oldSeed)
			Expect(newSeed.Generation).To(Equal(oldSeed.Generation + 1))
		})

		Context("settings migration", func() {
			Context("excess capacity reservation", func() {
				It("should change the setting if the taint was added", func() {
					oldSeed.Spec.Taints = nil
					newSeed.Spec.Taints = append(newSeed.Spec.Taints, core.SeedTaint{Key: core.DeprecatedSeedTaintDisableCapacityReservation})
					strategy.PrepareForUpdate(ctx, newSeed, oldSeed)
					Expect(newSeed.Spec.Settings.ExcessCapacityReservation.Enabled).To(BeFalse())
				})

				It("should change the setting if the taint was removed", func() {
					oldSeed.Spec.Taints = append(oldSeed.Spec.Taints, core.SeedTaint{Key: core.DeprecatedSeedTaintDisableCapacityReservation})
					newSeed.Spec.Taints = nil
					strategy.PrepareForUpdate(ctx, newSeed, oldSeed)
					Expect(newSeed.Spec.Settings.ExcessCapacityReservation.Enabled).To(BeTrue())
				})

				It("should add the taint if the setting was disabled", func() {
					oldSeed.Spec.Settings = &core.SeedSettings{ExcessCapacityReservation: &core.SeedSettingExcessCapacityReservation{Enabled: true}}
					newSeed.Spec.Settings = &core.SeedSettings{ExcessCapacityReservation: &core.SeedSettingExcessCapacityReservation{Enabled: false}}
					newSeed.Spec.Taints = nil
					strategy.PrepareForUpdate(ctx, newSeed, oldSeed)
					Expect(newSeed.Spec.Taints).To(ContainElement(core.SeedTaint{Key: core.DeprecatedSeedTaintDisableCapacityReservation}))
				})

				It("should remove the taint if the setting was enabled", func() {
					oldSeed.Spec.Taints = nil
					oldSeed.Spec.Settings = &core.SeedSettings{ExcessCapacityReservation: &core.SeedSettingExcessCapacityReservation{Enabled: false}}
					newSeed.Spec.Settings = &core.SeedSettings{ExcessCapacityReservation: &core.SeedSettingExcessCapacityReservation{Enabled: true}}
					strategy.PrepareForUpdate(ctx, newSeed, oldSeed)
					Expect(newSeed.Spec.Taints).NotTo(ContainElement(core.SeedTaint{Key: core.DeprecatedSeedTaintDisableCapacityReservation}))
				})
			})

			Context("scheduling visibility", func() {
				It("should change the setting if the taint was added", func() {
					oldSeed.Spec.Taints = nil
					newSeed.Spec.Taints = append(newSeed.Spec.Taints, core.SeedTaint{Key: core.DeprecatedSeedTaintInvisible})
					strategy.PrepareForUpdate(ctx, newSeed, oldSeed)
					Expect(newSeed.Spec.Settings.Scheduling.Visible).To(BeFalse())
				})

				It("should change the setting if the taint was removed", func() {
					oldSeed.Spec.Taints = append(oldSeed.Spec.Taints, core.SeedTaint{Key: core.DeprecatedSeedTaintInvisible})
					newSeed.Spec.Taints = nil
					strategy.PrepareForUpdate(ctx, newSeed, oldSeed)
					Expect(newSeed.Spec.Settings.Scheduling.Visible).To(BeTrue())
				})

				It("should add the taint if the setting was disabled", func() {
					oldSeed.Spec.Settings = &core.SeedSettings{Scheduling: &core.SeedSettingScheduling{Visible: true}}
					newSeed.Spec.Settings = &core.SeedSettings{Scheduling: &core.SeedSettingScheduling{Visible: false}}
					newSeed.Spec.Taints = nil
					strategy.PrepareForUpdate(ctx, newSeed, oldSeed)
					Expect(newSeed.Spec.Taints).To(ContainElement(core.SeedTaint{Key: core.DeprecatedSeedTaintInvisible}))
				})

				It("should remove the taint if the setting was enabled", func() {
					oldSeed.Spec.Taints = nil
					oldSeed.Spec.Settings = &core.SeedSettings{Scheduling: &core.SeedSettingScheduling{Visible: false}}
					newSeed.Spec.Settings = &core.SeedSettings{Scheduling: &core.SeedSettingScheduling{Visible: true}}
					strategy.PrepareForUpdate(ctx, newSeed, oldSeed)
					Expect(newSeed.Spec.Taints).NotTo(ContainElement(core.SeedTaint{Key: core.DeprecatedSeedTaintInvisible}))
				})
			})

			Context("shoot dns", func() {
				It("should change the setting if the taint was added", func() {
					oldSeed.Spec.Taints = nil
					newSeed.Spec.Taints = append(newSeed.Spec.Taints, core.SeedTaint{Key: core.DeprecatedSeedTaintDisableDNS})
					strategy.PrepareForUpdate(ctx, newSeed, oldSeed)
					Expect(newSeed.Spec.Settings.ShootDNS.Enabled).To(BeFalse())
				})

				It("should change the setting if the taint was removed", func() {
					oldSeed.Spec.Taints = append(oldSeed.Spec.Taints, core.SeedTaint{Key: core.DeprecatedSeedTaintDisableDNS})
					newSeed.Spec.Taints = nil
					strategy.PrepareForUpdate(ctx, newSeed, oldSeed)
					Expect(newSeed.Spec.Settings.ShootDNS.Enabled).To(BeTrue())
				})

				It("should add the taint if the setting was disabled", func() {
					oldSeed.Spec.Settings = &core.SeedSettings{ShootDNS: &core.SeedSettingShootDNS{Enabled: true}}
					newSeed.Spec.Settings = &core.SeedSettings{ShootDNS: &core.SeedSettingShootDNS{Enabled: false}}
					newSeed.Spec.Taints = nil
					strategy.PrepareForUpdate(ctx, newSeed, oldSeed)
					Expect(newSeed.Spec.Taints).To(ContainElement(core.SeedTaint{Key: core.DeprecatedSeedTaintDisableDNS}))
				})

				It("should remove the taint if the setting was enabled", func() {
					oldSeed.Spec.Taints = nil
					oldSeed.Spec.Settings = &core.SeedSettings{ShootDNS: &core.SeedSettingShootDNS{Enabled: false}}
					newSeed.Spec.Settings = &core.SeedSettings{ShootDNS: &core.SeedSettingShootDNS{Enabled: true}}
					strategy.PrepareForUpdate(ctx, newSeed, oldSeed)
					Expect(newSeed.Spec.Taints).NotTo(ContainElement(core.SeedTaint{Key: core.DeprecatedSeedTaintDisableDNS}))
				})
			})
		})
	})
})
