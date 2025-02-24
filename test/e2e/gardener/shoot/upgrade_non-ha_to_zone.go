// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shoot

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	e2e "github.com/gardener/gardener/test/e2e/gardener"
	"github.com/gardener/gardener/test/utils/shoots/update/highavailability"
)

var _ = Describe("Shoot Tests", Label("Shoot", "high-availability", "upgrade-to-zone"), func() {
	test := func(shoot *gardencorev1beta1.Shoot) {
		f := defaultShootCreationFramework()
		f.Shoot = shoot

		f.Shoot.Spec.ControlPlane = nil

		It("Create, Upgrade (non-HA to HA with failure tolerance type 'zone') and Delete Shoot", Offset(1), func() {
			By("Create Shoot")
			ctx, cancel := context.WithTimeout(parentCtx, 30*time.Minute)
			defer cancel()

			Expect(f.CreateShootAndWaitForCreation(ctx, false)).To(Succeed())
			f.Verify()

			// TODO: add back VerifyInClusterAccessToAPIServer once this test has been refactored to ordered containers
			// if !v1beta1helper.IsWorkerless(s.Shoot) {
			// 	inclusterclient.VerifyInClusterAccessToAPIServer(s)
			// }

			By("Upgrade Shoot (non-HA to HA with failure tolerance type 'zone')")
			ctx, cancel = context.WithTimeout(parentCtx, 30*time.Minute)
			defer cancel()
			highavailability.UpgradeAndVerify(ctx, f.ShootFramework, gardencorev1beta1.FailureToleranceTypeZone)

			// TODO: add back VerifyInClusterAccessToAPIServer once this test has been refactored to ordered containers
			// if !v1beta1helper.IsWorkerless(s.Shoot) {
			// 	inclusterclient.VerifyInClusterAccessToAPIServer(s)
			// }

			By("Delete Shoot")
			ctx, cancel = context.WithTimeout(parentCtx, 20*time.Minute)
			defer cancel()
			Expect(f.DeleteShootAndWaitForDeletion(ctx, f.Shoot)).To(Succeed())
		})
	}

	Context("Shoot with workers", func() {
		test(e2e.DefaultShoot("e2e-upd-zone"))
	})

	Context("Workerless Shoot", Label("workerless"), func() {
		test(e2e.DefaultWorkerlessShoot("e2e-upd-zone"))
	})
})
