// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	gomegatypes "github.com/onsi/gomega/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"

	operatorconfigv1alpha1 "github.com/gardener/gardener/pkg/operator/apis/config/v1alpha1"
	. "github.com/gardener/gardener/pkg/operator/apis/config/v1alpha1/validation"
)

var _ = Describe("#ValidateOperatorConfiguration", func() {
	var conf *operatorconfigv1alpha1.OperatorConfiguration

	BeforeEach(func() {
		conf = &operatorconfigv1alpha1.OperatorConfiguration{
			LogLevel:  "info",
			LogFormat: "text",
			Server: operatorconfigv1alpha1.ServerConfiguration{
				HealthProbes: &operatorconfigv1alpha1.Server{
					Port: 1234,
				},
				Metrics: &operatorconfigv1alpha1.Server{
					Port: 5678,
				},
			},
			Controllers: operatorconfigv1alpha1.ControllerConfiguration{
				Garden: operatorconfigv1alpha1.GardenControllerConfig{
					ConcurrentSyncs: ptr.To(5),
					SyncPeriod:      &metav1.Duration{Duration: time.Minute},
				},
				GardenCare: operatorconfigv1alpha1.GardenCareControllerConfiguration{
					SyncPeriod: &metav1.Duration{Duration: time.Minute},
				},
				GardenletDeployer: operatorconfigv1alpha1.GardenletDeployerControllerConfig{
					ConcurrentSyncs: ptr.To(5),
				},
				NetworkPolicy: operatorconfigv1alpha1.NetworkPolicyControllerConfiguration{
					ConcurrentSyncs: ptr.To(5),
				},
			},
		}
	})

	It("should return no errors because the config is valid", func() {
		Expect(ValidateOperatorConfiguration(conf)).To(BeEmpty())
	})

	DescribeTable("logging configuration",
		func(logLevel, logFormat string, matcher gomegatypes.GomegaMatcher) {
			conf.LogLevel = logLevel
			conf.LogFormat = logFormat

			Expect(ValidateOperatorConfiguration(conf)).To(matcher)
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

	Context("controller configuration", func() {
		Context("garden", func() {
			It("should return errors because concurrent syncs are <= 0", func() {
				conf.Controllers.Garden.ConcurrentSyncs = ptr.To(0)
				conf.Controllers.Garden.SyncPeriod = &metav1.Duration{Duration: time.Hour}

				Expect(ValidateOperatorConfiguration(conf)).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("controllers.garden.concurrentSyncs"),
					})),
				))
			})

			It("should return errors because sync period is nil", func() {
				conf.Controllers.Garden.ConcurrentSyncs = ptr.To(5)
				conf.Controllers.Garden.SyncPeriod = nil

				Expect(ValidateOperatorConfiguration(conf)).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("controllers.garden.syncPeriod"),
					})),
				))
			})

			It("should return errors because sync period is < 15s", func() {
				conf.Controllers.Garden.ConcurrentSyncs = ptr.To(5)
				conf.Controllers.Garden.SyncPeriod = &metav1.Duration{Duration: time.Second}

				Expect(ValidateOperatorConfiguration(conf)).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("controllers.garden.syncPeriod"),
					})),
				))
			})
		})

		Context("GardenCare", func() {
			It("should return errors because sync period is nil", func() {
				conf.Controllers.GardenCare.SyncPeriod = nil

				Expect(ValidateOperatorConfiguration(conf)).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("controllers.gardenCare.syncPeriod"),
					})),
				))
			})

			It("should return errors because sync period is < 15s", func() {
				conf.Controllers.GardenCare.SyncPeriod = &metav1.Duration{Duration: time.Second}

				Expect(ValidateOperatorConfiguration(conf)).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("controllers.gardenCare.syncPeriod"),
					})),
				))
			})
		})

		Context("network policy", func() {
			It("should return errors because concurrent syncs are <= 0", func() {
				conf.Controllers.NetworkPolicy.ConcurrentSyncs = ptr.To(0)

				Expect(ValidateOperatorConfiguration(conf)).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("controllers.networkPolicy.concurrentSyncs"),
					})),
				))
			})

			It("should return errors because some label selector is invalid", func() {
				conf.Controllers.NetworkPolicy.AdditionalNamespaceSelectors = append(conf.Controllers.NetworkPolicy.AdditionalNamespaceSelectors,
					metav1.LabelSelector{MatchLabels: map[string]string{"foo": "bar"}},
					metav1.LabelSelector{MatchLabels: map[string]string{"foo": "no/slash/allowed"}},
				)

				Expect(ValidateOperatorConfiguration(conf)).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("controllers.networkPolicy.additionalNamespaceSelectors[1].matchLabels"),
					})),
				))
			})
		})
	})

	Context("node toleration", func() {
		It("should pass with unset toleration options", func() {
			conf.NodeToleration = nil

			Expect(ValidateOperatorConfiguration(conf)).To(BeEmpty())
		})

		It("should pass with unset toleration seconds", func() {
			conf.NodeToleration = &operatorconfigv1alpha1.NodeTolerationConfiguration{
				DefaultNotReadyTolerationSeconds:    nil,
				DefaultUnreachableTolerationSeconds: nil,
			}

			Expect(ValidateOperatorConfiguration(conf)).To(BeEmpty())
		})

		It("should pass with valid toleration options", func() {
			conf.NodeToleration = &operatorconfigv1alpha1.NodeTolerationConfiguration{
				DefaultNotReadyTolerationSeconds:    ptr.To[int64](60),
				DefaultUnreachableTolerationSeconds: ptr.To[int64](120),
			}

			Expect(ValidateOperatorConfiguration(conf)).To(BeEmpty())
		})

		It("should fail with invalid toleration options", func() {
			conf.NodeToleration = &operatorconfigv1alpha1.NodeTolerationConfiguration{
				DefaultNotReadyTolerationSeconds:    ptr.To(int64(-1)),
				DefaultUnreachableTolerationSeconds: ptr.To(int64(-2)),
			}

			errorList := ValidateOperatorConfiguration(conf)

			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("nodeToleration.defaultNotReadyTolerationSeconds"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("nodeToleration.defaultUnreachableTolerationSeconds"),
				}))),
			)
		})
	})
})
