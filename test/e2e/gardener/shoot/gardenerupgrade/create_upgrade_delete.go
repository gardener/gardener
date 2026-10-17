// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package gardenerupgrade

import (
	. "github.com/onsi/ginkgo/v2"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/gardener/gardener/test/e2e/gardener"
	"github.com/gardener/gardener/test/e2e/gardener/seed"
	. "github.com/gardener/gardener/test/e2e/gardener/shoot"
	"github.com/gardener/gardener/test/e2e/gardener/shoot/internal/zerodowntimevalidator"
)

var _ = Describe("Gardener Upgrade Tests", func() {
	Describe("Create Shoot, Upgrade Gardener version, Delete Shoot", func() {
		test := func(tc *ShootContext) {
			BeforeAll(func() {
				tc.Init()
			})

			zeroDowntimeValidatorJob := &zerodowntimevalidator.Job{}

			Describe("Pre-Upgrade"+gardenerInfoPreUpgrade, Label("pre-upgrade"), func() {
				ItShouldCreateShoot(tc)
				ItShouldWaitForShootToBeReconciledAndHealthy(tc)
				ItShouldGetResponsibleSeed(tc)
				seed.ItShouldInitializeSeedClient(tc.SeedContext)

				zeroDowntimeValidatorJob.ItShouldDeployJob(tc)
				zeroDowntimeValidatorJob.ItShouldWaitForJobToBeReady(tc)
			})

			Describe("Post-Upgrade"+gardenerInfoPostUpgrade, Label("post-upgrade"), func() {
				ItShouldGetResponsibleSeed(tc)
				seed.ItShouldInitializeSeedClient(tc.SeedContext)

				zeroDowntimeValidatorJob.ItShouldEnsureThereWasNoDowntime(tc)
				zeroDowntimeValidatorJob.AfterAllDeleteJob(tc)

				// This tests that we can delete a Shoot which was not yet reconciled with the current Gardener version.
				itShouldEnsureShootWasReconciledWithPreviousGardenerVersion(tc)
				ItShouldDeleteShoot(tc)
				ItShouldWaitForShootToBeDeleted(tc)
			})
		}

		Context("Shoot with workers", Ordered, func() {
			shoot := DefaultShoot("e2e-upgrade")

			// add two more worker pools with in-place update strategies
			shoot.Spec.Provider.Workers = append(shoot.Spec.Provider.Workers,
				DefaultWorker("auto", new(gardencorev1beta1.AutoInPlaceUpdate)),
				DefaultWorker("manual", new(gardencorev1beta1.ManualInPlaceUpdate)),
			)

			test(NewShootContext(shoot))
		})

		Context("Workerless Shoot", Label("workerless"), Ordered, func() {
			test(NewShootContext(DefaultWorkerlessShoot("e2e-upgrade")))
		})
	})
})
