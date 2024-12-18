// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package tokeninvalidator_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	. "github.com/gardener/gardener/pkg/resourcemanager/controller/tokeninvalidator"
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
					Annotations: map[string]string{"kubernetes.io/service-account.name": "foo"},
				},
			}
		})

		Describe("#Create", func() {
			It("should return false when object has no service account name annotation", func() {
				delete(secret.Annotations, "kubernetes.io/service-account.name")
				Expect(p.Create(event.CreateEvent{Object: secret})).To(BeFalse())
			})

			It("should return true when object has service account name annotation", func() {
				Expect(p.Create(event.CreateEvent{Object: secret})).To(BeTrue())
			})
		})

		Describe("#Update", func() {
			It("should return false when object has no service account name annotation", func() {
				delete(secret.Annotations, "kubernetes.io/service-account.name")
				Expect(p.Update(event.UpdateEvent{ObjectNew: secret})).To(BeFalse())
			})

			It("should return true when object has service account name annotation", func() {
				Expect(p.Update(event.UpdateEvent{ObjectNew: secret})).To(BeTrue())
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

	Describe("#ServiceAccountPredicate", func() {
		var (
			p              predicate.Predicate
			serviceAccount *corev1.ServiceAccount
		)

		BeforeEach(func() {
			p = (&Reconciler{}).ServiceAccountPredicate()
			serviceAccount = &corev1.ServiceAccount{}
		})

		Describe("#Create", func() {
			It("should return false", func() {
				Expect(p.Create(event.CreateEvent{})).To(BeFalse())
			})
		})

		Describe("#Update", func() {
			It("should return false when old object is not ServiceAccount", func() {
				Expect(p.Update(event.UpdateEvent{})).To(BeFalse())
			})

			It("should return false when new object is not ServiceAccount", func() {
				Expect(p.Update(event.UpdateEvent{ObjectOld: serviceAccount})).To(BeFalse())
			})

			It("should return false when neither auto-token-mount setting nor skip label changed", func() {
				oldServiceAccount := serviceAccount.DeepCopy()
				Expect(p.Update(event.UpdateEvent{ObjectOld: oldServiceAccount, ObjectNew: serviceAccount})).To(BeFalse())
			})

			It("should return true when auto-token-mount setting changed", func() {
				oldServiceAccount := serviceAccount.DeepCopy()
				serviceAccount.AutomountServiceAccountToken = ptr.To(true)
				Expect(p.Update(event.UpdateEvent{ObjectOld: oldServiceAccount, ObjectNew: serviceAccount})).To(BeTrue())
			})

			It("should return true when skip label changed", func() {
				oldServiceAccount := serviceAccount.DeepCopy()
				serviceAccount.Labels = map[string]string{"token-invalidator.resources.gardener.cloud/skip": "true"}
				Expect(p.Update(event.UpdateEvent{ObjectOld: oldServiceAccount, ObjectNew: serviceAccount})).To(BeTrue())
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

	Describe("#MapServiceAccountToSecrets", func() {
		ctx := context.TODO()

		It("should return nil because object is not ServiceAccount", func() {
			Expect((&Reconciler{}).MapServiceAccountToSecrets(ctx, &corev1.Secret{})).To(BeNil())
		})

		It("should return map ServiceAccount to all referenced secrets", func() {
			serviceAccount := &corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{Namespace: "namespace"},
				Secrets: []corev1.ObjectReference{
					{Name: "secret1"},
					{Name: "secret2"},
				},
			}

			Expect((&Reconciler{}).MapServiceAccountToSecrets(ctx, serviceAccount)).To(ConsistOf(
				reconcile.Request{NamespacedName: types.NamespacedName{Name: "secret1", Namespace: "namespace"}},
				reconcile.Request{NamespacedName: types.NamespacedName{Name: "secret2", Namespace: "namespace"}},
			))
		})
	})
})
