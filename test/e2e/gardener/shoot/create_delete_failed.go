// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shoot

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/gardener/gardener/test/e2e"
	. "github.com/gardener/gardener/test/e2e/gardener"
)

var _ = Describe("Shoot Tests", Label("Shoot", "default"), func() {
	Describe("Create and Delete Failed Shoot", func() {
		Context("Shoot with invalid DNS configuration", Ordered, func() {
			var s *ShootContext

			BeforeTestSetup(func() {
				shoot := DefaultShoot("e2e-invalid-dns")
				shoot.Spec.DNS = &gardencorev1beta1.DNS{
					Domain: ptr.To("shoot.non-existing-domain"),
				}

				s = NewTestContext().ForShoot(shoot)
			})

			ItShouldCreateShoot(s)

			It("Wait until last operation in Shoot is set to Failed", func(ctx SpecContext) {
				Eventually(ctx, s.GardenKomega.Object(s.Shoot)).Should(
					HaveField("Status.LastOperation.State", Equal(gardencorev1beta1.LastOperationStateFailed)),
				)
			}, SpecTimeout(time.Minute))

			ItShouldDeleteShoot(s)
			ItShouldWaitForShootToBeDeleted(s)
		})
	})
})
