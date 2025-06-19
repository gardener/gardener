// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gardenerupgrade

import (
	. "github.com/onsi/ginkgo/v2"
	"k8s.io/utils/ptr"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/gardener/gardener/test/e2e/gardener"
	. "github.com/gardener/gardener/test/e2e/gardener/shoot"
	"github.com/gardener/gardener/test/e2e/gardener/shoot/internal/zerodowntimevalidator"
)

var _ = Describe("Gardener Upgrade Tests", func() {
	Describe("Create Shoot, Upgrade Gardener version, Delete Shoot", func() {
		test := func(s *ShootContext) {
			zeroDowntimeValidatorJob := &zerodowntimevalidator.Job{}

			Describe("Pre-Upgrade"+gardenerInfoPreUpgrade, Label("pre-upgrade"), func() {
				ItShouldCreateShoot(s)
				ItShouldWaitForShootToBeReconciledAndHealthy(s)
				ItShouldGetResponsibleSeed(s)
				ItShouldInitializeSeedClient(s)

				zeroDowntimeValidatorJob.ItShouldDeployJob(s)
				zeroDowntimeValidatorJob.ItShouldWaitForJobToBeReady(s)
			})

			Describe("Post-Upgrade"+gardenerInfoPostUpgrade, Label("post-upgrade"), func() {
				ItShouldGetResponsibleSeed(s)
				ItShouldInitializeSeedClient(s)

				zeroDowntimeValidatorJob.ItShouldEnsureThereWasNoDowntime(s)
				zeroDowntimeValidatorJob.AfterAllDeleteJob(s)

				// This tests that we can delete a Shoot which was not yet reconciled with the current Gardener version.
				itShouldEnsureShootWasReconciledWithPreviousGardenerVersion(s)
				ItShouldDeleteShoot(s)
				ItShouldWaitForShootToBeDeleted(s)
			})
		}

		Context("Shoot with workers", Ordered, func() {
			shoot := DefaultShoot("e2e-upgrade")

			// add two more worker pools with in-place update strategies
			shoot.Spec.Provider.Workers = append(shoot.Spec.Provider.Workers,
				DefaultWorker("auto", ptr.To(gardencorev1beta1.AutoInPlaceUpdate)),
				DefaultWorker("manual", ptr.To(gardencorev1beta1.ManualInPlaceUpdate)),
			)

			test(NewTestContext().ForShoot(shoot))
		})

		Context("Workerless Shoot", Label("workerless"), Ordered, func() {
			test(NewTestContext().ForShoot(DefaultWorkerlessShoot("e2e-upgrade")))
		})
	})
})
