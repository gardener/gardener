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

package validation_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	gomegatypes "github.com/onsi/gomega/types"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	admissioncontrollerconfig "github.com/gardener/gardener/pkg/admissioncontroller/apis/config"
	. "github.com/gardener/gardener/pkg/admissioncontroller/apis/config/validation"
)

var _ = Describe("#ValidateAdmissionControllerConfiguration", func() {
	Context("Resource validation configuration", func() {
		DescribeTable("Operation mode validation",
			func(mode string, matcher gomegatypes.GomegaMatcher) {
				var (
					admissionConfig *admissioncontrollerconfig.ResourceAdmissionConfiguration
					webhookMode     = admissioncontrollerconfig.ResourceAdmissionWebhookMode(mode)
				)
				if mode != "" {
					admissionConfig = &admissioncontrollerconfig.ResourceAdmissionConfiguration{
						OperationMode: &webhookMode,
					}
				}

				config := &admissioncontrollerconfig.AdmissionControllerConfiguration{
					LogLevel:  "info",
					LogFormat: "json",
					Server: admissioncontrollerconfig.ServerConfiguration{
						ResourceAdmissionConfiguration: admissionConfig,
					},
				}

				errs := ValidateAdmissionControllerConfiguration(config)

				Expect(errs).To(matcher)

			},
			Entry("should allow no mode", "", BeEmpty()),
			Entry("should allow blocking mode", "block", BeEmpty()),
			Entry("should allow logging mode", "log", BeEmpty()),
			Entry("should deny non existing mode", "foo", Not(BeEmpty())),
		)

		var (
			apiGroups = []string{"core.gardener.cloud"}
			versions  = []string{"v1beta1", "v1alpha1"}
			resources = []string{"shoot"}
			size      = "1Ki"
		)

		DescribeTable("Limits validation",
			func(apiGroups []string, versions []string, resources []string, size string, matcher gomegatypes.GomegaMatcher) {
				s, err := resource.ParseQuantity(size)
				utilruntime.Must(err)
				config := &admissioncontrollerconfig.AdmissionControllerConfiguration{
					LogLevel:  "info",
					LogFormat: "json",
					Server: admissioncontrollerconfig.ServerConfiguration{
						ResourceAdmissionConfiguration: &admissioncontrollerconfig.ResourceAdmissionConfiguration{
							Limits: []admissioncontrollerconfig.ResourceLimit{
								{
									APIGroups:   apiGroups,
									APIVersions: versions,
									Resources:   resources,
									Size:        s,
								},
							},
						},
					},
				}

				errs := ValidateAdmissionControllerConfiguration(config)

				Expect(errs).To(matcher)

			},
			Entry("should allow request", apiGroups, versions, resources, size,
				BeEmpty(),
			),
			Entry("should deny empty apiGroup", nil, versions, resources, size,
				ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{"Field": Equal("server.resourceAdmissionConfiguration.limits[0].apiGroups")}))),
			),
			Entry("should allow apiGroup w/ zero length", []string{""}, versions, resources, size,
				BeEmpty(),
			),
			Entry("should deny empty versions", apiGroups, nil, resources, size,
				ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{"Field": Equal("server.resourceAdmissionConfiguration.limits[0].versions")}))),
			),
			Entry("should deny versions w/ zero length", apiGroups, []string{""}, resources, size,
				ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{"Field": Equal("server.resourceAdmissionConfiguration.limits[0].versions[0]")}))),
			),
			Entry("should deny empty resources", apiGroups, versions, nil, size,
				ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{"Field": Equal("server.resourceAdmissionConfiguration.limits[0].resources")}))),
			),
			Entry("should deny resources w/ zero length", apiGroups, versions, []string{""}, size,
				ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{"Field": Equal("server.resourceAdmissionConfiguration.limits[0].resources[0]")}))),
			),
			Entry("should deny invalid size", apiGroups, versions, resources, "-1k",
				ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{"Field": Equal("server.resourceAdmissionConfiguration.limits[0].size")}))),
			),
			Entry("should deny invalid size and resources w/ zero length", apiGroups, versions, []string{resources[0], ""}, "-1k",
				ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{"Field": Equal("server.resourceAdmissionConfiguration.limits[0].size")})),
					PointTo(MatchFields(IgnoreExtras, Fields{"Field": Equal("server.resourceAdmissionConfiguration.limits[0].resources[1]")})),
				),
			),
		)

		var (
			userName       = "admin"
			namespace      = "default"
			emptyNamespace = ""
		)

		DescribeTable("User configuration validation",
			func(kind string, name string, namespace string, apiGroup string, matcher gomegatypes.GomegaMatcher) {
				config := &admissioncontrollerconfig.AdmissionControllerConfiguration{
					LogLevel:  "info",
					LogFormat: "json",
					Server: admissioncontrollerconfig.ServerConfiguration{
						ResourceAdmissionConfiguration: &admissioncontrollerconfig.ResourceAdmissionConfiguration{
							UnrestrictedSubjects: []rbacv1.Subject{
								{
									Kind:      kind,
									Name:      name,
									Namespace: namespace,
									APIGroup:  apiGroup,
								},
							},
						},
					},
				}

				errs := ValidateAdmissionControllerConfiguration(config)

				Expect(errs).To(matcher)

			},
			Entry("should allow request for user", rbacv1.UserKind, userName, emptyNamespace, rbacv1.GroupName,
				BeEmpty(),
			),
			Entry("should allow request for group", rbacv1.GroupKind, userName, emptyNamespace, rbacv1.GroupName,
				BeEmpty(),
			),
			Entry("should allow request for service account", rbacv1.ServiceAccountKind, userName, namespace, "",
				BeEmpty(),
			),
			Entry("should deny invalid apiGroup for user", rbacv1.UserKind, userName, emptyNamespace, "invalid",
				ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{"Field": Equal("server.resourceAdmissionConfiguration.unrestrictedSubjects[0].apiGroup")}))),
			),
			Entry("should deny invalid apiGroup for group", rbacv1.GroupKind, userName, emptyNamespace, "invalid",
				ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{"Field": Equal("server.resourceAdmissionConfiguration.unrestrictedSubjects[0].apiGroup")}))),
			),
			Entry("should deny invalid apiGroup for service account", rbacv1.ServiceAccountKind, userName, namespace, "invalid",
				ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{"Field": Equal("server.resourceAdmissionConfiguration.unrestrictedSubjects[0].apiGroup")}))),
			),
			Entry("should deny invalid namespace setting for user", rbacv1.UserKind, userName, namespace, rbacv1.GroupName,
				ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{"Field": Equal("server.resourceAdmissionConfiguration.unrestrictedSubjects[0].namespace")}))),
			),
			Entry("should deny invalid namespace setting for group", rbacv1.GroupKind, userName, namespace, rbacv1.GroupName,
				ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{"Field": Equal("server.resourceAdmissionConfiguration.unrestrictedSubjects[0].namespace")}))),
			),
			Entry("should deny invalid kind", "invalidKind", userName, emptyNamespace, rbacv1.GroupName,
				ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{"Field": Equal("server.resourceAdmissionConfiguration.unrestrictedSubjects[0].kind")}))),
			),
			Entry("should deny empty name", rbacv1.UserKind, "", emptyNamespace, rbacv1.GroupName,
				ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{"Field": Equal("server.resourceAdmissionConfiguration.unrestrictedSubjects[0].name")}))),
			),
		)
		DescribeTable("Logging configuration",
			func(logLevel, logFormat string, matcher gomegatypes.GomegaMatcher) {
				config := &admissioncontrollerconfig.AdmissionControllerConfiguration{
					LogLevel:  logLevel,
					LogFormat: logFormat,
				}

				errs := ValidateAdmissionControllerConfiguration(config)
				Expect(errs).To(matcher)
			},
			Entry("should be a valid logging configuration", "debug", "json", BeEmpty()),
			Entry("should be a valid logging configuration", "info", "json", BeEmpty()),
			Entry("should be a valid logging configuration", "error", "json", BeEmpty()),
			Entry("should be a valid logging configuration", "info", "text", BeEmpty()),
			Entry("should be an invalid logging level configuration", "foo", "json",
				ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{"Field": Equal("logLevel")}))),
			),
			Entry("should be an invalid logging format configuration", "info", "foo",
				ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{"Field": Equal("logFormat")}))),
			),
		)
	})

})
