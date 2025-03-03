// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package finalizer_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/controllermanager/controller/shoot/state/finalizer"
)

var _ = Describe("AddToManager", func() {
	Describe("#MapShootToShootState", func() {
		var (
			ctx        context.Context
			reconciler *finalizer.Reconciler
		)

		BeforeEach(func() {
			ctx = context.Background()
			reconciler = &finalizer.Reconciler{}
		})

		It("should return reconciliation request matching the shoot name", func() {
			shootName := "shoot-1"
			shootNamespace := "ns-1"
			shoot := &gardencorev1beta1.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      shootName,
					Namespace: shootNamespace,
				},
			}

			reconciliationRequest := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      shootName,
					Namespace: shootNamespace,
				},
			}

			requests := reconciler.MapShootToShootState(ctx, shoot)
			Expect(requests).To(ConsistOf(reconciliationRequest))
		})

		It("should return nil if argument is not a gardencorev1beta1.Shoot", func() {
			deploymentName := "dep-1"
			deploymentNamespace := "ns-1"
			deployment := appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      deploymentName,
					Namespace: deploymentNamespace,
				},
			}

			requests := reconciler.MapShootToShootState(ctx, &deployment)
			Expect(requests).To(BeNil())
		})
	})
})
