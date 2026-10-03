// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package gardenerupgrade

import (
	. "github.com/onsi/ginkgo/v2"

	. "github.com/gardener/gardener/test/e2e/gardener"
	"github.com/gardener/gardener/test/e2e/gardener/seed"
	. "github.com/gardener/gardener/test/e2e/gardener/shoot"
	"github.com/gardener/gardener/test/e2e/gardener/shoot/internal/highavailability"
)

var _ = Describe("Gardener Upgrade Tests", func() {
	Describe("Create Shoot, Upgrade Gardener version, Update to High Availability, Delete Shoot", func() {
		test := func(tc *ShootContext) {
			Describe("Pre-Upgrade"+gardenerInfoPreUpgrade, Label("pre-upgrade"), func() {
				tc.Shoot.Spec.ControlPlane = nil

				ItShouldCreateShoot(tc)
				ItShouldWaitForShootToBeReconciledAndHealthy(tc)
			})

			Describe("Post-Upgrade"+gardenerInfoPostUpgrade, Label("post-upgrade"), func() {
				itShouldEnsureShootWasReconciledWithPreviousGardenerVersion(tc)

				ItShouldGetResponsibleSeed(tc)
				seed.ItShouldInitializeSeedClient(&tc.SeedContext)

				ItShouldUpdateShootToHighAvailability(tc, GetFailureToleranceType())
				ItShouldWaitForShootToBeReconciledAndHealthy(tc)

				highavailability.VerifyHighAvailability(tc)
				itShouldEnsureShootWasReconciledWithCurrentGardenerVersion(tc)

				ItShouldDeleteShoot(tc)
				ItShouldWaitForShootToBeDeleted(tc)
			})
		}

		Context("Shoot with workers", Label("high-availability"), Ordered, func() {
			test(NewTestContext().Init().ForShoot(DefaultShoot("e2e-upg-ha")))
		})

		Context("Workerless Shoot", Label("high-availability", "workerless"), Ordered, func() {
			test(NewTestContext().Init().ForShoot(DefaultWorkerlessShoot("e2e-upg-ha")))
		})
	})
})
