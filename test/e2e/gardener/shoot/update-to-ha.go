// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package shoot

import (
	. "github.com/onsi/ginkgo/v2"

	v1beta1helper "github.com/gardener/gardener/pkg/api/core/v1beta1/helper"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/gardener/gardener/test/e2e/gardener"
	"github.com/gardener/gardener/test/e2e/gardener/seed"
	"github.com/gardener/gardener/test/e2e/gardener/shoot/internal/highavailability"
	"github.com/gardener/gardener/test/e2e/gardener/shoot/internal/inclusterclient"
)

var _ = Describe("Shoot Tests", Label("Shoot", "high-availability"), func() {
	container := func(shootName string, failureToleranceType gardencorev1beta1.FailureToleranceType) {
		test := func(tc *ShootContext) {
			tc.Shoot.Spec.ControlPlane = nil

			ItShouldCreateShoot(tc)
			ItShouldWaitForShootToBeReconciledAndHealthy(tc)
			ItShouldGetResponsibleSeed(tc)
			seed.ItShouldInitializeSeedClient(&tc.SeedContext)
			ItShouldInitializeShootClient(tc)

			if !v1beta1helper.IsWorkerless(tc.Shoot) {
				inclusterclient.VerifyInClusterAccessToAPIServer(tc)
			}

			ItShouldUpdateShootToHighAvailability(tc, failureToleranceType)
			ItShouldWaitForShootToBeReconciledAndHealthy(tc)
			highavailability.VerifyHighAvailability(tc)

			if !v1beta1helper.IsWorkerless(tc.Shoot) {
				inclusterclient.VerifyInClusterAccessToAPIServer(tc)
			}

			ItShouldDeleteShoot(tc)
			ItShouldWaitForShootToBeDeleted(tc)
		}

		Context("Shoot with workers", Ordered, func() {
			test(NewTestContext().Init().ForShoot(DefaultShoot(shootName)))
		})

		Context("Shoot with workers and overlapping CIDR ranges", Ordered, func() {
			test(NewTestContext().Init().ForShoot(DefaultOverlappingShoot(shootName)))
		})

		Context("Workerless Shoot", Label("workerless"), Ordered, func() {
			test(NewTestContext().Init().ForShoot(DefaultWorkerlessShoot(shootName)))
		})
	}

	Describe("Update from non-HA to HA with failure tolerance type 'node'", Label("update-to-node"), func() {
		container("e2e-upd-node", gardencorev1beta1.FailureToleranceTypeNode)
	})

	Describe("Update from non-HA to HA with failure tolerance type 'zone'", Label("update-to-zone"), func() {
		container("e2e-upd-zone", gardencorev1beta1.FailureToleranceTypeZone)
	})
})
