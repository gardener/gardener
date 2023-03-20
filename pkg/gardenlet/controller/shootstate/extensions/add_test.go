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

package extensions_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	. "github.com/gardener/gardener/pkg/gardenlet/controller/shootstate/extensions"
)

var _ = Describe("Add", func() {
	var (
		reconciler     *Reconciler
		infrastructure *extensionsv1alpha1.Infrastructure
	)

	BeforeEach(func() {
		reconciler = &Reconciler{}
		infrastructure = &extensionsv1alpha1.Infrastructure{
			Spec: extensionsv1alpha1.InfrastructureSpec{
				DefaultSpec: extensionsv1alpha1.DefaultSpec{
					Type: "type",
				},
			},
		}
	})

	Describe("#ObjectPredicate", func() {
		var p predicate.Predicate

		BeforeEach(func() {
			p = reconciler.ObjectPredicate()
		})

		Describe("#Create", func() {
			It("should return true", func() {
				Expect(p.Create(event.CreateEvent{})).To(BeTrue())
			})
		})

		Describe("#Update", func() {
			It("should return true for periodic cache resyncs", func() {
				Expect(p.Update(event.UpdateEvent{ObjectNew: infrastructure, ObjectOld: infrastructure.DeepCopy()})).To(BeTrue())
			})

			It("should return false because object is no extensions object", func() {
				Expect(p.Update(event.UpdateEvent{ObjectOld: &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{ResourceVersion: "123"}}, ObjectNew: infrastructure})).To(BeFalse())
			})

			It("should return false because old object is no extensions object", func() {
				Expect(p.Update(event.UpdateEvent{ObjectOld: infrastructure, ObjectNew: &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{ResourceVersion: "123"}}})).To(BeFalse())
			})

			It("should return false because neither state nor resources nor operation annotation changed", func() {
				oldInfrastructure := infrastructure.DeepCopy()
				infrastructure.ResourceVersion = "1"
				Expect(p.Update(event.UpdateEvent{ObjectNew: infrastructure, ObjectOld: oldInfrastructure})).To(BeFalse())
			})

			It("should return true because state was changed", func() {
				oldInfrastructure := infrastructure.DeepCopy()
				infrastructure.ResourceVersion = "1"
				infrastructure.Status.State = &runtime.RawExtension{}
				Expect(p.Update(event.UpdateEvent{ObjectNew: infrastructure, ObjectOld: oldInfrastructure})).To(BeTrue())
			})

			It("should return true because resources were changed", func() {
				oldInfrastructure := infrastructure.DeepCopy()
				infrastructure.ResourceVersion = "1"
				infrastructure.Status.Resources = []gardencorev1beta1.NamedResourceReference{{}}
				Expect(p.Update(event.UpdateEvent{ObjectNew: infrastructure, ObjectOld: oldInfrastructure})).To(BeTrue())
			})

			It("should return true because operation annotation was changed to valid value", func() {
				oldInfrastructure := infrastructure.DeepCopy()
				infrastructure.ResourceVersion = "1"
				metav1.SetMetaDataAnnotation(&oldInfrastructure.ObjectMeta, "gardener.cloud/operation", "wait-for-state")
				metav1.SetMetaDataAnnotation(&infrastructure.ObjectMeta, "gardener.cloud/operation", "foo")
				Expect(p.Update(event.UpdateEvent{ObjectNew: infrastructure, ObjectOld: oldInfrastructure})).To(BeTrue())
			})

			It("should return false because operation annotation was changed to invalid value", func() {
				oldInfrastructure := infrastructure.DeepCopy()
				infrastructure.ResourceVersion = "1"
				metav1.SetMetaDataAnnotation(&infrastructure.ObjectMeta, "gardener.cloud/operation", "restore")
				Expect(p.Update(event.UpdateEvent{ObjectNew: infrastructure, ObjectOld: oldInfrastructure})).To(BeFalse())
			})
		})

		Describe("#Delete", func() {
			It("should return true", func() {
				Expect(p.Delete(event.DeleteEvent{})).To(BeTrue())
			})
		})

		Describe("#Generic", func() {
			It("should return false", func() {
				Expect(p.Generic(event.GenericEvent{})).To(BeFalse())
			})
		})
	})

	Describe("#InvalidOperationAnnotationPredicate", func() {
		var p predicate.Predicate

		BeforeEach(func() {
			p = reconciler.InvalidOperationAnnotationPredicate()
		})

		tests := func(f func(*extensionsv1alpha1.Infrastructure) bool) {
			It("should return true when no operation annotation present", func() {
				Expect(f(infrastructure)).To(BeTrue())
			})

			It("should return false when operation annotation is 'wait-for-state'", func() {
				metav1.SetMetaDataAnnotation(&infrastructure.ObjectMeta, "gardener.cloud/operation", "wait-for-state")
				Expect(f(infrastructure)).To(BeFalse())
			})

			It("should return false when operation annotation is 'migrate'", func() {
				metav1.SetMetaDataAnnotation(&infrastructure.ObjectMeta, "gardener.cloud/operation", "migrate")
				Expect(f(infrastructure)).To(BeFalse())
			})

			It("should return false when operation annotation is 'restore'", func() {
				metav1.SetMetaDataAnnotation(&infrastructure.ObjectMeta, "gardener.cloud/operation", "restore")
				Expect(f(infrastructure)).To(BeFalse())
			})

			It("should return true when operation annotation has different value", func() {
				metav1.SetMetaDataAnnotation(&infrastructure.ObjectMeta, "gardener.cloud/operation", "foo")
				Expect(f(infrastructure)).To(BeTrue())
			})
		}

		Describe("#Create", func() {
			tests(func(obj *extensionsv1alpha1.Infrastructure) bool { return p.Create(event.CreateEvent{Object: obj}) })
		})

		Describe("#Update", func() {
			tests(func(obj *extensionsv1alpha1.Infrastructure) bool { return p.Update(event.UpdateEvent{ObjectNew: obj}) })
		})

		Describe("#Delete", func() {
			tests(func(obj *extensionsv1alpha1.Infrastructure) bool { return p.Delete(event.DeleteEvent{Object: obj}) })
		})

		Describe("#Generic", func() {
			tests(func(obj *extensionsv1alpha1.Infrastructure) bool { return p.Generic(event.GenericEvent{Object: obj}) })
		})
	})
})
