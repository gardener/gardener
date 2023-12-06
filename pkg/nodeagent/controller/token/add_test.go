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

package token_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	. "github.com/gardener/gardener/pkg/nodeagent/controller/token"
)

var _ = Describe("Add", func() {
	Describe("#SecretPredicate", func() {
		var (
			p      predicate.Predicate
			secret *corev1.Secret
		)

		BeforeEach(func() {
			p = (&Reconciler{}).SecretPredicate()
			secret = &corev1.Secret{}
		})

		Describe("#Create", func() {
			It("should return true", func() {
				Expect(p.Create(event.CreateEvent{})).To(BeTrue())
			})
		})

		Describe("#Update", func() {
			It("should return false because old object is not a secret", func() {
				Expect(p.Update(event.UpdateEvent{})).To(BeFalse())
			})

			It("should return false because new object is not a secret", func() {
				Expect(p.Update(event.UpdateEvent{ObjectOld: secret})).To(BeFalse())
			})

			It("should return false because token data does not change", func() {
				Expect(p.Update(event.UpdateEvent{ObjectOld: secret, ObjectNew: secret})).To(BeFalse())
			})

			It("should return true because token data changes", func() {
				oldSecret := secret.DeepCopy()
				secret.Data = map[string][]byte{"token": []byte("foo")}
				Expect(p.Update(event.UpdateEvent{ObjectOld: oldSecret, ObjectNew: secret})).To(BeTrue())
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
})
