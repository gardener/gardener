// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package secretbinding

import (
	"fmt"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardencoreinformers "github.com/gardener/gardener/pkg/client/core/informers/externalversions"
	gardencorelisters "github.com/gardener/gardener/pkg/client/core/listers/core/v1beta1"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

var _ = Describe("SecretBindingControl", func() {
	Describe("#mayReleaseSecret", func() {
		var (
			gardenCoreInformerFactory gardencoreinformers.SharedInformerFactory
			c                         *defaultControl

			secretBinding1Namespace = "foo"
			secretBinding1Name      = "bar"
			secretBinding2Namespace = "baz"
			secretBinding2Name      = "bax"
			secretNamespace         = "foo"
			secretName              = "bar"
		)

		BeforeEach(func() {
			gardenCoreInformerFactory = gardencoreinformers.NewSharedInformerFactory(nil, 0)

			secretBindingInformer := gardenCoreInformerFactory.Core().V1beta1().SecretBindings()
			secretBindingLister := secretBindingInformer.Lister()

			c = &defaultControl{secretBindingLister: secretBindingLister}
		})

		It("should return true as no other secretbinding exists", func() {
			allowed, err := c.mayReleaseSecret(secretBinding1Namespace, secretBinding1Name, secretNamespace, secretName)

			Expect(allowed).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return true as no other secretbinding references the secret", func() {
			secretBinding := &gardencorev1beta1.SecretBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      secretBinding1Name,
					Namespace: secretBinding1Namespace,
				},
				SecretRef: corev1.SecretReference{
					Namespace: secretNamespace,
					Name:      secretName,
				},
			}

			Expect(gardenCoreInformerFactory.Core().V1beta1().SecretBindings().Informer().GetStore().Add(secretBinding)).To(Succeed())

			allowed, err := c.mayReleaseSecret(secretBinding1Namespace, secretBinding1Name, secretNamespace, secretName)

			Expect(allowed).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return false as another secretbinding references the secret", func() {
			secretBinding := &gardencorev1beta1.SecretBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      secretBinding2Name,
					Namespace: secretBinding2Namespace,
				},
				SecretRef: corev1.SecretReference{
					Namespace: secretNamespace,
					Name:      secretName,
				},
			}

			Expect(gardenCoreInformerFactory.Core().V1beta1().SecretBindings().Informer().GetStore().Add(secretBinding)).To(Succeed())

			allowed, err := c.mayReleaseSecret(secretBinding1Namespace, secretBinding1Name, secretNamespace, secretName)

			Expect(allowed).To(BeFalse())
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return an error as the list failed", func() {
			c.secretBindingLister = &fakeLister{
				SecretBindingLister: c.secretBindingLister,
			}

			allowed, err := c.mayReleaseSecret(secretBinding1Namespace, secretBinding1Name, secretNamespace, secretName)

			Expect(allowed).To(BeFalse())
			Expect(err).To(HaveOccurred())
		})
	})
})

type fakeLister struct {
	gardencorelisters.SecretBindingLister
}

func (c *fakeLister) List(labels.Selector) ([]*gardencorev1beta1.SecretBinding, error) {
	return nil, fmt.Errorf("fake error")
}
