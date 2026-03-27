// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package care_test

import (
	"context"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/gardener/gardener/pkg/api/indexer"
	gardencore "github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	. "github.com/gardener/gardener/pkg/gardenlet/controller/controllerinstallation/care"
)

var _ = Describe("Add", func() {
	var (
		reconciler      *Reconciler
		managedResource *resourcesv1alpha1.ManagedResource
	)

	BeforeEach(func() {
		reconciler = &Reconciler{ManagedResourceNamespace: "garden"}
		managedResource = &resourcesv1alpha1.ManagedResource{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "garden",
				Labels:    map[string]string{"controllerinstallation-name": "foo"},
			},
		}
	})

	Describe("#IsExtensionDeployment", func() {
		var p predicate.Predicate

		BeforeEach(func() {
			p = reconciler.IsExtensionDeployment()
		})

		It("should return false because the namespace is not 'garden'", func() {
			managedResource.Namespace = "foo"

			Expect(p.Create(event.CreateEvent{Object: managedResource})).To(BeFalse())
			Expect(p.Update(event.UpdateEvent{ObjectNew: managedResource})).To(BeFalse())
			Expect(p.Delete(event.DeleteEvent{Object: managedResource})).To(BeFalse())
			Expect(p.Generic(event.GenericEvent{Object: managedResource})).To(BeFalse())
		})

		It("should return true because the label is present", func() {
			Expect(p.Create(event.CreateEvent{Object: managedResource})).To(BeTrue())
			Expect(p.Update(event.UpdateEvent{ObjectNew: managedResource})).To(BeTrue())
			Expect(p.Delete(event.DeleteEvent{Object: managedResource})).To(BeTrue())
			Expect(p.Generic(event.GenericEvent{Object: managedResource})).To(BeTrue())
		})

		It("should return false because the label is present but empty", func() {
			managedResource.Labels["controllerinstallation-name"] = ""

			Expect(p.Create(event.CreateEvent{Object: managedResource})).To(BeFalse())
			Expect(p.Update(event.UpdateEvent{ObjectNew: managedResource})).To(BeFalse())
			Expect(p.Delete(event.DeleteEvent{Object: managedResource})).To(BeFalse())
			Expect(p.Generic(event.GenericEvent{Object: managedResource})).To(BeFalse())
		})

		It("should return false because the label is not present", func() {
			delete(managedResource.Labels, "controllerinstallation-name")

			Expect(p.Create(event.CreateEvent{Object: managedResource})).To(BeFalse())
			Expect(p.Update(event.UpdateEvent{ObjectNew: managedResource})).To(BeFalse())
			Expect(p.Delete(event.DeleteEvent{Object: managedResource})).To(BeFalse())
			Expect(p.Generic(event.GenericEvent{Object: managedResource})).To(BeFalse())
		})
	})

	Describe("#MapManagedResourceToControllerInstallation", func() {
		var (
			ctx   = context.TODO()
			mapFn func(context.Context, client.Object) []reconcile.Request
		)

		BeforeEach(func() {
			mapFn = reconciler.MapManagedResourceToControllerInstallation(logr.Discard())
		})

		It("should return nothing because object is no ManagedResource", func() {
			Expect(mapFn(ctx, &corev1.Secret{})).To(BeEmpty())
		})

		It("should return nothing because label is not present", func() {
			delete(managedResource.Labels, "controllerinstallation-name")
			Expect(mapFn(ctx, managedResource)).To(BeEmpty())
		})

		It("should return nothing because label value is empty", func() {
			managedResource.Labels["controllerinstallation-name"] = ""
			Expect(mapFn(ctx, managedResource)).To(BeEmpty())
		})

		It("should return a request with the controller installation name", func() {
			Expect(mapFn(ctx, managedResource)).To(ConsistOf(
				reconcile.Request{NamespacedName: types.NamespacedName{Name: "foo"}},
			))
		})

		When("controllerinstallation-name label is stale (equals controllerregistration-name)", func() {
			BeforeEach(func() {
				managedResource.Labels["controllerinstallation-name"] = "my-extension"
				managedResource.Labels["controllerregistration-name"] = "my-extension"
			})

			It("should return nothing when no ControllerInstallation matches the registration name", func() {
				reconciler.GardenClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).
					WithIndex(&gardencorev1beta1.ControllerInstallation{}, gardencore.RegistrationRefName, indexer.ControllerInstallationRegistrationRefNameIndexerFunc).
					Build()
				mapFn = reconciler.MapManagedResourceToControllerInstallation(logr.Discard())

				Expect(mapFn(ctx, managedResource)).To(BeEmpty())
			})

			It("should return the correct ControllerInstallation name looked up by registration name", func() {
				reconciler.GardenClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).
					WithIndex(&gardencorev1beta1.ControllerInstallation{}, gardencore.RegistrationRefName, indexer.ControllerInstallationRegistrationRefNameIndexerFunc).
					WithObjects(
						&gardencorev1beta1.ControllerInstallation{
							ObjectMeta: metav1.ObjectMeta{Name: "my-extension-abc12"},
							Spec: gardencorev1beta1.ControllerInstallationSpec{
								RegistrationRef: corev1.ObjectReference{Name: "my-extension"},
								ShootRef:        &corev1.ObjectReference{Name: "shoot1", Namespace: "garden"},
							},
						},
					).Build()
				mapFn = reconciler.MapManagedResourceToControllerInstallation(logr.Discard())

				Expect(mapFn(ctx, managedResource)).To(ConsistOf(
					reconcile.Request{NamespacedName: types.NamespacedName{Name: "my-extension-abc12"}},
				))
			})
		})

		When("controllerinstallation-name differs from controllerregistration-name", func() {
			It("should use the controllerinstallation-name label directly", func() {
				managedResource.Labels["controllerinstallation-name"] = "my-extension-abc12"
				managedResource.Labels["controllerregistration-name"] = "my-extension"

				Expect(mapFn(ctx, managedResource)).To(ConsistOf(
					reconcile.Request{NamespacedName: types.NamespacedName{Name: "my-extension-abc12"}},
				))
			})
		})
	})
})
