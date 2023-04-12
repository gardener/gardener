// Copyright 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package v1alpha1_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	rbacv1 "k8s.io/api/rbac/v1"
	componentbaseconfigv1alpha1 "k8s.io/component-base/config/v1alpha1"

	. "github.com/gardener/gardener/pkg/admissioncontroller/apis/config/v1alpha1"
)

var _ = Describe("Defaults", func() {
	Describe("AdmissionControllerConfiguration", func() {
		var obj *AdmissionControllerConfiguration

		Context("Empty configuration", func() {
			BeforeEach(func() {
				obj = &AdmissionControllerConfiguration{}
			})

			It("should correctly default the admission controller configuration", func() {
				SetObjectDefaults_AdmissionControllerConfiguration(obj)

				Expect(obj.LogLevel).To(Equal("info"))
				Expect(obj.LogFormat).To(Equal("json"))
				Expect(obj.Server.Webhooks.BindAddress).To(BeEmpty())
				Expect(obj.Server.Webhooks.Port).To(Equal(2721))
				Expect(obj.Server.ResourceAdmissionConfiguration).To(Equal(&ResourceAdmissionConfiguration{}))
				Expect(obj.Server.HealthProbes.BindAddress).To(BeEmpty())
				Expect(obj.Server.HealthProbes.Port).To(Equal(2722))
				Expect(obj.Server.Metrics.BindAddress).To(BeEmpty())
				Expect(obj.Server.Metrics.Port).To(Equal(2723))
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
				SetObjectDefaults_AdmissionControllerConfiguration(obj)

				Expect(obj.Server.ResourceAdmissionConfiguration.UnrestrictedSubjects[0].APIGroup).To(Equal(rbacv1.GroupName))
				Expect(obj.Server.ResourceAdmissionConfiguration.UnrestrictedSubjects[1].APIGroup).To(Equal(rbacv1.GroupName))
				Expect(obj.Server.ResourceAdmissionConfiguration.UnrestrictedSubjects[2].APIGroup).To(Equal(""))
			})
		})

		Describe("GardenClientConnection", func() {
			It("should not default ContentType and AcceptContentTypes", func() {
				SetObjectDefaults_AdmissionControllerConfiguration(obj)

				// ContentType fields will be defaulted by client constructors / controller-runtime based on whether a
				// given APIGroup supports protobuf or not. defaults must not touch these, otherwise the integelligent
				// logic will be overwritten
				Expect(obj.GardenClientConnection.ContentType).To(BeEmpty())
				Expect(obj.GardenClientConnection.AcceptContentTypes).To(BeEmpty())
			})
			It("should correctly default GardenClientConnection", func() {
				SetObjectDefaults_AdmissionControllerConfiguration(obj)
				Expect(obj.GardenClientConnection).To(Equal(componentbaseconfigv1alpha1.ClientConnectionConfiguration{
					QPS:   50.0,
					Burst: 100,
				}))
			})
		})
	})
})
