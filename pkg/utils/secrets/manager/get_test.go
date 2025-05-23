// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

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
