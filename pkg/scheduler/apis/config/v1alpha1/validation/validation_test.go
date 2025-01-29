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
	componentbaseconfigv1alpha1 "k8s.io/component-base/config/v1alpha1"
	"k8s.io/utils/ptr"

	schedulerconfigv1alpha1 "github.com/gardener/gardener/pkg/scheduler/apis/config/v1alpha1"
)

var _ = Describe("#ValidateConfiguration", func() {
	var conf *schedulerconfigv1alpha1.SchedulerConfiguration

	BeforeEach(func() {
		conf = &schedulerconfigv1alpha1.SchedulerConfiguration{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "scheduler.config.gardener.cloud/v1alpha1",
				Kind:       "SchedulerConfiguration",
			},
			Schedulers: schedulerconfigv1alpha1.SchedulerControllerConfiguration{
				BackupBucket: &schedulerconfigv1alpha1.BackupBucketSchedulerConfiguration{
					ConcurrentSyncs: 2,
				},
				Shoot: &schedulerconfigv1alpha1.ShootSchedulerConfiguration{
					ConcurrentSyncs: 2,
					Strategy:        schedulerconfigv1alpha1.SameRegion,
				},
			},
		}
	})

	Context("client connection configuration", func() {
		var (
			clientConnection *componentbaseconfigv1alpha1.ClientConnectionConfiguration
			fldPath          *field.Path
		)

		BeforeEach(func() {
			schedulerconfigv1alpha1.SetObjectDefaults_SchedulerConfiguration(conf)

			clientConnection = &conf.ClientConnection
			fldPath = field.NewPath("clientConnection")
		})

		It("should allow default client connection configuration", func() {
			Expect(ValidateConfiguration(conf)).To(BeEmpty())
		})

		It("should return errors because some values are invalid", func() {
			clientConnection.Burst = -1

			Expect(ValidateConfiguration(conf)).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fldPath.Child("burst").String()),
				})),
			))
		})
	})

	Context("leader election configuration", func() {
		BeforeEach(func() {
			schedulerconfigv1alpha1.SetObjectDefaults_SchedulerConfiguration(conf)
		})

		It("should allow omitting leader election config", func() {
			conf.LeaderElection = nil

			Expect(ValidateConfiguration(conf)).To(BeEmpty())
		})

		It("should allow not enabling leader election", func() {
			conf.LeaderElection.LeaderElect = nil

			Expect(ValidateConfiguration(conf)).To(BeEmpty())
		})

		It("should allow disabling leader election", func() {
			conf.LeaderElection.LeaderElect = ptr.To(false)

			Expect(ValidateConfiguration(conf)).To(BeEmpty())
		})

		It("should allow default leader election configuration with required fields", func() {
			Expect(ValidateConfiguration(conf)).To(BeEmpty())
		})

		It("should reject leader election config with missing required fields", func() {
			conf.LeaderElection.ResourceNamespace = ""

			Expect(ValidateConfiguration(conf)).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("leaderElection.resourceNamespace"),
				})),
			))
		})
	})

	Context("scheduler controller configuration", func() {
		It("should pass because the Gardener Scheduler Configuration with the 'Same Region' Strategy is a valid configuration", func() {
			sameRegionConfiguration := conf.DeepCopy()
			sameRegionConfiguration.Schedulers.Shoot.Strategy = schedulerconfigv1alpha1.SameRegion
			err := ValidateConfiguration(sameRegionConfiguration)

			Expect(err).To(BeEmpty())
		})

		It("should pass because the Gardener Scheduler Configuration with the 'Minimal Distance' Strategy is a valid configuration", func() {
			minimalDistanceConfiguration := conf.DeepCopy()
			minimalDistanceConfiguration.Schedulers.Shoot.Strategy = schedulerconfigv1alpha1.MinimalDistance
			err := ValidateConfiguration(minimalDistanceConfiguration)

			Expect(err).To(BeEmpty())
		})

		It("should pass because the Gardener Scheduler Configuration with the default Strategy is a valid configuration", func() {
			err := ValidateConfiguration(conf)
			Expect(err).To(BeEmpty())
		})

		It("should fail because the Gardener Scheduler Configuration contains an invalid strategy", func() {
			invalidConfiguration := conf.DeepCopy()
			invalidConfiguration.Schedulers.Shoot.Strategy = "invalidStrategy"
			err := ValidateConfiguration(invalidConfiguration)

			Expect(err).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeNotSupported),
				"Field": Equal("schedulers.shoot.strategy"),
			}))))
		})

		It("should fail because backupBucket concurrentSyncs are negative", func() {
			invalidConfiguration := conf.DeepCopy()
			invalidConfiguration.Schedulers.BackupBucket.ConcurrentSyncs = -1

			err := ValidateConfiguration(invalidConfiguration)

			Expect(err).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("schedulers.backupBucket.concurrentSyncs"),
			}))))
		})

		It("should fail because shoot concurrentSyncs are negative", func() {
			invalidConfiguration := conf.DeepCopy()
			invalidConfiguration.Schedulers.Shoot.ConcurrentSyncs = -1

			err := ValidateConfiguration(invalidConfiguration)

			Expect(err).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("schedulers.shoot.concurrentSyncs"),
			}))))
		})
	})
})
