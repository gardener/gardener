// Copyright 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
