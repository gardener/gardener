//  SPDX-FileCopyrightText: Contributors to the Gardener project
//
//  SPDX-License-Identifier: Apache-2.0

package predicate_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	. "github.com/gardener/gardener/pkg/operator/predicate"
)

var _ = Describe("Secret", func() {
	var (
		p      predicate.Predicate
		secret *corev1.Secret
	)

	BeforeEach(func() {
		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "garden-ingress-certificate",
				Namespace: "garden",
				Labels:    map[string]string{"gardener.cloud/role": "garden-cert"},
			},
			Data: map[string][]byte{
				"tls.crt": []byte("cert"),
				"tls.key": []byte("key"),
			},
		}
	})

	Describe("#GardenWildcardCertificateChanged", func() {
		BeforeEach(func() {
			p = GardenWildcardCertificateChanged()
		})

		Describe("#Create", func() {
			It("should return true because secret has the garden wildcard certificate role", func() {
				Expect(p.Create(event.CreateEvent{Object: secret})).To(BeTrue())
			})

			It("should return false because secret has no role label", func() {
				secret.Labels = nil
				Expect(p.Create(event.CreateEvent{Object: secret})).To(BeFalse())
			})

			It("should return false because secret has a different role", func() {
				secret.Labels["gardener.cloud/role"] = "controlplane-cert"
				Expect(p.Create(event.CreateEvent{Object: secret})).To(BeFalse())
			})
		})

		Describe("#Update", func() {
			var oldSecret *corev1.Secret

			BeforeEach(func() {
				oldSecret = secret.DeepCopy()
			})

			It("should return false because new object is no secret", func() {
				Expect(p.Update(event.UpdateEvent{})).To(BeFalse())
			})

			It("should return false because old object is no secret", func() {
				Expect(p.Update(event.UpdateEvent{ObjectNew: secret})).To(BeFalse())
			})

			It("should return false because nothing changed", func() {
				Expect(p.Update(event.UpdateEvent{ObjectOld: oldSecret, ObjectNew: secret})).To(BeFalse())
			})

			It("should return false because only annotations changed", func() {
				metav1.SetMetaDataAnnotation(&secret.ObjectMeta, "foo", "bar")
				Expect(p.Update(event.UpdateEvent{ObjectOld: oldSecret, ObjectNew: secret})).To(BeFalse())
			})

			It("should return true because data changed", func() {
				secret.Data["tls.crt"] = []byte("renewed-cert")
				Expect(p.Update(event.UpdateEvent{ObjectOld: oldSecret, ObjectNew: secret})).To(BeTrue())
			})

			It("should return true because role label was added", func() {
				oldSecret.Labels = nil
				Expect(p.Update(event.UpdateEvent{ObjectOld: oldSecret, ObjectNew: secret})).To(BeTrue())
			})

			It("should return true because role label was removed", func() {
				secret.Labels = nil
				Expect(p.Update(event.UpdateEvent{ObjectOld: oldSecret, ObjectNew: secret})).To(BeTrue())
			})

			It("should return true because role changed to the garden wildcard certificate role", func() {
				oldSecret.Labels["gardener.cloud/role"] = "controlplane-cert"
				Expect(p.Update(event.UpdateEvent{ObjectOld: oldSecret, ObjectNew: secret})).To(BeTrue())
			})

			It("should return false because data of a secret without role label changed", func() {
				secret.Labels = nil
				oldSecret.Labels = nil
				secret.Data["tls.crt"] = []byte("renewed-cert")
				Expect(p.Update(event.UpdateEvent{ObjectOld: oldSecret, ObjectNew: secret})).To(BeFalse())
			})

			It("should return false because data of a secret with a different role changed", func() {
				secret.Labels["gardener.cloud/role"] = "controlplane-cert"
				oldSecret.Labels["gardener.cloud/role"] = "controlplane-cert"
				secret.Data["tls.crt"] = []byte("renewed-cert")
				Expect(p.Update(event.UpdateEvent{ObjectOld: oldSecret, ObjectNew: secret})).To(BeFalse())
			})
		})

		Describe("#Delete", func() {
			It("should return true because secret has the garden wildcard certificate role", func() {
				Expect(p.Delete(event.DeleteEvent{Object: secret})).To(BeTrue())
			})

			It("should return false because secret has no role label", func() {
				secret.Labels = nil
				Expect(p.Delete(event.DeleteEvent{Object: secret})).To(BeFalse())
			})

			It("should return false because secret has a different role", func() {
				secret.Labels["gardener.cloud/role"] = "controlplane-cert"
				Expect(p.Delete(event.DeleteEvent{Object: secret})).To(BeFalse())
			})
		})

		Describe("#Generic", func() {
			It("should return false", func() {
				Expect(p.Generic(event.GenericEvent{})).To(BeFalse())
			})
		})
	})
})
