// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"

	resourcemanagerconfigv1alpha1 "github.com/gardener/gardener/pkg/resourcemanager/apis/config/v1alpha1"
	. "github.com/gardener/gardener/pkg/resourcemanager/apis/config/v1alpha1/validation"
)

var _ = Describe("Validation", func() {
	Describe("#ValidateResourceManagerConfiguration", func() {
		var conf *resourcemanagerconfigv1alpha1.ResourceManagerConfiguration

		BeforeEach(func() {
			conf = &resourcemanagerconfigv1alpha1.ResourceManagerConfiguration{
				LogLevel:  "info",
				LogFormat: "text",
				Server: resourcemanagerconfigv1alpha1.ServerConfiguration{
					HealthProbes: &resourcemanagerconfigv1alpha1.Server{
						Port: 1234,
					},
					Metrics: &resourcemanagerconfigv1alpha1.Server{
						Port: 5678,
					},
				},
				Controllers: resourcemanagerconfigv1alpha1.ResourceManagerControllerConfiguration{
					ClusterID:     ptr.To(""),
					ResourceClass: ptr.To("foo"),
					Health: resourcemanagerconfigv1alpha1.HealthControllerConfig{
						ConcurrentSyncs: ptr.To(5),
						SyncPeriod:      &metav1.Duration{Duration: time.Minute},
					},
					ManagedResource: resourcemanagerconfigv1alpha1.ManagedResourceControllerConfig{
						ConcurrentSyncs:     ptr.To(5),
						SyncPeriod:          &metav1.Duration{Duration: time.Minute},
						ManagedByLabelValue: ptr.To("foo"),
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
				conf.TargetClientConnection = &resourcemanagerconfigv1alpha1.ClientConnection{}
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
					conf.Controllers.CSRApprover.Enabled = true
					conf.Controllers.CSRApprover.ConcurrentSyncs = ptr.To(0)

					Expect(ValidateResourceManagerConfiguration(conf)).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeInvalid),
							"Field": Equal("controllers.csrApprover.concurrentSyncs"),
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
					conf.Controllers.Health.ConcurrentSyncs = ptr.To(0)
					conf.Controllers.Health.SyncPeriod = &metav1.Duration{Duration: time.Hour}

					Expect(ValidateResourceManagerConfiguration(conf)).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeInvalid),
							"Field": Equal("controllers.health.concurrentSyncs"),
						})),
					))
				})

				It("should return errors because sync period is nil", func() {
					conf.Controllers.Health.ConcurrentSyncs = ptr.To(5)
					conf.Controllers.Health.SyncPeriod = nil

					Expect(ValidateResourceManagerConfiguration(conf)).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeInvalid),
							"Field": Equal("controllers.health.syncPeriod"),
						})),
					))
				})

				It("should return errors because sync period is < 15s", func() {
					conf.Controllers.Health.ConcurrentSyncs = ptr.To(5)
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
					conf.Controllers.ManagedResource.ConcurrentSyncs = ptr.To(0)
					conf.Controllers.ManagedResource.SyncPeriod = &metav1.Duration{Duration: time.Hour}

					Expect(ValidateResourceManagerConfiguration(conf)).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeInvalid),
							"Field": Equal("controllers.managedResources.concurrentSyncs"),
						})),
					))
				})

				It("should return errors because sync period is nil", func() {
					conf.Controllers.ManagedResource.ConcurrentSyncs = ptr.To(5)
					conf.Controllers.ManagedResource.SyncPeriod = nil

					Expect(ValidateResourceManagerConfiguration(conf)).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeInvalid),
							"Field": Equal("controllers.managedResources.syncPeriod"),
						})),
					))
				})

				It("should return errors because sync period is < 15s", func() {
					conf.Controllers.ManagedResource.ConcurrentSyncs = ptr.To(5)
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
					conf.Controllers.ManagedResource.ManagedByLabelValue = ptr.To("")

					Expect(ValidateResourceManagerConfiguration(conf)).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeRequired),
							"Field": Equal("controllers.managedResources.managedByLabelValue"),
						})),
					))
				})
			})

			Context("node agent reconciliation delay", func() {
				BeforeEach(func() {
					conf.Controllers.NodeAgentReconciliationDelay.Enabled = true
				})

				It("should return errors because delays are <= 0", func() {
					conf.Controllers.NodeAgentReconciliationDelay.MinDelay = &metav1.Duration{Duration: -1}
					conf.Controllers.NodeAgentReconciliationDelay.MaxDelay = &metav1.Duration{Duration: -1}

					Expect(ValidateResourceManagerConfiguration(conf)).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":   Equal(field.ErrorTypeInvalid),
							"Field":  Equal("controllers.nodeAgentReconciliationDelay.minDelay"),
							"Detail": ContainSubstring("must be non-negative"),
						})),
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":   Equal(field.ErrorTypeInvalid),
							"Field":  Equal("controllers.nodeAgentReconciliationDelay.maxDelay"),
							"Detail": ContainSubstring("must be non-negative"),
						})),
					))
				})

				It("should return an error because min delay > max delay", func() {
					conf.Controllers.NodeAgentReconciliationDelay.MinDelay = &metav1.Duration{Duration: 4}
					conf.Controllers.NodeAgentReconciliationDelay.MaxDelay = &metav1.Duration{Duration: 3}

					Expect(ValidateResourceManagerConfiguration(conf)).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":   Equal(field.ErrorTypeInvalid),
							"Field":  Equal("controllers.nodeAgentReconciliationDelay.minDelay"),
							"Detail": ContainSubstring("minimum delay must not be higher than maximum delay"),
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
					conf.Webhooks.PodSchedulerName.SchedulerName = ptr.To("")

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
					conf.Webhooks.ProjectedTokenMount.ExpirationSeconds = ptr.To[int64](123)

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
					conf.Webhooks.HighAvailabilityConfig.DefaultNotReadyTolerationSeconds = ptr.To[int64](60)
					conf.Webhooks.HighAvailabilityConfig.DefaultUnreachableTolerationSeconds = ptr.To[int64](120)

					Expect(ValidateResourceManagerConfiguration(conf)).To(BeEmpty())
				})

				It("should fail with invalid toleration options", func() {
					conf.Webhooks.HighAvailabilityConfig.DefaultNotReadyTolerationSeconds = ptr.To(int64(-1))
					conf.Webhooks.HighAvailabilityConfig.DefaultUnreachableTolerationSeconds = ptr.To(int64(-2))

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

			Context("node agent authorizer", func() {
				It("should succeed with a valid machine namespace", func() {
					conf.Webhooks.NodeAgentAuthorizer.Enabled = true
					conf.Webhooks.NodeAgentAuthorizer.MachineNamespace = "foo-namespace"

					Expect(ValidateResourceManagerConfiguration(conf)).To(BeEmpty())
				})

				It("should return errors when machine namespace is empty", func() {
					conf.Webhooks.NodeAgentAuthorizer.Enabled = true
					conf.Webhooks.NodeAgentAuthorizer.MachineNamespace = ""

					Expect(ValidateResourceManagerConfiguration(conf)).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeRequired),
							"Field": Equal("webhooks.nodeAgentAuthorizer.machineNamespace"),
						})),
					))
				})
			})
		})
	})
})
