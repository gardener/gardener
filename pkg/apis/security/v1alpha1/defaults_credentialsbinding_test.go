// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package v1alpha1_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/gardener/gardener/pkg/apis/security/v1alpha1"
)

var _ = Describe("CredentialsBinding defaulting", func() {
	var obj *CredentialsBinding

	Context("Secret", func() {
		BeforeEach(func() {
			obj = &CredentialsBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "credentialsbinding",
					Namespace: "test",
				},
				CredentialsRef: corev1.ObjectReference{
					APIVersion: "",
					Kind:       "Secret",
					Name:       "foo",
				},
			}
		})

		It("should default secret namespace", func() {
			SetObjectDefaults_CredentialsBinding(obj)

			Expect(obj.CredentialsRef.Namespace).NotTo(BeEmpty())
			Expect(obj.CredentialsRef.Namespace).To(Equal("test"))
		})

		It("should not overwrite already set values for secret namespace", func() {
			obj.CredentialsRef.Namespace = "other"

			SetObjectDefaults_CredentialsBinding(obj)

			Expect(obj.CredentialsRef.Namespace).To(Equal("other"))
		})

		It("should default quotas namespace", func() {
			obj.Quotas = []corev1.ObjectReference{{Name: "obj"}}

			SetObjectDefaults_CredentialsBinding(obj)

			Expect(obj.Quotas[0].Namespace).To(Equal("test"))
		})

		It("should not overwrite already set values for quotas namespace", func() {
			obj.Quotas = []corev1.ObjectReference{{Name: "obj", Namespace: "ns"}}

			SetObjectDefaults_CredentialsBinding(obj)

			Expect(obj.Quotas[0].Namespace).To(Equal("ns"))
		})
	})

	Context("WorkloadIdentity", func() {
		BeforeEach(func() {
			obj = &CredentialsBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "credentialsbinding",
					Namespace: "test",
				},
				CredentialsRef: corev1.ObjectReference{
					APIVersion: "security.gardener.cloud/v1alpha1",
					Kind:       "WorkloadIdentity",
					Name:       "bar",
				},
			}
		})

		It("should default workloadidentity namespace", func() {
			SetObjectDefaults_CredentialsBinding(obj)

			Expect(obj.CredentialsRef.Namespace).NotTo(BeEmpty())
			Expect(obj.CredentialsRef.Namespace).To(Equal("test"))
		})

		It("should not overwrite already set values for workloadidentity namespace", func() {
			obj.CredentialsRef.Namespace = "other"

			SetObjectDefaults_CredentialsBinding(obj)

			Expect(obj.CredentialsRef.Namespace).To(Equal("other"))
		})

		It("should default quotas namespace", func() {
			obj.Quotas = []corev1.ObjectReference{{Name: "obj"}}

			SetObjectDefaults_CredentialsBinding(obj)

			Expect(obj.Quotas[0].Namespace).To(Equal("test"))
		})

		It("should not overwrite already set values for quotas namespace", func() {
			obj.Quotas = []corev1.ObjectReference{{Name: "obj", Namespace: "ns"}}

			SetObjectDefaults_CredentialsBinding(obj)

			Expect(obj.Quotas[0].Namespace).To(Equal("ns"))
		})
	})
})
