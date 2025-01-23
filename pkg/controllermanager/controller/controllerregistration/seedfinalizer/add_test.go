// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package seedfinalizer_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/gardener/gardener/pkg/controllermanager/controller/controllerregistration/seedfinalizer"
)

var _ = Describe("Add", func() {
	Describe("#MapControllerInstallationToSeed", func() {
		var (
			ctx context.Context
			r   *Reconciler

			seedName               string
			controllerInstallation *gardencorev1beta1.ControllerInstallation
		)

		BeforeEach(func() {
			ctx = context.Background()
			r = &Reconciler{}

			seedName = "seed-1"
			controllerInstallation = &gardencorev1beta1.ControllerInstallation{
				Spec: gardencorev1beta1.ControllerInstallationSpec{
					SeedRef: corev1.ObjectReference{
						Name: seedName,
					},
				},
			}
		})

		It("should return a request with the seed name", func() {
			Expect(r.MapControllerInstallationToSeed(ctx, controllerInstallation)).To(ConsistOf(reconcile.Request{NamespacedName: types.NamespacedName{Name: seedName}}))
		})

		It("should return nil when object is not a ControllerInstallation", func() {
			Expect(r.MapControllerInstallationToSeed(ctx, nil)).To(BeNil())
		})
	})
})
