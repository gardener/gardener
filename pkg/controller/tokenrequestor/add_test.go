// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package tokenrequestor_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	. "github.com/gardener/gardener/pkg/controller/tokenrequestor"
)

var _ = Describe("Add", func() {
	Describe("#SecretPredicate", func() {
		var (
			p      predicate.Predicate
			secret *corev1.Secret
		)

		BeforeEach(func() {
			p = (&Reconciler{}).SecretPredicate()
			secret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"resources.gardener.cloud/purpose": "token-requestor"},
				},
			}
		})

		Describe("#Create", func() {
			It("should return false when object is not Secret", func() {
				Expect(p.Create(event.CreateEvent{Object: &corev1.ConfigMap{}})).To(BeFalse())
			})

			It("should return false when secret is not labeled as expected", func() {
				secret.Labels["resources.gardener.cloud/purpose"] = "foo"
				Expect(p.Create(event.CreateEvent{Object: secret})).To(BeFalse())
			})

			It("should return true when secret is labeled as expected", func() {
				Expect(p.Create(event.CreateEvent{Object: secret})).To(BeTrue())
			})

			It("should return true when secret is labeled with class but reconciler is responsible for all classes", func() {
				metav1.SetMetaDataLabel(&secret.ObjectMeta, "resources.gardener.cloud/class", "foo")
				Expect(p.Create(event.CreateEvent{Object: secret})).To(BeTrue())
			})

			It("should return true when secret is labeled with class that reconciler is responsible for", func() {
				p = (&Reconciler{Class: ptr.To("foo")}).SecretPredicate()
				metav1.SetMetaDataLabel(&secret.ObjectMeta, "resources.gardener.cloud/class", "foo")
				Expect(p.Create(event.CreateEvent{Object: secret})).To(BeTrue())
			})

			It("should return false when secret is labeled with class but reconciler is responsible for another one", func() {
				p = (&Reconciler{Class: ptr.To("foo")}).SecretPredicate()
				metav1.SetMetaDataLabel(&secret.ObjectMeta, "resources.gardener.cloud/class", "bar")
				Expect(p.Create(event.CreateEvent{Object: secret})).To(BeFalse())
			})
		})

		Describe("#Update", func() {
			It("should return false when object is not Secret", func() {
				Expect(p.Update(event.UpdateEvent{ObjectNew: &corev1.ConfigMap{}})).To(BeFalse())
			})

			It("should return false when secret is not labeled as expected", func() {
				secret.Labels["resources.gardener.cloud/purpose"] = "foo"
				Expect(p.Update(event.UpdateEvent{ObjectNew: secret})).To(BeFalse())
			})

			It("should return true when secret is no longer relevant because class changed", func() {
				p = (&Reconciler{Class: ptr.To("foo")}).SecretPredicate()
				oldSecret := secret.DeepCopy()
				metav1.SetMetaDataLabel(&oldSecret.ObjectMeta, "resources.gardener.cloud/class", "foo")
				metav1.SetMetaDataLabel(&secret.ObjectMeta, "resources.gardener.cloud/class", "bar")
				Expect(p.Update(event.UpdateEvent{ObjectNew: secret, ObjectOld: oldSecret})).To(BeTrue())
			})

			It("should return true when secret is labeled as expected", func() {
				Expect(p.Update(event.UpdateEvent{ObjectNew: secret})).To(BeTrue())
			})

			It("should return true when secret is labeled with class but reconciler is responsible for all classes", func() {
				metav1.SetMetaDataLabel(&secret.ObjectMeta, "resources.gardener.cloud/class", "foo")
				Expect(p.Update(event.UpdateEvent{ObjectNew: secret})).To(BeTrue())
			})

			It("should return true when secret was not relevant but purpose changed", func() {
				oldSecret := secret.DeepCopy()
				secret.Labels["resources.gardener.cloud/purpose"] = "foo"
				Expect(p.Update(event.UpdateEvent{ObjectNew: secret, ObjectOld: oldSecret})).To(BeTrue())
			})

			It("should return true when secret was not relevant but class changed", func() {
				p = (&Reconciler{Class: ptr.To("foo")}).SecretPredicate()
				oldSecret := secret.DeepCopy()
				metav1.SetMetaDataLabel(&oldSecret.ObjectMeta, "resources.gardener.cloud/class", "bar")
				metav1.SetMetaDataLabel(&secret.ObjectMeta, "resources.gardener.cloud/class", "foo")
				Expect(p.Update(event.UpdateEvent{ObjectNew: secret, ObjectOld: oldSecret})).To(BeTrue())
			})
		})

		Describe("#Delete", func() {
			It("should return false when object is not Secret", func() {
				Expect(p.Delete(event.DeleteEvent{Object: &corev1.ConfigMap{}})).To(BeFalse())
			})

			It("should return false when secret is not labeled as expected", func() {
				secret.Labels["resources.gardener.cloud/purpose"] = "foo"
				Expect(p.Delete(event.DeleteEvent{Object: secret})).To(BeFalse())
			})

			It("should return true when secret is labeled as expected", func() {
				Expect(p.Delete(event.DeleteEvent{Object: secret})).To(BeTrue())
			})

			It("should return true when secret is labeled with class but reconciler is responsible for all classes", func() {
				metav1.SetMetaDataLabel(&secret.ObjectMeta, "resources.gardener.cloud/class", "foo")
				Expect(p.Delete(event.DeleteEvent{Object: secret})).To(BeTrue())
			})

			It("should return true when secret is labeled with class that reconciler is responsible for", func() {
				p = (&Reconciler{Class: ptr.To("foo")}).SecretPredicate()
				metav1.SetMetaDataLabel(&secret.ObjectMeta, "resources.gardener.cloud/class", "foo")
				Expect(p.Delete(event.DeleteEvent{Object: secret})).To(BeTrue())
			})

			It("should return false when secret is labeled with class but reconciler is responsible for another one", func() {
				p = (&Reconciler{Class: ptr.To("foo")}).SecretPredicate()
				metav1.SetMetaDataLabel(&secret.ObjectMeta, "resources.gardener.cloud/class", "bar")
				Expect(p.Create(event.CreateEvent{Object: secret})).To(BeFalse())
			})
		})

		Describe("#Generic", func() {
			It("should return false", func() {
				Expect(p.Generic(event.GenericEvent{})).To(BeFalse())
			})
		})
	})
})
