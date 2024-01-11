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
	var obj *AdmissionControllerConfiguration

	BeforeEach(func() {
		obj = &AdmissionControllerConfiguration{}
	})

	Describe("AdmissionControllerConfiguration defaulting", func() {
		It("should correctly default the AdmissionControllerConfiguration", func() {
			SetObjectDefaults_AdmissionControllerConfiguration(obj)

			Expect(obj.LogLevel).To(Equal("info"))
			Expect(obj.LogFormat).To(Equal("json"))
		})

		It("should not overwrite already set values", func() {
			obj = &AdmissionControllerConfiguration{
				LogLevel:  "warning",
				LogFormat: "md",
			}
			SetObjectDefaults_AdmissionControllerConfiguration(obj)

			Expect(obj.LogLevel).To(Equal("warning"))
			Expect(obj.LogFormat).To(Equal("md"))
		})
	})

	Describe("GardenClientConnection defaulting", func() {
		It("should not default ContentType and AcceptContentTypes", func() {
			SetObjectDefaults_AdmissionControllerConfiguration(obj)
			// ContentType fields will be defaulted by client constructors / controller-runtime based on whether a
			// given APIGroup supports protobuf or not. defaults must not touch these, otherwise the intelligent
			// logic will be overwritten
			Expect(obj.GardenClientConnection.ContentType).To(BeEmpty())
			Expect(obj.GardenClientConnection.AcceptContentTypes).To(BeEmpty())
		})

		It("should correctly default the GardenClientConnection", func() {
			expected := componentbaseconfigv1alpha1.ClientConnectionConfiguration{
				QPS:   50.0,
				Burst: 100,
			}
			SetObjectDefaults_AdmissionControllerConfiguration(obj)

			Expect(obj.GardenClientConnection).To(Equal(expected))
		})

		It("should not overwrite already set values", func() {
			obj = &AdmissionControllerConfiguration{
				GardenClientConnection: componentbaseconfigv1alpha1.ClientConnectionConfiguration{
					QPS:   67.0,
					Burst: 230,
				},
			}
			expected := obj.GardenClientConnection.DeepCopy()
			SetObjectDefaults_AdmissionControllerConfiguration(obj)

			Expect(&obj.GardenClientConnection).To(Equal(expected))
		})
	})

	Describe("ServerConfiguration defaulting", func() {
		It("should correctly default the ServerConfiguration", func() {
			expected := &ServerConfiguration{
				Webhooks: HTTPSServer{
					Server: Server{
						BindAddress: "",
						Port:        2721,
					},
				},
				ResourceAdmissionConfiguration: &ResourceAdmissionConfiguration{},
				HealthProbes: &Server{
					BindAddress: "",
					Port:        2722,
				},
				Metrics: &Server{
					BindAddress: "",
					Port:        2723,
				},
			}
			SetObjectDefaults_AdmissionControllerConfiguration(obj)

			Expect(&obj.Server).To(Equal(expected))
		})

		It("should not overwrite already set values", func() {
			obj = &AdmissionControllerConfiguration{
				Server: ServerConfiguration{
					Webhooks: HTTPSServer{
						Server: Server{
							BindAddress: "foo",
							Port:        1234,
						},
					},
					HealthProbes: &Server{
						BindAddress: "bar",
						Port:        4321,
					},
					Metrics: &Server{
						BindAddress: "baz",
						Port:        5555,
					},
					ResourceAdmissionConfiguration: &ResourceAdmissionConfiguration{},
				},
			}
			expected := obj.Server.DeepCopy()
			SetObjectDefaults_AdmissionControllerConfiguration(obj)

			Expect(&obj.Server).To(Equal(expected))
		})
	})

	Describe("ResourceAdmissionConfiguration defaulting", func() {
		It("should correctly default the resource admission configuration", func() {
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
			expected := &ResourceAdmissionConfiguration{
				UnrestrictedSubjects: []rbacv1.Subject{
					{Kind: rbacv1.UserKind, Name: "foo", APIGroup: rbacv1.GroupName},
					{Kind: rbacv1.GroupKind, Name: "bar", APIGroup: rbacv1.GroupName},
					{Kind: rbacv1.ServiceAccountKind, Name: "foobar", Namespace: "default", APIGroup: ""},
				},
			}
			SetObjectDefaults_AdmissionControllerConfiguration(obj)

			Expect(obj.Server.ResourceAdmissionConfiguration).To(Equal(expected))
		})

		It("should not overwrite already set values", func() {
			obj = &AdmissionControllerConfiguration{
				Server: ServerConfiguration{
					ResourceAdmissionConfiguration: &ResourceAdmissionConfiguration{
						UnrestrictedSubjects: []rbacv1.Subject{
							{Kind: rbacv1.UserKind, Name: "foo", APIGroup: "fooGroup"},
							{Kind: rbacv1.GroupKind, Name: "bar", APIGroup: "barGroup"},
							{Kind: rbacv1.ServiceAccountKind, Name: "foobar", Namespace: "default", APIGroup: "foobarGroup"},
						},
					},
				},
			}
			expected := obj.Server.ResourceAdmissionConfiguration.DeepCopy()
			SetObjectDefaults_AdmissionControllerConfiguration(obj)

			Expect(obj.Server.ResourceAdmissionConfiguration).To(Equal(expected))
		})
	})
})
