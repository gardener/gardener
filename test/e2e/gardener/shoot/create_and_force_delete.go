// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shoot

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	shootextensionactuator "github.com/gardener/gardener/pkg/provider-local/controller/extension/shoot"
	. "github.com/gardener/gardener/test/e2e"
	. "github.com/gardener/gardener/test/e2e/gardener"
	. "github.com/gardener/gardener/test/e2e/gardener/shoot/internal"
)

var _ = Describe("Shoot Tests", Label("Shoot", "default"), func() {
	test := func(s *ShootContext) {
		BeforeTestSetup(func() {
			metav1.SetMetaDataAnnotation(&s.Shoot.ObjectMeta, shootextensionactuator.AnnotationTestForceDeleteShoot, "true")
		})

		Describe("Create and Force Delete Shoot", Label("force-delete"), func() {
			ItShouldCreateShoot(s)
			ItShouldWaitForShootToBeReconciledAndHealthy(s)
			ItShouldSetShootAnnotation(s, v1beta1constants.ShootIgnore, "true")
			ItShouldDeleteShoot(s)

			It("Add ErrorInfraDependencies to LastErrors", func(ctx SpecContext) {
				patch := client.MergeFrom(s.Shoot.DeepCopy())
				s.Shoot.Status.LastErrors = []gardencorev1beta1.LastError{{
					Codes: []gardencorev1beta1.ErrorCode{gardencorev1beta1.ErrorInfraDependencies},
				}}

				Eventually(ctx, func() error {
					return s.GardenClient.Status().Patch(ctx, s.Shoot, patch)
				}).Should(Succeed())
			}, SpecTimeout(time.Minute))

			ItShouldSetShootAnnotation(s, v1beta1constants.AnnotationConfirmationForceDeletion, "true")
			ItShouldSetShootAnnotation(s, v1beta1constants.ShootIgnore, "false")

			// manually trigger reconcilation after stop ignoring the shoot
			ItShouldSetShootAnnotation(s, v1beta1constants.GardenerOperation, v1beta1constants.GardenerOperationReconcile)

			ItShouldWaitForShootToBeDeleted(s)
		})
	}

	Context("Shoot with workers", Ordered, func() {
		test(NewTestContext().ForShoot(DefaultShoot("e2e-force-delete")))
	})

	Context("Hibernated Shoot", Ordered, func() {
		shoot := DefaultShoot("e2e-fd-hib")
		shoot.Spec.Hibernation = &gardencorev1beta1.Hibernation{
			Enabled: ptr.To(true),
		}

		test(NewTestContext().ForShoot(shoot))
	})

	Context("Workerless Shoot", Ordered, func() {
		test(NewTestContext().ForShoot(DefaultWorkerlessShoot("e2e-fd-wl")))
	})
})
