// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gardenerupgrade

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/gardener/gardener/test/e2e"
	. "github.com/gardener/gardener/test/e2e/gardener"
	. "github.com/gardener/gardener/test/e2e/gardener/shoot/internal"
)

var _ = Describe("Gardener Upgrade Tests", func() {
	Describe("Create and Hibernate Shoot, Upgrade Gardener version, Wake Up and Delete Shoot", func() {
		test := func(s *ShootContext) {
			Describe("Pre-Upgrade"+gardenerInfoPreUpgrade, Label("pre-upgrade"), func() {
				ItShouldCreateShoot(s)
				ItShouldWaitForShootToBeReconciledAndHealthy(s)

				ItShouldHibernateShoot(s)
				ItShouldWaitForShootToBeReconciledAndHealthy(s)
			})

			Describe("Post-Upgrade"+gardenerInfoPostUpgrade, Label("post-upgrade"), func() {
				BeforeTestSetup(func() {
					It("Read Shoot from API server", func(ctx SpecContext) {
						Eventually(ctx, s.GardenKomega.Get(s.Shoot)).Should(Succeed())
					}, SpecTimeout(time.Minute))
				})

				It("should ensure Shoot was hibernated with previous Gardener version", func() {
					Expect(s.Shoot.Status.Gardener.Version).Should(Equal(gardenerPreviousVersion))
				})

				ItShouldWakeUpShoot(s)
				ItShouldWaitForShootToBeReconciledAndHealthy(s)

				It("should ensure Shoot was woken up with current Gardener version", func() {
					Expect(s.Shoot.Status.Gardener.Version).Should(Equal(gardenerCurrentVersion))
				})

				ItShouldDeleteShoot(s)
				ItShouldWaitForShootToBeDeleted(s)
			})
		}

		Context("Shoot with workers", Ordered, func() {
			test(NewTestContext().ForShoot(DefaultShoot("e2e-upg-hib")))
		})

		Context("Workerless Shoot", Label("workerless"), Ordered, func() {
			test(NewTestContext().ForShoot(DefaultWorkerlessShoot("e2e-upg-hib")))
		})
	})
})
