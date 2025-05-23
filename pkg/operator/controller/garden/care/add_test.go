// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package care_test

import (
	"context"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	operatorclient "github.com/gardener/gardener/pkg/operator/client"
	. "github.com/gardener/gardener/pkg/operator/controller/garden/care"
)

var _ = Describe("Add", func() {
	var (
		ctx           context.Context
		runtimeClient client.Client

		reconciler *Reconciler
		garden     *operatorv1alpha1.Garden
	)

	BeforeEach(func() {
		ctx = context.Background()

		runtimeClient = fakeclient.NewClientBuilder().WithScheme(operatorclient.RuntimeScheme).Build()

		garden = &operatorv1alpha1.Garden{
			ObjectMeta: metav1.ObjectMeta{
				Name: gardenName,
			},
		}

		reconciler = &Reconciler{
			RuntimeClient: runtimeClient,
		}
	})

	Describe("#MapManagedResourceToGarden", func() {
		JustBeforeEach(func() {
			Expect(runtimeClient.Create(ctx, garden)).To(Succeed())
		})

		Context("when Garden reconciliation is not processing", func() {
			It("should return a request with the garden name", func() {
				Expect(reconciler.MapManagedResourceToGarden(logr.Discard())(ctx, nil)).To(ConsistOf(reconcile.Request{NamespacedName: types.NamespacedName{Name: gardenName}}))
			})
		})

		Context("when Garden reconciliation is processing", func() {
			BeforeEach(func() {
				garden.Status.LastOperation = &gardencorev1beta1.LastOperation{State: gardencorev1beta1.LastOperationStateProcessing}
			})

			It("should return an empty list", func() {
				Expect(reconciler.MapManagedResourceToGarden(logr.Discard())(ctx, nil)).To(BeEmpty())
			})
		})
	})
})
