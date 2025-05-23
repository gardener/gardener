// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package v1alpha1_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/gardener/gardener/pkg/logger"
	. "github.com/gardener/gardener/pkg/nodeagent/apis/config/v1alpha1"
)

var _ = Describe("Defaults", func() {
	Describe("NodeAgentConfiguration", func() {
		var obj *NodeAgentConfiguration

		BeforeEach(func() {
			obj = &NodeAgentConfiguration{}
		})

		It("should correctly default the configuration", func() {
			SetObjectDefaults_NodeAgentConfiguration(obj)

			Expect(obj.LogLevel).To(Equal(logger.InfoLevel))
			Expect(obj.LogFormat).To(Equal(logger.FormatJSON))
		})

		It("should not overwrite custom settings", func() {
			var (
				expectedLogLevel  = "foo"
				expectedLogFormat = "bar"
			)

			obj.LogLevel = expectedLogLevel
			obj.LogFormat = expectedLogFormat

			SetObjectDefaults_NodeAgentConfiguration(obj)

			Expect(obj.LogLevel).To(Equal(expectedLogLevel))
			Expect(obj.LogFormat).To(Equal(expectedLogFormat))
		})

		Describe("Controller configuration", func() {
			Describe("Operating System Config controller", func() {
				It("should default the object", func() {
					obj := &OperatingSystemConfigControllerConfig{}

					SetDefaults_OperatingSystemConfigControllerConfig(obj)

					Expect(obj.SyncPeriod).To(PointTo(Equal(metav1.Duration{Duration: 10 * time.Minute})))
				})

				It("should not overwrite existing values", func() {
					obj := &OperatingSystemConfigControllerConfig{
						SyncPeriod: &metav1.Duration{Duration: time.Second},
					}

					SetDefaults_OperatingSystemConfigControllerConfig(obj)

					Expect(obj.SyncPeriod).To(PointTo(Equal(metav1.Duration{Duration: time.Second})))
				})
			})

			Describe("Token controller", func() {
				It("should default the object", func() {
					obj := &TokenControllerConfig{}

					SetDefaults_TokenControllerConfig(obj)

					Expect(obj.SyncPeriod).To(PointTo(Equal(metav1.Duration{Duration: time.Hour})))
				})

				It("should not overwrite existing values", func() {
					obj := &TokenControllerConfig{
						SyncPeriod: &metav1.Duration{Duration: time.Second},
					}

					SetDefaults_TokenControllerConfig(obj)

					Expect(obj.SyncPeriod).To(PointTo(Equal(metav1.Duration{Duration: time.Second})))
				})
			})
		})

		Describe("Server configuration", func() {
			It("should default the object", func() {
				obj := &ServerConfiguration{}

				SetDefaults_ServerConfiguration(obj)

				Expect(obj.HealthProbes.BindAddress).To(BeEmpty())
				Expect(obj.HealthProbes.Port).To(Equal(2751))
				Expect(obj.Metrics.BindAddress).To(BeEmpty())
				Expect(obj.Metrics.Port).To(Equal(2752))
			})

			It("should not overwrite existing values", func() {
				obj := &ServerConfiguration{
					HealthProbes: &Server{BindAddress: "1", Port: 2345},
					Metrics:      &Server{BindAddress: "6", Port: 7890},
				}

				SetDefaults_ServerConfiguration(obj)

				Expect(obj.HealthProbes.BindAddress).To(Equal("1"))
				Expect(obj.HealthProbes.Port).To(Equal(2345))
				Expect(obj.Metrics.BindAddress).To(Equal("6"))
				Expect(obj.Metrics.Port).To(Equal(7890))
			})
		})
	})
})
