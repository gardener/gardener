// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gardenerupgrade

import (
	. "github.com/onsi/ginkgo/v2"

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
				ItShouldReadShootFromAPIServer(s)

				// This tests that we can wake-up a Shoot which was hibernated with the previous Gardener version.
				itShouldEnsureShootWasReconciledWithPreviousGardenerVersion(s)
				ItShouldWakeUpShoot(s)
				ItShouldWaitForShootToBeReconciledAndHealthy(s)
				itShouldEnsureShootWasReconciledWithCurrentGardenerVersion(s)

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
