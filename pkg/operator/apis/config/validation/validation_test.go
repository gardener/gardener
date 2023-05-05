// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	gomegatypes "github.com/onsi/gomega/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/pointer"

	"github.com/gardener/gardener/pkg/operator/apis/config"
	. "github.com/gardener/gardener/pkg/operator/apis/config/validation"
)

var _ = Describe("#ValidateOperatorConfiguration", func() {
	var conf *config.OperatorConfiguration

	BeforeEach(func() {
		conf = &config.OperatorConfiguration{
			LogLevel:  "info",
			LogFormat: "text",
			Server: config.ServerConfiguration{
				HealthProbes: &config.Server{
					Port: 1234,
				},
				Metrics: &config.Server{
					Port: 5678,
				},
			},
			Controllers: config.ControllerConfiguration{
				Garden: config.GardenControllerConfig{
					ConcurrentSyncs: pointer.Int(5),
					SyncPeriod:      &metav1.Duration{Duration: time.Minute},
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
				conf.Controllers.Garden.ConcurrentSyncs = pointer.Int(0)
				conf.Controllers.Garden.SyncPeriod = &metav1.Duration{Duration: time.Hour}

				Expect(ValidateOperatorConfiguration(conf)).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("controllers.garden.concurrentSyncs"),
					})),
				))
			})

			It("should return errors because sync period is nil", func() {
				conf.Controllers.Garden.ConcurrentSyncs = pointer.Int(5)
				conf.Controllers.Garden.SyncPeriod = nil

				Expect(ValidateOperatorConfiguration(conf)).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("controllers.garden.syncPeriod"),
					})),
				))
			})

			It("should return errors because sync period is < 15s", func() {
				conf.Controllers.Garden.ConcurrentSyncs = pointer.Int(5)
				conf.Controllers.Garden.SyncPeriod = &metav1.Duration{Duration: time.Second}

				Expect(ValidateOperatorConfiguration(conf)).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("controllers.garden.syncPeriod"),
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
			conf.NodeToleration = &config.NodeTolerationConfiguration{
				DefaultNotReadyTolerationSeconds:    nil,
				DefaultUnreachableTolerationSeconds: nil,
			}

			Expect(ValidateOperatorConfiguration(conf)).To(BeEmpty())
		})

		It("should pass with valid toleration options", func() {
			conf.NodeToleration = &config.NodeTolerationConfiguration{
				DefaultNotReadyTolerationSeconds:    pointer.Int64(60),
				DefaultUnreachableTolerationSeconds: pointer.Int64(120),
			}

			Expect(ValidateOperatorConfiguration(conf)).To(BeEmpty())
		})

		It("should fail with invalid toleration options", func() {
			conf.NodeToleration = &config.NodeTolerationConfiguration{
				DefaultNotReadyTolerationSeconds:    pointer.Int64(-1),
				DefaultUnreachableTolerationSeconds: pointer.Int64(-2),
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
