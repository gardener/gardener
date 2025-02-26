// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shoot

import (
	. "github.com/onsi/ginkgo/v2"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	. "github.com/gardener/gardener/test/e2e/gardener"
	. "github.com/gardener/gardener/test/e2e/gardener/shoot/internal"
	"github.com/gardener/gardener/test/e2e/gardener/shoot/internal/highavailability"
	"github.com/gardener/gardener/test/e2e/gardener/shoot/internal/inclusterclient"
)

var _ = Describe("Shoot Tests", Label("Shoot", "high-availability"), func() {
	container := func(shootName string, failureToleranceType gardencorev1beta1.FailureToleranceType) {
		test := func(s *ShootContext) {
			s.Shoot.Spec.ControlPlane = nil

			ItShouldCreateShoot(s)
			ItShouldWaitForShootToBeReconciledAndHealthy(s)
			ItShouldGetResponsibleSeed(s)
			ItShouldInitializeSeedClient(s)
			ItShouldInitializeShootClient(s)

			if !v1beta1helper.IsWorkerless(s.Shoot) {
				inclusterclient.VerifyInClusterAccessToAPIServer(s)
			}

			ItShouldUpdateShootToHighAvailability(s, failureToleranceType)
			ItShouldWaitForShootToBeReconciledAndHealthy(s)
			highavailability.VerifyHighAvailabilityUpdate(s)

			if !v1beta1helper.IsWorkerless(s.Shoot) {
				inclusterclient.VerifyInClusterAccessToAPIServer(s)
			}

			ItShouldDeleteShoot(s)
			ItShouldWaitForShootToBeDeleted(s)
		}

		Context("Shoot with workers", Ordered, func() {
			test(NewTestContext().ForShoot(DefaultShoot(shootName)))
		})

		Context("Workerless Shoot", Label("workerless"), Ordered, func() {
			test(NewTestContext().ForShoot(DefaultWorkerlessShoot(shootName)))
		})
	}

	Describe("Update from non-HA to HA with failure tolerance type 'node'", Label("upgrade-to-node"), func() {
		container("e2e-upd-node", gardencorev1beta1.FailureToleranceTypeNode)
	})

	Describe("Update from non-HA to HA with failure tolerance type 'zone'", Label("upgrade-to-zone"), func() {
		container("e2e-upd-zone", gardencorev1beta1.FailureToleranceTypeZone)
	})
})
