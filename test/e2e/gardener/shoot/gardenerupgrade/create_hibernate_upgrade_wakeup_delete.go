// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package gardenerupgrade

import (
	. "github.com/onsi/ginkgo/v2"

	. "github.com/gardener/gardener/test/e2e/gardener"
	. "github.com/gardener/gardener/test/e2e/gardener/shoot"
)

var _ = Describe("Gardener Upgrade Tests", func() {
	Describe("Create and Hibernate Shoot, Upgrade Gardener version, Wake Up and Delete Shoot", func() {
		test := func(tc *ShootContext) {
			BeforeAll(func() {
				tc.Init()
			})

			Describe("Pre-Upgrade"+gardenerInfoPreUpgrade, Label("pre-upgrade"), func() {
				ItShouldCreateShoot(tc)
				ItShouldWaitForShootToBeReconciledAndHealthy(tc)

				ItShouldHibernateShoot(tc)
				ItShouldWaitForShootToBeReconciledAndHealthy(tc)
			})

			Describe("Post-Upgrade"+gardenerInfoPostUpgrade, Label("post-upgrade"), func() {
				// This tests that we can wake-up a Shoot which was hibernated with the previous Gardener version.
				itShouldEnsureShootWasReconciledWithPreviousGardenerVersion(tc)
				ItShouldWakeUpShoot(tc)
				ItShouldWaitForShootToBeReconciledAndHealthy(tc)
				itShouldEnsureShootWasReconciledWithCurrentGardenerVersion(tc)

				ItShouldDeleteShoot(tc)
				ItShouldWaitForShootToBeDeleted(tc)
			})
		}

		Context("Shoot with workers", Ordered, func() {
			test(NewShootContext(DefaultShoot("e2e-upg-hib")))
		})

		Context("Workerless Shoot", Label("workerless"), Ordered, func() {
			test(NewShootContext(DefaultWorkerlessShoot("e2e-upg-hib")))
		})
	})
})
