// Copyright 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
