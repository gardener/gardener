// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package v1alpha1_test

import (
	. "github.com/gardener/gardener/pkg/admissioncontroller/apis/config/v1alpha1"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	rbacv1 "k8s.io/api/rbac/v1"
)

var _ = Describe("Defaults", func() {
	Describe("#SetDefaults_AdmissionControllerConfiguration", func() {
		var obj *AdmissionControllerConfiguration

		Context("Empty configuration", func() {
			BeforeEach(func() {
				obj = &AdmissionControllerConfiguration{}
			})

			It("should correctly default the admission controller configuration", func() {
				SetDefaults_AdmissionControllerConfiguration(obj)

				Expect(obj.LogLevel).To(Equal("info"))
				Expect(obj.Server.HTTPS.BindAddress).To(Equal("0.0.0.0"))
				Expect(obj.Server.HTTPS.Port).To(Equal(2721))
				Expect(obj.Server.ResourceAdmissionConfiguration).To(Equal(&ResourceAdmissionConfiguration{}))
			})
		})

		Context("Resource Admission Configuration", func() {
			BeforeEach(func() {
				obj = &AdmissionControllerConfiguration{
					Server: ServerConfiguration{
						ResourceAdmissionConfiguration: &ResourceAdmissionConfiguration{
							UnrestrictedSubjects: []rbacv1.Subject{
								{Kind: rbacv1.UserKind, Name: "foo"},
								{Kind: rbacv1.GroupKind, Name: "bar"},
								{Kind: rbacv1.ServiceAccountKind, Name: "foobar", Namespace: "default"},
							},
						},
					},
				}
			})
			It("should correctly default the resource admission configuration if given", func() {
				SetDefaults_AdmissionControllerConfiguration(obj)

				Expect(obj.Server.ResourceAdmissionConfiguration.UnrestrictedSubjects[0].APIGroup).To(Equal(rbacv1.GroupName))
				Expect(obj.Server.ResourceAdmissionConfiguration.UnrestrictedSubjects[1].APIGroup).To(Equal(rbacv1.GroupName))
				Expect(obj.Server.ResourceAdmissionConfiguration.UnrestrictedSubjects[2].APIGroup).To(Equal(""))
			})
		})
	})
})
