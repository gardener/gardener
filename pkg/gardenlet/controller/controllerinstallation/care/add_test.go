// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package care_test

import (
	"context"

	"github.com/go-logr/logr"
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
		var (
			ctx = context.TODO()
			log = logr.Discard()
		)

		It("should return nothing because object is no ManagedResource", func() {
			Expect(reconciler.MapManagedResourceToControllerInstallation(ctx, log, nil, &corev1.Secret{})).To(BeEmpty())
		})

		It("should return nothing because label is not present", func() {
			delete(managedResource.Labels, "controllerinstallation-name")
			Expect(reconciler.MapManagedResourceToControllerInstallation(ctx, log, nil, managedResource)).To(BeEmpty())
		})

		It("should return nothing because label value is empty", func() {
			managedResource.Labels["controllerinstallation-name"] = ""
			Expect(reconciler.MapManagedResourceToControllerInstallation(ctx, log, nil, managedResource)).To(BeEmpty())
		})

		It("should return a request with the controller installation name", func() {
			Expect(reconciler.MapManagedResourceToControllerInstallation(ctx, log, nil, managedResource)).To(ConsistOf(
				reconcile.Request{NamespacedName: types.NamespacedName{Name: controllerInstallationName}},
			))
		})
	})
})
