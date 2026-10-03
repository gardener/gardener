// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package shoot

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	shootextensionactuator "github.com/gardener/gardener/pkg/provider-local/controller/extension/shoot"
	. "github.com/gardener/gardener/test/e2e"
	. "github.com/gardener/gardener/test/e2e/gardener"
)

var _ = Describe("Shoot Tests", Label("Shoot", "default"), func() {
	test := func(tc *ShootContext) {
		BeforeTestSetup(func() {
			metav1.SetMetaDataAnnotation(&tc.Shoot.ObjectMeta, shootextensionactuator.AnnotationTestForceDeleteShoot, "true")
		})

		Describe("Create and Force Delete Shoot", Label("force-delete"), func() {
			ItShouldCreateShoot(tc)
			ItShouldWaitForShootToBeReconciledAndHealthy(tc)
			ItShouldAnnotateShoot(tc, map[string]string{
				v1beta1constants.ShootIgnore: "true",
			})
			ItShouldDeleteShoot(tc)

			It("Add ErrorInfraDependencies to LastErrors", func(ctx SpecContext) {
				patch := client.MergeFrom(tc.Shoot.DeepCopy())
				tc.Shoot.Status.LastErrors = []gardencorev1beta1.LastError{{
					Codes: []gardencorev1beta1.ErrorCode{gardencorev1beta1.ErrorInfraDependencies},
				}}

				Eventually(ctx, func() error {
					return tc.GardenClient.Status().Patch(ctx, tc.Shoot, patch)
				}).Should(Succeed())
			}, SpecTimeout(time.Minute))

			ItShouldAnnotateShoot(tc, map[string]string{
				v1beta1constants.AnnotationConfirmationForceDeletion: "true",
				v1beta1constants.ShootIgnore:                         "false",
			})

			ItShouldWaitForShootToBeDeleted(tc)
		})
	}

	Context("Shoot with workers", Ordered, func() {
		test(NewTestContext().Init().ForShoot(DefaultShoot("e2e-force-delete")))
	})

	Context("Hibernated Shoot", Ordered, func() {
		shoot := DefaultShoot("e2e-fd-hib")
		shoot.Spec.Hibernation = &gardencorev1beta1.Hibernation{
			Enabled: new(true),
		}

		test(NewTestContext().Init().ForShoot(shoot))
	})

	Context("Workerless Shoot", Ordered, func() {
		test(NewTestContext().Init().ForShoot(DefaultWorkerlessShoot("e2e-fd")))
	})
})
