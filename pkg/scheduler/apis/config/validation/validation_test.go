// SPDX-FileCopyrightText: 2019 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	schedulerapi "github.com/gardener/gardener/pkg/scheduler/apis/config"
)

var _ = Describe("gardener-scheduler", func() {
	Describe("#ValidateConfiguration", func() {
		var defaultAdmissionConfiguration = schedulerapi.SchedulerConfiguration{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "scheduler.config.gardener.cloud/v1alpha1",
				Kind:       "SchedulerConfiguration",
			},
			Schedulers: schedulerapi.SchedulerControllerConfiguration{
				Shoot: &schedulerapi.ShootSchedulerConfiguration{
					Strategy: schedulerapi.SameRegion,
				},
			},
		}

		Context("Validate Admission Plugin SchedulerConfiguration", func() {
			It("should pass because the Gardener Scheduler Configuration with the 'Same Region' Strategy is a valid configuration", func() {
				sameRegionConfiguration := defaultAdmissionConfiguration
				sameRegionConfiguration.Schedulers.Shoot.Strategy = schedulerapi.SameRegion
				err := ValidateConfiguration(&sameRegionConfiguration)

				Expect(err).ToNot(HaveOccurred())
			})

			It("should pass because the Gardener Scheduler Configuration with the 'Minimal Distance' Strategy is a valid configuration", func() {
				minimalDistanceConfiguration := defaultAdmissionConfiguration
				minimalDistanceConfiguration.Schedulers.Shoot.Strategy = schedulerapi.MinimalDistance
				err := ValidateConfiguration(&minimalDistanceConfiguration)

				Expect(err).ToNot(HaveOccurred())
			})

			It("should pass because the Gardener Scheduler Configuration with the default Strategy is a valid configuration", func() {
				err := ValidateConfiguration(&defaultAdmissionConfiguration)
				Expect(err).ToNot(HaveOccurred())
			})

			It("should fail because the Gardener Scheduler Configuration is invalid", func() {
				invalidConfiguration := defaultAdmissionConfiguration
				invalidConfiguration.Schedulers.Shoot.Strategy = "invalidStrategy"
				err := ValidateConfiguration(&invalidConfiguration)

				Expect(err).To(HaveOccurred())
			})
		})
	})
})
