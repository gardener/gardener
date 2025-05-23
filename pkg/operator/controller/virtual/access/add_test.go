// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package access_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	. "github.com/gardener/gardener/pkg/operator/controller/virtual/access"
)

var _ = Describe("Add", func() {
	Describe("#HasRenewAnnotationPredicate", func() {
		var (
			name, namespace string
			predicate       predicate.Predicate

			test = func(object client.Object, result bool) {
				ExpectWithOffset(1, predicate.Create(event.CreateEvent{Object: object})).To(Equal(result))
				ExpectWithOffset(1, predicate.Update(event.UpdateEvent{ObjectNew: object})).To(Equal(result))
				ExpectWithOffset(1, predicate.Delete(event.DeleteEvent{Object: object})).To(Equal(result))
				ExpectWithOffset(1, predicate.Generic(event.GenericEvent{Object: object})).To(Equal(result))
			}
		)

		BeforeEach(func() {
			namespace = "garden"
			name = "access"

			predicate = HasRenewAnnotationPredicate(name, namespace)
		})

		It("should return true when expected object has renew annotation", func() {
			test(&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace, Annotations: map[string]string{"serviceaccount.resources.gardener.cloud/token-renew-timestamp": ""}}}, true)
		})

		It("should return false when expected object doesn't have renew annotation", func() {
			test(&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace}}, false)
		})

		It("should return false when unexpected object", func() {
			test(&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: name + "-foo", Namespace: namespace, Annotations: map[string]string{"serviceaccount.resources.gardener.cloud/token-renew-timestamp": ""}}}, false)
		})
	})
})
