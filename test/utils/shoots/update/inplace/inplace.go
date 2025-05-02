// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package inplace

import (
	"context"
	"time"

	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	. "github.com/gardener/gardener/test/e2e/gardener"
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

// LabelManualInPlaceNodesWithSelectedForUpdate labels all manual in-place nodes with the selected-for-update label.
// In the actual scenario, this should be done by the user, but for testing purposes, we do it here.
func LabelManualInPlaceNodesWithSelectedForUpdate(ctx context.Context, shootClient client.Client, shoot *gardencorev1beta1.Shoot) {
	GinkgoHelper()

	for _, pool := range shoot.Spec.Provider.Workers {
		if !v1beta1helper.IsUpdateStrategyManualInPlace(pool.UpdateStrategy) {
			continue
		}
		nodeList := &corev1.NodeList{}
		Eventually(ctx, func(g Gomega) {
			g.Expect(shootClient.List(ctx, nodeList, client.MatchingLabels{v1beta1constants.LabelWorkerPool: pool.Name})).To(Succeed())
		}).Should(Succeed())

		for _, node := range nodeList.Items {
			if metav1.HasLabel(node.ObjectMeta, machinev1alpha1.LabelKeyNodeSelectedForUpdate) {
				continue
			}

			metav1.SetMetaDataLabel(&node.ObjectMeta, machinev1alpha1.LabelKeyNodeSelectedForUpdate, "true")
			Eventually(ctx, func(g Gomega) {
				g.Expect(shootClient.Update(ctx, &node)).To(Succeed())
			}).Should(Succeed(), "node %s should be labeled", node.Name)
		}
	}
}

// FindAllNodesOfInPlaceWorker finds all nodes of in-place workers and returns their names.
func FindAllNodesOfInPlaceWorker(ctx context.Context, shootClient client.Client, shoot *gardencorev1beta1.Shoot) sets.Set[string] {
	GinkgoHelper()

	nodesOfInPlaceWorkers := sets.New[string]()

	for _, pool := range shoot.Spec.Provider.Workers {
		if !v1beta1helper.IsUpdateStrategyInPlace(pool.UpdateStrategy) {
			continue
		}
		nodeList := &corev1.NodeList{}
		Eventually(ctx, func(g Gomega) {
			g.Expect(shootClient.List(context.Background(), nodeList, client.MatchingLabels{v1beta1constants.LabelWorkerPool: pool.Name})).To(Succeed())
		}).Should(Succeed())

		for _, node := range nodeList.Items {
			nodesOfInPlaceWorkers.Insert(node.Name)
		}
	}

	return nodesOfInPlaceWorkers
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

// ItShouldFindAllNodesOfInPlaceWorker finds all nodes of in-place workers and returns their names.
func ItShouldFindAllNodesOfInPlaceWorker(s *ShootContext) sets.Set[string] {
	GinkgoHelper()

	nodesOfInPlaceWorkers := sets.New[string]()

	It("should get the nodes of worker with in-place update strategy", func(ctx SpecContext) {
		nodesOfInPlaceWorkers = FindAllNodesOfInPlaceWorker(ctx, s.ShootClient, s.Shoot)
	}, SpecTimeout(2*time.Minute))

	return nodesOfInPlaceWorkers
}

// ItShouldLabelManualInPlaceNodesWithSelectedForUpdate labels all manual in-place nodes with the selected-for-update label.
// In the actual scenario, this should be done by the user, but for testing purposes, we do it here.
func ItShouldLabelManualInPlaceNodesWithSelectedForUpdate(s *ShootContext) {
	GinkgoHelper()

	It("should label all the manual in-place nodes with selected-for-update", func(ctx SpecContext) {
		LabelManualInPlaceNodesWithSelectedForUpdate(ctx, s.ShootClient, s.Shoot)
	}, SpecTimeout(2*time.Minute))
}
