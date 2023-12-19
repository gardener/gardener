// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Gardener contributors
// SPDX-License-Identifier: Apache-2.0

package extensionscheck_test

import (
	"context"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	. "github.com/gardener/gardener/pkg/controllermanager/controller/seed/extensionscheck"
)

var _ = Describe("Add", func() {
	var (
		reconciler             *Reconciler
		controllerInstallation *gardencorev1beta1.ControllerInstallation
	)

	BeforeEach(func() {
		reconciler = &Reconciler{}
		controllerInstallation = &gardencorev1beta1.ControllerInstallation{
			Spec: gardencorev1beta1.ControllerInstallationSpec{
				SeedRef: corev1.ObjectReference{
					Name: "seed",
				},
			},
		}
	})

	Describe("#MapControllerInstallationToSeed", func() {
		var (
			ctx        = context.TODO()
			log        logr.Logger
			fakeClient client.Client
		)

		BeforeEach(func() {
			log = logr.Discard()
			fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).Build()
		})

		It("should do nothing if the object is no ControllerInstallation", func() {
			Expect(reconciler.MapControllerInstallationToSeed(ctx, log, fakeClient, &corev1.Secret{})).To(BeEmpty())
		})

		It("should map the ControllerInstallation to the Seed", func() {
			Expect(reconciler.MapControllerInstallationToSeed(ctx, log, fakeClient, controllerInstallation)).To(ConsistOf(
				reconcile.Request{NamespacedName: types.NamespacedName{Name: controllerInstallation.Spec.SeedRef.Name}},
			))
		})
	})
})
