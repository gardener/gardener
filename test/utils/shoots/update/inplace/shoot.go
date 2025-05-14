// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package inplace

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

// ItShouldVerifyInPlaceUpdateStart verifies that the starting of in-place update  by checking the
// .status.inPlaceUpdates and the ManualInPlaceWorkersUpdated constraint of the Shoot.
func ItShouldVerifyInPlaceUpdateStart(gardenClient client.Client, shoot *gardencorev1beta1.Shoot, hasAutoInplaceUpdate, hasManualInplaceUpdate bool) {
	GinkgoHelper()

	It("Verify in-place update start", func(ctx SpecContext) {
		Eventually(ctx, func(g Gomega) {
			g.Expect(gardenClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).Should(Succeed())

			g.Expect(shoot.Status.InPlaceUpdates).NotTo(BeNil())
			g.Expect(shoot.Status.InPlaceUpdates.PendingWorkerUpdates).NotTo(BeNil())
			if hasAutoInplaceUpdate {
				g.Expect(shoot.Status.InPlaceUpdates.PendingWorkerUpdates.AutoInPlaceUpdate).NotTo(BeEmpty())
			}
			if hasManualInplaceUpdate {
				g.Expect(shoot.Status.InPlaceUpdates.PendingWorkerUpdates.ManualInPlaceUpdate).NotTo(BeEmpty())
				g.Expect(shoot.Status.Constraints).To(ContainCondition(
					OfType(gardencorev1beta1.ShootManualInPlaceWorkersUpdated),
					WithReason("WorkerPoolsWithManualInPlaceUpdateStrategyPending"),
					Or(WithStatus(gardencorev1beta1.ConditionFalse), WithStatus(gardencorev1beta1.ConditionProgressing)),
				))
			}
		}).Should(Succeed())
	}, SpecTimeout(2*time.Minute))
}

// ItShouldVerifyInPlaceUpdateCompletion verifies that the in-place update was completed successfully by checking the
// .status.inPlaceUpdates and the ManualInPlaceWorkersUpdated constraint of the Shoot.
func ItShouldVerifyInPlaceUpdateCompletion(gardenClient client.Client, shoot *gardencorev1beta1.Shoot) {
	GinkgoHelper()

	It("Verify in-place update completion", func(ctx SpecContext) {
		Eventually(ctx, func(g Gomega) {
			g.Expect(gardenClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).Should(Succeed())

			g.Expect(shoot.Status.InPlaceUpdates).To(BeNil())
			g.Expect(shoot.Status.Constraints).NotTo(ContainCondition(
				OfType(gardencorev1beta1.ShootManualInPlaceWorkersUpdated),
			))
		}).Should(Succeed())
	}, SpecTimeout(2*time.Minute))
}
