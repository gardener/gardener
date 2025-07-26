// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package inplace

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	. "github.com/gardener/gardener/test/e2e/gardener"
)

// ItShouldVerifyInPlaceUpdateStart verifies that the starting of in-place update  by checking the
// .status.inPlaceUpdates and the ManualInPlaceWorkersUpdated constraint of the Shoot.
func ItShouldVerifyInPlaceUpdateStart(s *ShootContext, hasAutoInplaceUpdate, hasManualInplaceUpdate bool) {
	GinkgoHelper()

	It("Verify in-place update start", func(ctx SpecContext) {
		VerifyInPlaceUpdateStart(ctx, s.Log, s.GardenClient, s.Shoot, hasAutoInplaceUpdate, hasManualInplaceUpdate)
	}, SpecTimeout(2*time.Minute))
}

// VerifyInPlaceUpdateStart verifies that the in-place update has started by checking the
// .status.inPlaceUpdates and the ManualInPlaceWorkersUpdated constraint of the Shoot.
func VerifyInPlaceUpdateStart(ctx context.Context, log logr.Logger, gardenClient client.Client, shoot *gardencorev1beta1.Shoot, hasAutoInplaceUpdate, hasManualInplaceUpdate bool) {
	GinkgoHelper()

	log = log.WithValues("shoot", client.ObjectKeyFromObject(shoot))
	log.Info("Verifying in-place update start", "hasWorkerWithAutoInplaceUpdate", hasAutoInplaceUpdate, "hasWorkerWithManualInplaceUpdate", hasManualInplaceUpdate)

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
}

// ItShouldVerifyInPlaceUpdateCompletion verifies that the in-place update was completed successfully by checking the
// .status.inPlaceUpdates and the ManualInPlaceWorkersUpdated constraint of the Shoot.
func ItShouldVerifyInPlaceUpdateCompletion(s *ShootContext) {
	GinkgoHelper()

	It("Verify in-place update completion", func(ctx SpecContext) {
		VerifyInPlaceUpdateCompletion(ctx, s.Log, s.GardenClient, s.Shoot)
	}, SpecTimeout(5*time.Minute))
}

// VerifyInPlaceUpdateCompletion verifies that the in-place update was completed successfully by checking the
// .status.inPlaceUpdates and the ManualInPlaceWorkersUpdated constraint of the Shoot.
func VerifyInPlaceUpdateCompletion(ctx context.Context, log logr.Logger, gardenClient client.Client, shoot *gardencorev1beta1.Shoot) {
	GinkgoHelper()

	log = log.WithValues("shoot", client.ObjectKeyFromObject(shoot))
	log.Info("Verifying in-place update completion")

	Eventually(ctx, func(g Gomega) {
		g.Expect(gardenClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).Should(Succeed())

		g.Expect(shoot.Status.InPlaceUpdates).To(BeNil())
		g.Expect(shoot.Status.Constraints).NotTo(ContainCondition(
			OfType(gardencorev1beta1.ShootManualInPlaceWorkersUpdated),
		))
	}).Should(Succeed())
}
