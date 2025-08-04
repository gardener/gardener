// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package inplace

import (
	"context"
	"strings"
	"time"

	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	. "github.com/gardener/gardener/test/e2e/gardener"
)

// LabelManualInPlaceNodesWithSelectedForUpdate labels all manual in-place nodes with the selected-for-update label.
// In the actual scenario, this should be done by the user, but for testing purposes, we do it here.
func LabelManualInPlaceNodesWithSelectedForUpdate(ctx context.Context, log logr.Logger, shootClient client.Client, shoot *gardencorev1beta1.Shoot) {
	GinkgoHelper()

	log = log.WithValues("shoot", client.ObjectKeyFromObject(shoot))

	for _, pool := range shoot.Spec.Provider.Workers {
		if !v1beta1helper.IsUpdateStrategyManualInPlace(pool.UpdateStrategy) {
			continue
		}

		nodeList := &corev1.NodeList{}
		Eventually(ctx, func() error {
			return shootClient.List(ctx, nodeList, client.MatchingLabels{v1beta1constants.LabelWorkerPool: pool.Name})
		}).Should(Succeed())

		nodes := make([]string, 0, len(nodeList.Items))
		for _, node := range nodeList.Items {
			if metav1.HasLabel(node.ObjectMeta, machinev1alpha1.LabelKeyNodeSelectedForUpdate) {
				continue
			}

			nodes = append(nodes, node.Name)
			patch := client.MergeFrom(node.DeepCopy())
			metav1.SetMetaDataLabel(&node.ObjectMeta, machinev1alpha1.LabelKeyNodeSelectedForUpdate, "true")

			Eventually(ctx, func(g Gomega) {
				g.Expect(shootClient.Patch(ctx, &node, patch)).To(Succeed())
			}).Should(Succeed(), "node %s should be labeled", node.Name)
		}

		log.Info("All manual in-place nodes have been labeled with selected-for-update", "workerPool", pool.Name, "nodes", strings.Join(nodes, ", "))
	}
}

// FindNodesOfInPlaceWorkers finds all nodes of in-place workers and returns their names.
func FindNodesOfInPlaceWorkers(ctx context.Context, log logr.Logger, shootClient client.Client, shoot *gardencorev1beta1.Shoot) sets.Set[string] {
	GinkgoHelper()

	log = log.WithValues("shoot", client.ObjectKeyFromObject(shoot))
	nodesOfInPlaceWorkers := sets.New[string]()

	for _, pool := range shoot.Spec.Provider.Workers {
		if !v1beta1helper.IsUpdateStrategyInPlace(pool.UpdateStrategy) {
			continue
		}

		nodeList := &corev1.NodeList{}
		Eventually(ctx, func() error {
			return shootClient.List(ctx, nodeList, client.MatchingLabels{v1beta1constants.LabelWorkerPool: pool.Name})
		}).Should(Succeed())

		nodes := make([]string, 0, len(nodeList.Items))
		for _, node := range nodeList.Items {
			nodes = append(nodes, node.Name)
			nodesOfInPlaceWorkers.Insert(node.Name)
		}
		log.Info("Found nodes for worker pool with in-place update strategy", "workerPool", pool.Name, "nodes", strings.Join(nodes, ", "))
	}

	return nodesOfInPlaceWorkers
}

// ItShouldLabelManualInPlaceNodesWithSelectedForUpdate labels all manual in-place nodes with the selected-for-update label.
// In the actual scenario, this should be done by the user, but for testing purposes, we do it here.
func ItShouldLabelManualInPlaceNodesWithSelectedForUpdate(s *ShootContext) {
	GinkgoHelper()

	It("should label all the manual in-place nodes with selected-for-update", func(ctx SpecContext) {
		LabelManualInPlaceNodesWithSelectedForUpdate(ctx, s.Log, s.ShootClient, s.Shoot)
	}, SpecTimeout(2*time.Minute))
}
