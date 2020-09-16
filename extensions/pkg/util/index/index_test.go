// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package index_test

import (
	"testing"

	"github.com/gardener/gardener/extensions/pkg/util/index"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
)

func TestIndex(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Index suite")
}

var _ = Describe("Index", func() {

	Context("#SecretRefNamespaceIndexerFunc", func() {
		It("should return empty slice for non SecretBinding", func() {
			actual := index.SecretRefNamespaceIndexerFunc(&corev1.Secret{})
			Expect(actual).To(Equal([]string{}))
		})

		It("should return secretRef.namespace for SecretBinding", func() {
			secretBinding := &gardencorev1beta1.SecretBinding{
				SecretRef: corev1.SecretReference{
					Namespace: "garden-dev",
				},
			}

			actual := index.SecretRefNamespaceIndexerFunc(secretBinding)
			Expect(actual).To(Equal([]string{"garden-dev"}))
		})
	})

	Context("#SecretBindingNameIndexerFunc", func() {
		It("should return empty slice for non Shoot", func() {
			actual := index.SecretBindingNameIndexerFunc(&corev1.Pod{})
			Expect(actual).To(Equal([]string{}))
		})

		It("should return spec.secretBindingName for Shoot", func() {
			shoot := &gardencorev1beta1.Shoot{
				Spec: gardencorev1beta1.ShootSpec{
					SecretBindingName: "foo",
				},
			}

			actual := index.SecretBindingNameIndexerFunc(shoot)
			Expect(actual).To(Equal([]string{"foo"}))
		})
	})
})
