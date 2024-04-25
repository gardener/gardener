// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"

	schedulerconfig "github.com/gardener/gardener/pkg/scheduler/apis/config"
)

var _ = Describe("gardener-scheduler", func() {
	Describe("#ValidateConfiguration", func() {
		var defaultAdmissionConfiguration schedulerconfig.SchedulerConfiguration

		BeforeEach(func() {
			defaultAdmissionConfiguration = schedulerconfig.SchedulerConfiguration{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "scheduler.config.gardener.cloud/v1alpha1",
					Kind:       "SchedulerConfiguration",
				},
				Schedulers: schedulerconfig.SchedulerControllerConfiguration{
					BackupBucket: &schedulerconfig.BackupBucketSchedulerConfiguration{
						ConcurrentSyncs: 2,
					},
					Shoot: &schedulerconfig.ShootSchedulerConfiguration{
						ConcurrentSyncs: 2,
						Strategy:        schedulerconfig.SameRegion,
					},
				},
			}
		})

		Context("Validate Admission Plugin SchedulerConfiguration", func() {
			It("should pass because the Gardener Scheduler Configuration with the 'Same Region' Strategy is a valid configuration", func() {
				sameRegionConfiguration := defaultAdmissionConfiguration
				sameRegionConfiguration.Schedulers.Shoot.Strategy = schedulerconfig.SameRegion
				err := ValidateConfiguration(&sameRegionConfiguration)

				Expect(err).To(BeEmpty())
			})

			It("should pass because the Gardener Scheduler Configuration with the 'Minimal Distance' Strategy is a valid configuration", func() {
				minimalDistanceConfiguration := defaultAdmissionConfiguration
				minimalDistanceConfiguration.Schedulers.Shoot.Strategy = schedulerconfig.MinimalDistance
				err := ValidateConfiguration(&minimalDistanceConfiguration)

				Expect(err).To(BeEmpty())
			})

			It("should pass because the Gardener Scheduler Configuration with the default Strategy is a valid configuration", func() {
				err := ValidateConfiguration(&defaultAdmissionConfiguration)
				Expect(err).To(BeEmpty())
			})

			It("should fail because the Gardener Scheduler Configuration contains an invalid strategy", func() {
				invalidConfiguration := defaultAdmissionConfiguration
				invalidConfiguration.Schedulers.Shoot.Strategy = "invalidStrategy"
				err := ValidateConfiguration(&invalidConfiguration)

				Expect(err).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeNotSupported),
					"Field": Equal("schedulers.shoot.strategy"),
				}))))
			})

			It("should fail because backupBucket concurrentSyncs are negative", func() {
				invalidConfiguration := defaultAdmissionConfiguration
				invalidConfiguration.Schedulers.BackupBucket.ConcurrentSyncs = -1

				err := ValidateConfiguration(&invalidConfiguration)

				Expect(err).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("schedulers.backupBucket.concurrentSyncs"),
				}))))
			})

			It("should fail because shoot concurrentSyncs are negative", func() {
				invalidConfiguration := defaultAdmissionConfiguration
				invalidConfiguration.Schedulers.Shoot.ConcurrentSyncs = -1

				err := ValidateConfiguration(&invalidConfiguration)

				Expect(err).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("schedulers.shoot.concurrentSyncs"),
				}))))
			})
		})
	})
})
