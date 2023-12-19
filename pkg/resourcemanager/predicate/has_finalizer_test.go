// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
// SPDX-License-Identifier: Apache-2.0

package predicate_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	. "github.com/gardener/gardener/pkg/resourcemanager/predicate"
)

var _ = Describe("#HasFinalizer", func() {
	var (
		secret    *corev1.Secret
		finalizer string
		predicate predicate.Predicate
	)

	BeforeEach(func() {
		finalizer = "foo"
		predicate = HasFinalizer(finalizer)

		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Finalizers: nil,
			},
		}
	})

	Context("#Create", func() {
		It("should not match on create event (no finalizer)", func() {
			Expect(predicate.Create(event.CreateEvent{
				Object: secret,
			})).To(BeFalse())
		})

		It("should not match on create event (different finalizer)", func() {
			secret.Finalizers = []string{"other"}

			Expect(predicate.Create(event.CreateEvent{
				Object: secret,
			})).To(BeFalse())
		})

		It("should match on create event (correct finalizer)", func() {
			secret.Finalizers = []string{finalizer}

			Expect(predicate.Create(event.CreateEvent{
				Object: secret,
			})).To(BeTrue())
		})
	})

	Context("#Update", func() {
		It("should not match on update event (no finalizer)", func() {
			Expect(predicate.Update(event.UpdateEvent{
				ObjectOld: secret,
				ObjectNew: secret,
			})).To(BeFalse())
		})

		It("should not match on update event (different finalizer)", func() {
			secret.Finalizers = []string{"other"}

			Expect(predicate.Update(event.UpdateEvent{
				ObjectOld: secret,
				ObjectNew: secret,
			})).To(BeFalse())
		})

		It("should not match on update event (correct finalizer removed)", func() {
			secretCopy := *secret
			secret.Finalizers = []string{finalizer}

			Expect(predicate.Update(event.UpdateEvent{
				ObjectOld: secret,
				ObjectNew: &secretCopy,
			})).To(BeFalse())
		})

		It("should match on update event (correct finalizer added)", func() {
			secretCopy := *secret
			secret.Finalizers = []string{finalizer}

			Expect(predicate.Update(event.UpdateEvent{
				ObjectOld: &secretCopy,
				ObjectNew: secret,
			})).To(BeTrue())
		})

		It("should match on update event (correct finalizer)", func() {
			secret.Finalizers = []string{finalizer}

			Expect(predicate.Update(event.UpdateEvent{
				ObjectOld: secret,
				ObjectNew: secret,
			})).To(BeTrue())
		})
	})

	Describe("#Delete", func() {
		It("should not match on delete event", func() {
			Expect(predicate.Delete(event.DeleteEvent{
				Object: secret,
			})).To(BeFalse())
		})
	})

	Describe("#Generic", func() {
		It("should not match on generic event", func() {
			Expect(predicate.Generic(event.GenericEvent{
				Object: secret,
			})).To(BeFalse())
		})
	})
})
