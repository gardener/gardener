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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/pointer"

	"github.com/gardener/gardener/pkg/resourcemanager/apis/config"
	. "github.com/gardener/gardener/pkg/resourcemanager/apis/config/validation"
)

var _ = Describe("Validation", func() {
	Describe("#ValidateResourceManagerConfiguration", func() {
		var conf *config.ResourceManagerConfiguration

		BeforeEach(func() {
			conf = &config.ResourceManagerConfiguration{
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
				Controllers: config.ResourceManagerControllerConfiguration{
					ClusterID:     pointer.String(""),
					ResourceClass: pointer.String("foo"),
					Health: config.HealthControllerConfig{
						ConcurrentSyncs: pointer.Int(5),
						SyncPeriod:      &metav1.Duration{Duration: time.Minute},
					},
					ManagedResource: config.ManagedResourceControllerConfig{
						ConcurrentSyncs:     pointer.Int(5),
						SyncPeriod:          &metav1.Duration{Duration: time.Minute},
						ManagedByLabelValue: pointer.String("foo"),
					},
					Secret: config.SecretControllerConfig{
						ConcurrentSyncs: pointer.Int(5),
					},
				},
			}
		})

		It("should return no errors because the config is valid", func() {
			Expect(ValidateResourceManagerConfiguration(conf)).To(BeEmpty())
		})

		Context("source client connection", func() {
			It("should return errors because some values are not satisfying", func() {
				conf.SourceClientConnection.CacheResyncPeriod = &metav1.Duration{Duration: time.Second}
				conf.SourceClientConnection.Burst = -1

				Expect(ValidateResourceManagerConfiguration(conf)).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("sourceClientConnection.cacheResyncPeriod"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("sourceClientConnection.burst"),
					})),
				))
			})
		})

		Context("target client connection", func() {
			It("should return errors because some values are not satisfying", func() {
				conf.TargetClientConnection = &config.ClientConnection{}
				conf.TargetClientConnection.CacheResyncPeriod = &metav1.Duration{Duration: time.Second}
				conf.TargetClientConnection.Burst = -1

				Expect(ValidateResourceManagerConfiguration(conf)).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("targetClientConnection.cacheResyncPeriod"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("targetClientConnection.burst"),
					})),
				))
			})
		})

		Context("logging config", func() {
			It("should return errors because log level is not supported", func() {
				conf.LogLevel = "foo"

				Expect(ValidateResourceManagerConfiguration(conf)).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeNotSupported),
						"Field": Equal("logLevel"),
					})),
				))
			})

			It("should return errors because log level is not supported", func() {
				conf.LogFormat = "bar"

				Expect(ValidateResourceManagerConfiguration(conf)).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeNotSupported),
						"Field": Equal("logFormat"),
					})),
				))
			})
		})

		Context("server configuration", func() {
			It("should return errors because configuration is nil", func() {
				conf.Server.HealthProbes = nil
				conf.Server.Metrics = nil

				Expect(ValidateResourceManagerConfiguration(conf)).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal("server.healthProbes"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal("server.metrics"),
					})),
				))
			})

			It("should return errors because ports are negative", func() {
				conf.Server.HealthProbes.Port = -1
				conf.Server.Metrics.Port = -2

				Expect(ValidateResourceManagerConfiguration(conf)).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("server.healthProbes.port"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("server.metrics.port"),
					})),
				))
			})
		})

		Context("controller configuration", func() {
			It("should return errors because cluster id is nil", func() {
				conf.Controllers.ClusterID = nil

				Expect(ValidateResourceManagerConfiguration(conf)).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal("controllers.clusterID"),
					})),
				))
			})

			It("should return errors because resource class is nil", func() {
				conf.Controllers.ResourceClass = nil

				Expect(ValidateResourceManagerConfiguration(conf)).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal("controllers.resourceClass"),
					})),
				))
			})

			Context("kubelet csr approver", func() {
				It("should return errors because concurrent syncs are <= 0", func() {
					conf.Controllers.KubeletCSRApprover.Enabled = true
					conf.Controllers.KubeletCSRApprover.ConcurrentSyncs = pointer.Int(0)

					Expect(ValidateResourceManagerConfiguration(conf)).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeInvalid),
							"Field": Equal("controllers.kubeletCSRApprover.concurrentSyncs"),
						})),
					))
				})
			})

			Context("garbage collector", func() {
				It("should return errors because sync period is nil", func() {
					conf.Controllers.GarbageCollector.Enabled = true

					Expect(ValidateResourceManagerConfiguration(conf)).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeInvalid),
							"Field": Equal("controllers.garbageCollector.syncPeriod"),
						})),
					))
				})

				It("should return errors because sync period is < 15s", func() {
					conf.Controllers.GarbageCollector.Enabled = true
					conf.Controllers.GarbageCollector.SyncPeriod = &metav1.Duration{Duration: time.Second}

					Expect(ValidateResourceManagerConfiguration(conf)).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeInvalid),
							"Field": Equal("controllers.garbageCollector.syncPeriod"),
						})),
					))
				})
			})

			Context("health", func() {
				It("should return errors because concurrent syncs are <= 0", func() {
					conf.Controllers.Health.ConcurrentSyncs = pointer.Int(0)
					conf.Controllers.Health.SyncPeriod = &metav1.Duration{Duration: time.Hour}

					Expect(ValidateResourceManagerConfiguration(conf)).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeInvalid),
							"Field": Equal("controllers.health.concurrentSyncs"),
						})),
					))
				})

				It("should return errors because sync period is nil", func() {
					conf.Controllers.Health.ConcurrentSyncs = pointer.Int(5)
					conf.Controllers.Health.SyncPeriod = nil

					Expect(ValidateResourceManagerConfiguration(conf)).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeInvalid),
							"Field": Equal("controllers.health.syncPeriod"),
						})),
					))
				})

				It("should return errors because sync period is < 15s", func() {
					conf.Controllers.Health.ConcurrentSyncs = pointer.Int(5)
					conf.Controllers.Health.SyncPeriod = &metav1.Duration{Duration: time.Second}

					Expect(ValidateResourceManagerConfiguration(conf)).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeInvalid),
							"Field": Equal("controllers.health.syncPeriod"),
						})),
					))
				})
			})

			Context("managed resources", func() {
				It("should return errors because concurrent syncs are <= 0", func() {
					conf.Controllers.ManagedResource.ConcurrentSyncs = pointer.Int(0)
					conf.Controllers.ManagedResource.SyncPeriod = &metav1.Duration{Duration: time.Hour}

					Expect(ValidateResourceManagerConfiguration(conf)).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeInvalid),
							"Field": Equal("controllers.managedResources.concurrentSyncs"),
						})),
					))
				})

				It("should return errors because sync period is nil", func() {
					conf.Controllers.ManagedResource.ConcurrentSyncs = pointer.Int(5)
					conf.Controllers.ManagedResource.SyncPeriod = nil

					Expect(ValidateResourceManagerConfiguration(conf)).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeInvalid),
							"Field": Equal("controllers.managedResources.syncPeriod"),
						})),
					))
				})

				It("should return errors because sync period is < 15s", func() {
					conf.Controllers.ManagedResource.ConcurrentSyncs = pointer.Int(5)
					conf.Controllers.ManagedResource.SyncPeriod = &metav1.Duration{Duration: time.Second}

					Expect(ValidateResourceManagerConfiguration(conf)).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeInvalid),
							"Field": Equal("controllers.managedResources.syncPeriod"),
						})),
					))
				})

				It("should return errors because managed by label value is nil", func() {
					conf.Controllers.ManagedResource.ManagedByLabelValue = nil

					Expect(ValidateResourceManagerConfiguration(conf)).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeRequired),
							"Field": Equal("controllers.managedResources.managedByLabelValue"),
						})),
					))
				})

				It("should return errors because managed by label value is empty", func() {
					conf.Controllers.ManagedResource.ManagedByLabelValue = pointer.String("")

					Expect(ValidateResourceManagerConfiguration(conf)).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeRequired),
							"Field": Equal("controllers.managedResources.managedByLabelValue"),
						})),
					))
				})
			})
		})

		Context("webhook configuration", func() {
			Context("pod scheduler name", func() {
				It("should return errors when scheduler name is nil", func() {
					conf.Webhooks.PodSchedulerName.Enabled = true

					Expect(ValidateResourceManagerConfiguration(conf)).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeRequired),
							"Field": Equal("webhooks.podSchedulerName.schedulerName"),
						})),
					))
				})

				It("should return errors when scheduler name is empty", func() {
					conf.Webhooks.PodSchedulerName.Enabled = true
					conf.Webhooks.PodSchedulerName.SchedulerName = pointer.String("")

					Expect(ValidateResourceManagerConfiguration(conf)).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeRequired),
							"Field": Equal("webhooks.podSchedulerName.schedulerName"),
						})),
					))
				})
			})

			Context("projected token mount", func() {
				It("should return errors when expiration seconds is nil", func() {
					conf.Webhooks.ProjectedTokenMount.Enabled = true

					Expect(ValidateResourceManagerConfiguration(conf)).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeInvalid),
							"Field": Equal("webhooks.projectedTokenMount.expirationSeconds"),
						})),
					))
				})

				It("should return errors when expiration seconds is lower than 600", func() {
					conf.Webhooks.ProjectedTokenMount.Enabled = true
					conf.Webhooks.ProjectedTokenMount.ExpirationSeconds = pointer.Int64(123)

					Expect(ValidateResourceManagerConfiguration(conf)).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeInvalid),
							"Field": Equal("webhooks.projectedTokenMount.expirationSeconds"),
						})),
					))
				})
			})

			Context("high availability config", func() {
				It("should succeed with valid toleration options", func() {
					conf.Webhooks.HighAvailabilityConfig.DefaultNotReadyTolerationSeconds = pointer.Int64(60)
					conf.Webhooks.HighAvailabilityConfig.DefaultUnreachableTolerationSeconds = pointer.Int64(120)

					Expect(ValidateResourceManagerConfiguration(conf)).To(BeEmpty())
				})

				It("should fail with invalid toleration options", func() {
					conf.Webhooks.HighAvailabilityConfig.DefaultNotReadyTolerationSeconds = pointer.Int64(-1)
					conf.Webhooks.HighAvailabilityConfig.DefaultUnreachableTolerationSeconds = pointer.Int64(-2)

					Expect(ValidateResourceManagerConfiguration(conf)).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeInvalid),
							"Field": Equal("webhooks.highAvailabilityConfig.defaultNotReadyTolerationSeconds"),
						})),
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeInvalid),
							"Field": Equal("webhooks.highAvailabilityConfig.defaultUnreachableTolerationSeconds"),
						})),
					))
				})
			})
		})
	})
})
