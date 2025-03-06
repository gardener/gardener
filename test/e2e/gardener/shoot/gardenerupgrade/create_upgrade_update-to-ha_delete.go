// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gardenerupgrade

import (
	. "github.com/onsi/ginkgo/v2"

	. "github.com/gardener/gardener/test/e2e/gardener"
	. "github.com/gardener/gardener/test/e2e/gardener/shoot/internal"
	"github.com/gardener/gardener/test/e2e/gardener/shoot/internal/highavailability"
)

var _ = Describe("Gardener Upgrade Tests", func() {
	Describe("Create Shoot, Upgrade Gardener version, Update to High Availability, Delete Shoot", func() {
		test := func(s *ShootContext) {
			Describe("Pre-Upgrade"+gardenerInfoPreUpgrade, Label("pre-upgrade"), func() {
				s.Shoot.Spec.ControlPlane = nil

				ItShouldCreateShoot(s)
				ItShouldWaitForShootToBeReconciledAndHealthy(s)
			})

			Describe("Post-Upgrade"+gardenerInfoPostUpgrade, Label("post-upgrade"), func() {
				ItShouldReadShootFromAPIServer(s)
				itShouldEnsureShootWasReconciledWithPreviousGardenerVersion(s)

				ItShouldGetResponsibleSeed(s)
				ItShouldInitializeSeedClient(s)

				ItShouldUpdateShootToHighAvailability(s, GetFailureToleranceType())
				ItShouldWaitForShootToBeReconciledAndHealthy(s)

				highavailability.VerifyHighAvailability(s)
				itShouldEnsureShootWasReconciledWithCurrentGardenerVersion(s)

				ItShouldDeleteShoot(s)
				ItShouldWaitForShootToBeDeleted(s)
			})
		}

		Context("Shoot with workers", Label("high-availability"), Ordered, func() {
			test(NewTestContext().ForShoot(DefaultShoot("e2e-upg-ha")))
		})

		Context("Workerless Shoot", Label("high-availability", "workerless"), Ordered, func() {
			test(NewTestContext().ForShoot(DefaultWorkerlessShoot("e2e-upg-ha")))
		})
	})
})
