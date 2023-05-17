// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
