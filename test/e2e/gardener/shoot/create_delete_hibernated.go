// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package shoot

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/gardener/gardener/test/e2e/gardener"
	"github.com/gardener/gardener/test/e2e/gardener/seed"
)

var _ = Describe("Shoot Tests", Label("Shoot", "default"), func() {
	Describe("Create and Delete Hibernated Shoot", Label("hibernated"), func() {
		test := func(tc *ShootContext) {
			tc.Shoot.Spec.Hibernation = &gardencorev1beta1.Hibernation{
				Enabled: new(true),
			}

			BeforeAll(func() {
				tc.Init()
			})

			ItShouldCreateShoot(tc)
			ItShouldWaitForShootToBeReconciledAndHealthy(tc)
			ItShouldGetResponsibleSeed(tc)
			seed.ItShouldInitializeSeedClient(&tc.SeedContext)

			It("should not have any control plane pods", func(ctx SpecContext) {
				Eventually(ctx,
					tc.SeedKomega.ObjectList(&corev1.PodList{}, client.InNamespace(tc.Shoot.Status.TechnicalID)),
				).Should(
					HaveField("Items", BeEmpty()),
				)
			}, SpecTimeout(time.Minute))

			ItShouldDeleteShoot(tc)
			ItShouldWaitForShootToBeDeleted(tc)
		}

		Context("Shoot with workers", Ordered, func() {
			test(NewShootContext(DefaultShoot("e2e-hib")))
		})

		Context("Workerless Shoot", Label("workerless"), Ordered, func() {
			test(NewShootContext(DefaultWorkerlessShoot("e2e-hib")))
		})
	})
})
