// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gardenerupgrade

import (
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/gardener/gardener/test/e2e"
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

				ItShouldHibernateShoot(s)
				ItShouldWaitForShootToBeReconciledAndHealthy(s)
			})

			Describe("Post-Upgrade"+gardenerInfoPostUpgrade, Label("post-upgrade"), func() {
				BeforeTestSetup(func() {
					It("Read Shoot from API server", func(ctx SpecContext) {
						Eventually(ctx, s.GardenKomega.Get(s.Shoot)).Should(Succeed())
					}, SpecTimeout(time.Minute))
				})

				It("should ensure Shoot was created with previous Gardener version", func() {
					Expect(s.Shoot.Status.Gardener.Version).Should(Equal(gardenerPreviousVersion))
				})

				ItShouldGetResponsibleSeed(s)
				ItShouldInitializeSeedClient(s)

				ItShouldUpdateShootToHighAvailability(s, getFailureToleranceType())
				ItShouldWaitForShootToBeReconciledAndHealthy(s)
				highavailability.VerifyHighAvailabilityUpdate(s)

				It("should ensure Shoot was updated to high-availability with current Gardener version", func() {
					Expect(s.Shoot.Status.Gardener.Version).Should(Equal(gardenerCurrentVersion))
				})

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

// getFailureToleranceType returns a failureToleranceType based on env variable SHOOT_FAILURE_TOLERANCE_TYPE value
func getFailureToleranceType() gardencorev1beta1.FailureToleranceType {
	var failureToleranceType gardencorev1beta1.FailureToleranceType

	switch os.Getenv("SHOOT_FAILURE_TOLERANCE_TYPE") {
	case "zone":
		failureToleranceType = gardencorev1beta1.FailureToleranceTypeZone
	case "node":
		failureToleranceType = gardencorev1beta1.FailureToleranceTypeNode
	}
	return failureToleranceType
}
