// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package v1beta1_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

var _ = Describe("SecretBinding defaulting", func() {
	var obj *SecretBinding

	BeforeEach(func() {
		obj = &SecretBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "secretbinding",
				Namespace: "test",
			},
			SecretRef: corev1.SecretReference{
				Name: "secret",
			},
		}
	})

	It("should default secretRef namespace", func() {
		SetObjectDefaults_SecretBinding(obj)

		Expect(obj.SecretRef.Namespace).NotTo(BeNil())
		Expect(obj.SecretRef.Namespace).To(Equal("test"))
	})

	It("should not overwrite already set values for secretRef namespace", func() {
		obj.SecretRef.Namespace = "other"

		SetObjectDefaults_SecretBinding(obj)

		Expect(obj.SecretRef.Namespace).To(Equal("other"))
	})

	It("should default quotas namespace", func() {
		obj.Quotas = []corev1.ObjectReference{{Name: "obj"}}

		SetObjectDefaults_SecretBinding(obj)

		Expect(obj.Quotas[0].Namespace).To(Equal("test"))
	})

	It("should not overwrite already set values for quotas namespace", func() {
		obj.Quotas = []corev1.ObjectReference{{Name: "obj", Namespace: "ns"}}

		SetObjectDefaults_SecretBinding(obj)

		Expect(obj.Quotas[0].Namespace).To(Equal("ns"))
	})
})
