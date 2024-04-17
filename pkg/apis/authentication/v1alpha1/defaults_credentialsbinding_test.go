// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package v1alpha1_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/gardener/gardener/pkg/apis/authentication/v1alpha1"
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
				CredentialsRef: Credentials{
					Secret: &corev1.SecretReference{
						Name: "secret",
					},
				},
			}
		})

		It("should default secret namespace", func() {
			SetObjectDefaults_CredentialsBinding(obj)

			Expect(obj.CredentialsRef.Secret.Namespace).NotTo(BeNil())
			Expect(obj.CredentialsRef.Secret.Namespace).To(Equal("test"))
		})

		It("should not overwrite already set values for secret namespace", func() {
			obj.CredentialsRef.Secret.Namespace = "other"

			SetObjectDefaults_CredentialsBinding(obj)

			Expect(obj.CredentialsRef.Secret.Namespace).To(Equal("other"))
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

		It("should not default unset workloadidentity", func() {
			SetObjectDefaults_CredentialsBinding(obj)

			Expect(obj.CredentialsRef.WorkloadIdentity).To(BeNil())
		})
	})

	Context("WorkloadIdentity", func() {
		BeforeEach(func() {
			obj = &CredentialsBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "credentialsbinding",
					Namespace: "test",
				},
				CredentialsRef: Credentials{
					WorkloadIdentity: &WorkloadIdentityReference{
						Name: "workloadidentity",
					},
				},
			}
		})

		It("should default workloadidentity namespace", func() {
			SetObjectDefaults_CredentialsBinding(obj)

			Expect(obj.CredentialsRef.WorkloadIdentity.Namespace).NotTo(BeNil())
			Expect(obj.CredentialsRef.WorkloadIdentity.Namespace).To(Equal("test"))
		})

		It("should not overwrite already set values for workloadidentity namespace", func() {
			obj.CredentialsRef.WorkloadIdentity.Namespace = "other"

			SetObjectDefaults_CredentialsBinding(obj)

			Expect(obj.CredentialsRef.WorkloadIdentity.Namespace).To(Equal("other"))
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

		It("should not default unset secret", func() {
			SetObjectDefaults_CredentialsBinding(obj)

			Expect(obj.CredentialsRef.Secret).To(BeNil())
		})
	})
})
