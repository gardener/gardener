// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	gomegatypes "github.com/onsi/gomega/types"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	componentbaseconfigv1alpha1 "k8s.io/component-base/config/v1alpha1"

	admissioncontrollerconfigv1alpha1 "github.com/gardener/gardener/pkg/admissioncontroller/apis/config/v1alpha1"
	. "github.com/gardener/gardener/pkg/admissioncontroller/apis/config/v1alpha1/validation"
)

var _ = Describe("#ValidateAdmissionControllerConfiguration", func() {
	var conf *admissioncontrollerconfigv1alpha1.AdmissionControllerConfiguration

	BeforeEach(func() {
		conf = &admissioncontrollerconfigv1alpha1.AdmissionControllerConfiguration{
			LogLevel:  "info",
			LogFormat: "text",
		}
	})

	Context("client connection configuration", func() {
		var (
			clientConnection *componentbaseconfigv1alpha1.ClientConnectionConfiguration
			fldPath          *field.Path
		)

		BeforeEach(func() {
			admissioncontrollerconfigv1alpha1.SetObjectDefaults_AdmissionControllerConfiguration(conf)

			clientConnection = &conf.GardenClientConnection
			fldPath = field.NewPath("gardenClientConnection")
		})

		It("should allow default client connection configuration", func() {
			Expect(ValidateAdmissionControllerConfiguration(conf)).To(BeEmpty())
		})

		It("should return errors because some values are invalid", func() {
			clientConnection.Burst = -1

			Expect(ValidateAdmissionControllerConfiguration(conf)).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fldPath.Child("burst").String()),
				})),
			))
		})
	})

	Context("Resource validation configuration", func() {
		DescribeTable("Operation mode validation",
			func(mode string, matcher gomegatypes.GomegaMatcher) {
				var (
					admissionConfig *admissioncontrollerconfigv1alpha1.ResourceAdmissionConfiguration
					webhookMode     = admissioncontrollerconfigv1alpha1.ResourceAdmissionWebhookMode(mode)
				)
				if mode != "" {
					admissionConfig = &admissioncontrollerconfigv1alpha1.ResourceAdmissionConfiguration{
						OperationMode: &webhookMode,
					}
				}

				config := &admissioncontrollerconfigv1alpha1.AdmissionControllerConfiguration{
					LogLevel:  "info",
					LogFormat: "json",
					Server: admissioncontrollerconfigv1alpha1.ServerConfiguration{
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
				config := &admissioncontrollerconfigv1alpha1.AdmissionControllerConfiguration{
					LogLevel:  "info",
					LogFormat: "json",
					Server: admissioncontrollerconfigv1alpha1.ServerConfiguration{
						ResourceAdmissionConfiguration: &admissioncontrollerconfigv1alpha1.ResourceAdmissionConfiguration{
							Limits: []admissioncontrollerconfigv1alpha1.ResourceLimit{
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
				config := &admissioncontrollerconfigv1alpha1.AdmissionControllerConfiguration{
					LogLevel:  "info",
					LogFormat: "json",
					Server: admissioncontrollerconfigv1alpha1.ServerConfiguration{
						ResourceAdmissionConfiguration: &admissioncontrollerconfigv1alpha1.ResourceAdmissionConfiguration{
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
				config := &admissioncontrollerconfigv1alpha1.AdmissionControllerConfiguration{
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
