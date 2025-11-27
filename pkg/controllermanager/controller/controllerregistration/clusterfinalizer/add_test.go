// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package clusterfinalizer_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/gardener/gardener/pkg/controllermanager/controller/controllerregistration/clusterfinalizer"
)

var _ = Describe("Add", func() {
	var (
		ctx context.Context

		controllerInstallation *gardencorev1beta1.ControllerInstallation
	)

	BeforeEach(func() {
		ctx = context.Background()

		controllerInstallation = &gardencorev1beta1.ControllerInstallation{}
	})

	Describe("#MapControllerInstallationToSeed", func() {
		var seedName string

		BeforeEach(func() {
			seedName = "seed-1"
			controllerInstallation = &gardencorev1beta1.ControllerInstallation{
				Spec: gardencorev1beta1.ControllerInstallationSpec{
					SeedRef: &corev1.ObjectReference{
						Name: seedName,
					},
				},
			}
		})

		It("should return a request with the seed name", func() {
			Expect(MapControllerInstallationToSeed(ctx, controllerInstallation)).To(ConsistOf(reconcile.Request{NamespacedName: types.NamespacedName{Name: seedName}}))
		})

		It("should return nil when object is not a ControllerInstallation", func() {
			Expect(MapControllerInstallationToSeed(ctx, nil)).To(BeNil())
		})
	})

	Describe("#MapControllerInstallationToShoot", func() {
		var shootName, shootNamespace string

		BeforeEach(func() {
			shootName, shootNamespace = "shoot-1-name", "shoot-1-namespace"
			controllerInstallation = &gardencorev1beta1.ControllerInstallation{
				Spec: gardencorev1beta1.ControllerInstallationSpec{
					ShootRef: &corev1.ObjectReference{
						Name:      shootName,
						Namespace: shootNamespace,
					},
				},
			}
		})

		It("should return a request with the shoot name", func() {
			Expect(MapControllerInstallationToShoot(ctx, controllerInstallation)).To(ConsistOf(reconcile.Request{NamespacedName: types.NamespacedName{Name: shootName, Namespace: shootNamespace}}))
		})

		It("should return nil when object is not a ControllerInstallation", func() {
			Expect(MapControllerInstallationToShoot(ctx, nil)).To(BeNil())
		})
	})
})
