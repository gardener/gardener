// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package rootcapublisher_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	. "github.com/gardener/gardener/pkg/resourcemanager/controller/rootcapublisher"
)

var _ = Describe("Add", func() {
	Describe("#NamespacePredicate", func() {
		var p predicate.Predicate

		BeforeEach(func() {
			p = (&Reconciler{}).NamespacePredicate()
		})

		Describe("#Create", func() {
			It("should return true", func() {
				Expect(p.Create(event.CreateEvent{})).To(BeTrue())
			})
		})

		Describe("#Update", func() {
			It("should return false when object is not Namespace", func() {
				Expect(p.Update(event.UpdateEvent{ObjectNew: &corev1.Secret{}})).To(BeFalse())
			})

			It("should return false when namespace is not active", func() {
				Expect(p.Update(event.UpdateEvent{ObjectNew: &corev1.Namespace{Status: corev1.NamespaceStatus{Phase: corev1.NamespaceTerminating}}})).To(BeFalse())
			})

			It("should return true when namespace is active", func() {
				Expect(p.Update(event.UpdateEvent{ObjectNew: &corev1.Namespace{Status: corev1.NamespaceStatus{Phase: corev1.NamespaceActive}}})).To(BeTrue())
			})
		})

		Describe("#Delete", func() {
			It("should return false", func() {
				Expect(p.Delete(event.DeleteEvent{})).To(BeFalse())
			})
		})

		Describe("#Generic", func() {
			It("should return false", func() {
				Expect(p.Generic(event.GenericEvent{})).To(BeFalse())
			})
		})
	})

	Describe("#ConfigMapPredicate", func() {
		var (
			p         predicate.Predicate
			configMap *corev1.ConfigMap
		)

		BeforeEach(func() {
			p = (&Reconciler{}).ConfigMapPredicate()
			configMap = &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "kube-root-ca.crt",
					Annotations: map[string]string{"kubernetes.io/description": ""},
				},
			}
		})

		Describe("#Create", func() {
			It("should return false", func() {
				Expect(p.Create(event.CreateEvent{})).To(BeFalse())
			})
		})

		Describe("#Update", func() {
			It("should return false when object does not have expected name", func() {
				configMap.Name = "foo"
				Expect(p.Update(event.UpdateEvent{ObjectNew: configMap})).To(BeFalse())
			})

			It("should return false when object does not have expected annotation", func() {
				configMap.Annotations["kubernetes.io/description"] = "foo"
				Expect(p.Update(event.UpdateEvent{ObjectNew: configMap})).To(BeFalse())
			})

			It("should return true when object has expected name and annotation", func() {
				Expect(p.Update(event.UpdateEvent{ObjectNew: configMap})).To(BeTrue())
			})
		})

		Describe("#Delete", func() {
			It("should return false when object does not have expected name", func() {
				configMap.Name = "foo"
				Expect(p.Delete(event.DeleteEvent{Object: configMap})).To(BeFalse())
			})

			It("should return false when object does not have expected annotation", func() {
				configMap.Annotations["kubernetes.io/description"] = "foo"
				Expect(p.Delete(event.DeleteEvent{Object: configMap})).To(BeFalse())
			})

			It("should return true when object has expected name and annotation", func() {
				Expect(p.Delete(event.DeleteEvent{Object: configMap})).To(BeTrue())
			})
		})

		Describe("#Generic", func() {
			It("should return false", func() {
				Expect(p.Generic(event.GenericEvent{})).To(BeFalse())
			})
		})
	})
})
