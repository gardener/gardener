// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shoot

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/gardener/gardener/test/e2e"
	. "github.com/gardener/gardener/test/e2e/gardener"
	. "github.com/gardener/gardener/test/e2e/gardener/shoot/internal"
)

var _ = Describe("Shoot Tests", Label("Shoot", "default"), func() {
	Describe("Create and Delete Hibernated Shoot", Label("hibernated"), func() {
		test := func(s *ShootContext) {
			BeforeTestSetup(func() {
				s.Shoot.Spec.Hibernation = &gardencorev1beta1.Hibernation{
					Enabled: ptr.To(true),
				}
			})

			ItShouldCreateShoot(s)
			ItShouldWaitForShootToBeReconciledAndHealthy(s)
			ItShouldGetResponsibleSeed(s)
			ItShouldInitializeSeedClient(s)

			It("should not have any control plane pods", func(ctx SpecContext) {
				Eventually(ctx,
					s.SeedKomega.ObjectList(&corev1.PodList{}, client.InNamespace(s.Shoot.Status.TechnicalID)),
				).Should(
					HaveField("Items", BeEmpty()),
				)
			}, SpecTimeout(time.Minute))

			ItShouldDeleteShoot(s)
			ItShouldWaitForShootToBeDeleted(s)
		}

		Context("Shoot with workers", Ordered, func() {
			test(NewTestContext().ForShoot(DefaultShoot("e2e-hib")))
		})

		Context("Workerless Shoot", Label("workerless"), Ordered, func() {
			test(NewTestContext().ForShoot(DefaultWorkerlessShoot("e2e-hib")))
		})
	})
})
