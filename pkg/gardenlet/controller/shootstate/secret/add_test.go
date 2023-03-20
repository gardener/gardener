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

package secret_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	. "github.com/gardener/gardener/pkg/gardenlet/controller/shootstate/secret"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

var _ = Describe("Add", func() {
	var reconciler *Reconciler

	BeforeEach(func() {
		reconciler = &Reconciler{}
	})

	Describe("SecretPredicate", func() {
		var (
			p      predicate.Predicate
			secret *corev1.Secret
		)

		BeforeEach(func() {
			p = reconciler.SecretPredicate()
			secret = &corev1.Secret{}
		})

		It("should return false if required label is not present on secret", func() {
			Expect(p.Create(event.CreateEvent{Object: secret})).To(BeFalse())
			Expect(p.Update(event.UpdateEvent{ObjectNew: secret})).To(BeFalse())
			Expect(p.Delete(event.DeleteEvent{Object: secret})).To(BeFalse())
		})

		It("should return true if required label is present on secret", func() {
			secret.ObjectMeta = metav1.ObjectMeta{
				Labels: map[string]string{
					secretsmanager.LabelKeyManagedBy: secretsmanager.LabelValueSecretsManager,
					secretsmanager.LabelKeyPersist:   secretsmanager.LabelValueTrue,
				},
			}

			Expect(p.Create(event.CreateEvent{Object: secret})).To(BeTrue())
			Expect(p.Update(event.UpdateEvent{ObjectNew: secret})).To(BeTrue())
			Expect(p.Delete(event.DeleteEvent{Object: secret})).To(BeTrue())
		})
	})
})
