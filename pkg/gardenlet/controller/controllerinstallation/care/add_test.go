// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package care_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	. "github.com/gardener/gardener/pkg/gardenlet/controller/controllerinstallation/care"
)

var _ = Describe("Add", func() {
	var (
		reconciler                 *Reconciler
		managedResource            *resourcesv1alpha1.ManagedResource
		controllerInstallationName = "foo"
	)

	BeforeEach(func() {
		reconciler = &Reconciler{}
		managedResource = &resourcesv1alpha1.ManagedResource{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "garden",
				Labels:    map[string]string{"controllerinstallation-name": controllerInstallationName},
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
		ctx := context.TODO()

		It("should return nothing because object is no ManagedResource", func() {
			Expect(reconciler.MapManagedResourceToControllerInstallation(ctx, &corev1.Secret{})).To(BeEmpty())
		})

		It("should return nothing because label is not present", func() {
			delete(managedResource.Labels, "controllerinstallation-name")
			Expect(reconciler.MapManagedResourceToControllerInstallation(ctx, managedResource)).To(BeEmpty())
		})

		It("should return nothing because label value is empty", func() {
			managedResource.Labels["controllerinstallation-name"] = ""
			Expect(reconciler.MapManagedResourceToControllerInstallation(ctx, managedResource)).To(BeEmpty())
		})

		It("should return a request with the controller installation name", func() {
			Expect(reconciler.MapManagedResourceToControllerInstallation(ctx, managedResource)).To(ConsistOf(
				reconcile.Request{NamespacedName: types.NamespacedName{Name: controllerInstallationName}},
			))
		})
	})
})
