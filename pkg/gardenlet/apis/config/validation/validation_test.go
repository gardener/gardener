// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	. "github.com/gardener/gardener/pkg/gardenlet/apis/config/validation"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/pointer"
)

var _ = Describe("GardenletConfiguration", func() {
	var (
		cfg *config.GardenletConfiguration

		concurrentSyncs = 20
	)

	BeforeEach(func() {
		cfg = &config.GardenletConfiguration{
			Controllers: &config.GardenletControllerConfiguration{
				Shoot: &config.ShootControllerConfiguration{
					ConcurrentSyncs:      &concurrentSyncs,
					ProgressReportPeriod: &metav1.Duration{Duration: time.Hour},
					SyncPeriod:           &metav1.Duration{Duration: time.Hour},
					RetryDuration:        &metav1.Duration{Duration: time.Hour},
					DNSEntryTTLSeconds:   pointer.Int64Ptr(120),
				},
			},
			SeedConfig: &config.SeedConfig{},
			Server: &config.ServerConfiguration{
				HTTPS: config.HTTPSServer{
					Server: config.Server{
						BindAddress: "0.0.0.0",
						Port:        2720,
					},
				},
			},
			SNI: &config.SNI{
				Ingress: &config.SNIIngress{
					Namespace:   pointer.StringPtr("foo"),
					Labels:      map[string]string{"baz": "bar"},
					ServiceName: pointer.StringPtr("waldo"),
				},
			},
			Resources: &config.ResourcesConfiguration{
				Capacity: corev1.ResourceList{
					"foo": resource.MustParse("42"),
					"bar": resource.MustParse("13"),
				},
				Reserved: corev1.ResourceList{
					"foo": resource.MustParse("7"),
				},
			},
		}
	})

	Describe("#ValidGardenletConfiguration", func() {
		It("should allow valid configurations", func() {
			errorList := ValidateGardenletConfiguration(cfg)

			Expect(errorList).To(BeEmpty())
		})

		Context("shoot controller", func() {
			It("should forbid invalid configuration", func() {
				invalidConcurrentSyncs := -1

				cfg.Controllers.Shoot.ConcurrentSyncs = &invalidConcurrentSyncs
				cfg.Controllers.Shoot.ProgressReportPeriod = &metav1.Duration{Duration: -1}
				cfg.Controllers.Shoot.SyncPeriod = &metav1.Duration{Duration: -1}
				cfg.Controllers.Shoot.RetryDuration = &metav1.Duration{Duration: -1}

				errorList := ValidateGardenletConfiguration(cfg)

				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("controllers.shoot.concurrentSyncs"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("controllers.shoot.progressReporterPeriod"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("controllers.shoot.syncPeriod"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("controllers.shoot.retryDuration"),
					})),
				))
			})

			It("should forbid too low values for the DNS TTL", func() {
				cfg.Controllers.Shoot.DNSEntryTTLSeconds = pointer.Int64Ptr(-1)

				errorList := ValidateGardenletConfiguration(cfg)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("controllers.shoot.dnsEntryTTLSeconds"),
				}))))
			})

			It("should forbid too high values for the DNS TTL", func() {
				cfg.Controllers.Shoot.DNSEntryTTLSeconds = pointer.Int64Ptr(601)

				errorList := ValidateGardenletConfiguration(cfg)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("controllers.shoot.dnsEntryTTLSeconds"),
				}))))
			})
		})

		Context("seed selector/config", func() {
			It("should forbid specifying neither a seed selector nor a seed config", func() {
				cfg.SeedSelector = nil
				cfg.SeedConfig = nil

				errorList := ValidateGardenletConfiguration(cfg)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("seedSelector/seedConfig"),
				}))))
			})

			It("should forbid specifying both a seed selector and a seed config", func() {
				cfg.SeedSelector = &metav1.LabelSelector{}
				cfg.SeedConfig = &config.SeedConfig{}

				errorList := ValidateGardenletConfiguration(cfg)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("seedSelector/seedConfig"),
				}))))
			})
		})

		Context("server", func() {
			It("should forbid not specifying a server configuration", func() {
				cfg.Server = nil

				errorList := ValidateGardenletConfiguration(cfg)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("server"),
				}))))
			})

			It("should forbid invalid server configuration", func() {
				cfg.Server = &config.ServerConfiguration{}

				errorList := ValidateGardenletConfiguration(cfg)

				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal("server.https.bindAddress"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal("server.https.port"),
					})),
				))
			})
		})

		It("should forbid not specifying a sni configuration", func() {
			cfg.SNI = nil

			errorList := ValidateGardenletConfiguration(cfg)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("sni"),
			}))))
		})

		It("should forbid not specifying a sni ingress configuration", func() {
			cfg.SNI.Ingress = nil

			errorList := ValidateGardenletConfiguration(cfg)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("sni.ingress"),
			}))))
		})

		It("should forbid not specifying a sni ingress namespace configuration", func() {
			cfg.SNI.Ingress.Namespace = pointer.StringPtr("")

			errorList := ValidateGardenletConfiguration(cfg)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("sni.ingress.namespace"),
			}))))
		})

		It("should forbid not specifying a sni ingress service name configuration", func() {
			cfg.SNI.Ingress.ServiceName = pointer.StringPtr("")

			errorList := ValidateGardenletConfiguration(cfg)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("sni.ingress.serviceName"),
			}))))
		})

		It("should forbid not specifying a sni ingress labels configuration", func() {
			cfg.SNI.Ingress.Labels = nil

			errorList := ValidateGardenletConfiguration(cfg)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("sni.ingress.labels"),
			}))))
		})

		It("should forbid reserved greater than capacity", func() {
			cfg.Resources = &config.ResourcesConfiguration{
				Capacity: corev1.ResourceList{
					"foo": resource.MustParse("42"),
				},
				Reserved: corev1.ResourceList{
					"foo": resource.MustParse("43"),
				},
			}

			errorList := ValidateGardenletConfiguration(cfg)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("resources.reserved.foo"),
			}))))
		})

		It("should forbid reserved without capacity", func() {
			cfg.Resources = &config.ResourcesConfiguration{
				Reserved: corev1.ResourceList{
					"foo": resource.MustParse("42"),
				},
			}

			errorList := ValidateGardenletConfiguration(cfg)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("resources.reserved.foo"),
			}))))
		})
	})
})
