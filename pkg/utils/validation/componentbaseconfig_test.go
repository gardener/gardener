// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"k8s.io/apimachinery/pkg/util/validation/field"
	componentbaseconfigv1alpha1 "k8s.io/component-base/config/v1alpha1"
	"k8s.io/utils/ptr"

	. "github.com/gardener/gardener/pkg/utils/validation"
)

var _ = Describe("componentbaseconfig validation helpers", func() {
	Describe("#ValidateClientConnectionConfiguration", func() {
		var (
			fldPath *field.Path
			config  *componentbaseconfigv1alpha1.ClientConnectionConfiguration
		)

		BeforeEach(func() {
			fldPath = field.NewPath("clientConnection")

			config = &componentbaseconfigv1alpha1.ClientConnectionConfiguration{}
		})

		It("should ignore a nil config", func() {
			Expect(ValidateClientConnectionConfiguration(nil, fldPath)).To(BeEmpty())
		})

		It("should allow an empty config", func() {
			Expect(ValidateClientConnectionConfiguration(config, fldPath)).To(BeEmpty())
		})

		It("should allow default configuration", func() {
			componentbaseconfigv1alpha1.RecommendedDefaultClientConnectionConfiguration(config)

			Expect(ValidateClientConnectionConfiguration(config, fldPath)).To(BeEmpty())
		})

		It("should reject invalid fields", func() {
			config.Burst = -1

			Expect(ValidateClientConnectionConfiguration(config, fldPath)).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fldPath.Child("burst").String()),
				})),
			))
		})
	})

	Describe("#ValidateLeaderElectionConfiguration", func() {
		var (
			fldPath *field.Path
			config  *componentbaseconfigv1alpha1.LeaderElectionConfiguration
		)

		BeforeEach(func() {
			fldPath = field.NewPath("leaderElection")

			config = &componentbaseconfigv1alpha1.LeaderElectionConfiguration{
				LeaderElect: ptr.To(true),
			}
		})

		It("should ignore a nil config", func() {
			Expect(ValidateLeaderElectionConfiguration(nil, fldPath)).To(BeEmpty())
		})

		It("should allow not enabling leader election", func() {
			config.LeaderElect = nil

			Expect(ValidateLeaderElectionConfiguration(nil, fldPath)).To(BeEmpty())
		})

		It("should allow disabling leader election", func() {
			config.LeaderElect = ptr.To(false)

			Expect(ValidateLeaderElectionConfiguration(config, fldPath)).To(BeEmpty())
		})

		It("should reject config with missing required fields", func() {
			componentbaseconfigv1alpha1.RecommendedDefaultLeaderElectionConfiguration(config)
			config.ResourceName = "foo"

			Expect(ValidateLeaderElectionConfiguration(config, fldPath)).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fldPath.Child("resourceNamespace").String()),
				})),
			))
		})

		It("should allow default configuration with required fields", func() {
			componentbaseconfigv1alpha1.RecommendedDefaultLeaderElectionConfiguration(config)
			config.ResourceName = "foo"
			config.ResourceNamespace = "bar"

			Expect(ValidateLeaderElectionConfiguration(config, fldPath)).To(BeEmpty())
		})

		It("should allow config with required fields", func() {
			config.ResourceName = "foo"
			config.ResourceNamespace = "bar"
			config.ResourceLock = "leases"
			config.LeaseDuration.Duration = 2 * time.Minute
			config.RenewDeadline.Duration = time.Minute
			config.RetryPeriod.Duration = time.Minute

			Expect(ValidateLeaderElectionConfiguration(config, fldPath)).To(BeEmpty())
		})

		It("should reject leader election config with missing required fields", func() {
			config.ResourceName = "foo"
			config.ResourceNamespace = "bar"
			config.ResourceLock = "leases"
			config.RenewDeadline.Duration = time.Minute
			config.RetryPeriod.Duration = time.Minute

			// invalid value, must be greater than renewDeadline
			config.LeaseDuration.Duration = time.Minute

			Expect(ValidateLeaderElectionConfiguration(config, fldPath)).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fldPath.Child("leaseDuration").String()),
				})),
			))
		})
	})
})
