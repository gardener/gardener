// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package networkpolicy_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	. "github.com/gardener/gardener/pkg/provider-local/controller/networkpolicy"
)

var _ = Describe("Add", func() {
	var (
		namespace *corev1.Namespace
		p         predicate.Predicate
	)

	Describe("#IsShootNamespacePredicate", func() {
		BeforeEach(func() {
			p = IsShootNamespace()
			namespace = &corev1.Namespace{}
		})

		It("should return true because the namespace is a shoot namespace", func() {
			namespace.Name = "shoot-garden-local"

			Expect(p.Create(event.CreateEvent{Object: namespace})).To(BeTrue())
			Expect(p.Update(event.UpdateEvent{ObjectNew: namespace})).To(BeTrue())
			Expect(p.Delete(event.DeleteEvent{Object: namespace})).To(BeTrue())
			Expect(p.Generic(event.GenericEvent{Object: namespace})).To(BeTrue())
		})

		It("should return true because the namespace is a shoot namespace", func() {
			namespace.Name = "shoot--local-local"

			Expect(p.Create(event.CreateEvent{Object: namespace})).To(BeTrue())
			Expect(p.Update(event.UpdateEvent{ObjectNew: namespace})).To(BeTrue())
			Expect(p.Delete(event.DeleteEvent{Object: namespace})).To(BeTrue())
			Expect(p.Generic(event.GenericEvent{Object: namespace})).To(BeTrue())
		})

		It("should return true because the namespace is not a shoot namespace", func() {
			namespace.Name = "foo"

			Expect(p.Create(event.CreateEvent{Object: namespace})).To(BeFalse())
			Expect(p.Update(event.UpdateEvent{ObjectNew: namespace})).To(BeFalse())
			Expect(p.Delete(event.DeleteEvent{Object: namespace})).To(BeFalse())
			Expect(p.Generic(event.GenericEvent{Object: namespace})).To(BeFalse())
		})
	})

	Describe("#IsShootProviderLocal", func() {
		BeforeEach(func() {
			p = IsShootProviderLocal()
			namespace = &corev1.Namespace{}
		})

		It("should return true because shoot has provider type local", func() {
			namespace.Labels = map[string]string{
				"shoot.gardener.cloud/provider": "local",
			}

			Expect(p.Create(event.CreateEvent{Object: namespace})).To(BeTrue())
			Expect(p.Update(event.UpdateEvent{ObjectNew: namespace})).To(BeTrue())
			Expect(p.Delete(event.DeleteEvent{Object: namespace})).To(BeTrue())
			Expect(p.Generic(event.GenericEvent{Object: namespace})).To(BeTrue())
		})

		It("should return false because shoot doesn't have provider type local", func() {
			Expect(p.Create(event.CreateEvent{Object: namespace})).To(BeFalse())
			Expect(p.Update(event.UpdateEvent{ObjectNew: namespace})).To(BeFalse())
			Expect(p.Delete(event.DeleteEvent{Object: namespace})).To(BeFalse())
			Expect(p.Generic(event.GenericEvent{Object: namespace})).To(BeFalse())
		})
	})
})
