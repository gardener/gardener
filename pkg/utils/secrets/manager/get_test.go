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

package manager

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Get", func() {
	var m *manager

	BeforeEach(func() {
		m = &manager{store: make(secretStore)}
	})

	Describe("#Get", func() {
		name := "some-name"

		It("should return an error because no secrets found for name", func() {
			result, found := m.Get(name)
			Expect(found).To(BeFalse())
			Expect(result).To(BeNil())
		})

		Context("bundle", func() {
			It("should return an error since there is no bundle secret in the internal store", func() {
				currentSecret := secretForClass(current)
				Expect(m.addToStore(name, currentSecret, current)).To(Succeed())

				result, found := m.Get(name, Bundle)
				Expect(found).To(BeFalse())
				Expect(result).To(BeNil())
			})

			secret := secretForClass(bundle)

			It("should get the bundle secret from the internal store", func() {
				Expect(m.addToStore(name, secret, bundle)).To(Succeed())

				result, found := m.Get(name, Bundle)
				Expect(found).To(BeTrue())
				Expect(result).To(Equal(secret))
			})

			It("should get the bundle secret from the internal store (w/o explicit option)", func() {
				Expect(m.addToStore(name, secret, bundle)).To(Succeed())

				result, found := m.Get(name)
				Expect(found).To(BeTrue())
				Expect(result).To(Equal(secret))
			})

		})

		Context("current", func() {
			var (
				currentSecret = secretForClass(current)
				bundleSecret  = secretForClass(bundle)
			)

			BeforeEach(func() {
				Expect(m.addToStore(name, currentSecret, current)).To(Succeed())
			})

			It("should get the bundle secret from the internal store (default behaviour w/o options)", func() {
				Expect(m.addToStore(name, bundleSecret, bundle)).To(Succeed())

				result, found := m.Get(name)
				Expect(found).To(BeTrue())
				Expect(result).To(Equal(bundleSecret))
			})

			It("should get the current secret from the internal store since there is no bundle secret", func() {
				result, found := m.Get(name)
				Expect(found).To(BeTrue())
				Expect(result).To(Equal(currentSecret))
			})

			It("should get the current secret from the internal store despite a bundle secret (w/ explicit option)", func() {
				Expect(m.addToStore(name, bundleSecret, bundle)).To(Succeed())

				result, found := m.Get(name, Current)
				Expect(found).To(BeTrue())
				Expect(result).To(Equal(currentSecret))
			})
		})

		Context("old", func() {
			It("should return an error since there is no old secret in the internal store", func() {
				currentSecret := secretForClass(current)
				Expect(m.addToStore(name, currentSecret, current)).To(Succeed())

				result, found := m.Get(name, Old)
				Expect(found).To(BeFalse())
				Expect(result).To(BeNil())
			})

			It("should get the old secret from the internal store", func() {
				oldSecret := secretForClass(old)
				Expect(m.addToStore(name, oldSecret, old)).To(Succeed())

				result, found := m.Get(name, Old)
				Expect(found).To(BeTrue())
				Expect(result).To(Equal(oldSecret))
			})
		})
	})
})

func secretForClass(class secretClass) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: string(class) + "secret",
		},
	}
}
