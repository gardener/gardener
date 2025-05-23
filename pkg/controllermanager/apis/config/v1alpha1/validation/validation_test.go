// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	componentbaseconfigv1alpha1 "k8s.io/component-base/config/v1alpha1"
	"k8s.io/utils/ptr"

	controllermanagerconfigv1alpha1 "github.com/gardener/gardener/pkg/controllermanager/apis/config/v1alpha1"
	. "github.com/gardener/gardener/pkg/controllermanager/apis/config/v1alpha1/validation"
)

var _ = Describe("#ValidateControllerManagerConfiguration", func() {
	var conf *controllermanagerconfigv1alpha1.ControllerManagerConfiguration

	BeforeEach(func() {
		conf = &controllermanagerconfigv1alpha1.ControllerManagerConfiguration{
			Controllers: controllermanagerconfigv1alpha1.ControllerManagerControllerConfiguration{},
		}
	})

	Context("client connection configuration", func() {
		var (
			clientConnection *componentbaseconfigv1alpha1.ClientConnectionConfiguration
			fldPath          *field.Path
		)

		BeforeEach(func() {
			controllermanagerconfigv1alpha1.SetObjectDefaults_ControllerManagerConfiguration(conf)

			clientConnection = &conf.GardenClientConnection
			fldPath = field.NewPath("gardenClientConnection")
		})

		It("should allow default client connection configuration", func() {
			Expect(ValidateControllerManagerConfiguration(conf)).To(BeEmpty())
		})

		It("should return errors because some values are invalid", func() {
			clientConnection.Burst = -1

			Expect(ValidateControllerManagerConfiguration(conf)).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fldPath.Child("burst").String()),
				})),
			))
		})
	})

	Context("leader election configuration", func() {
		BeforeEach(func() {
			controllermanagerconfigv1alpha1.SetObjectDefaults_ControllerManagerConfiguration(conf)
		})

		It("should allow omitting leader election config", func() {
			conf.LeaderElection = nil

			Expect(ValidateControllerManagerConfiguration(conf)).To(BeEmpty())
		})

		It("should allow not enabling leader election", func() {
			conf.LeaderElection.LeaderElect = nil

			Expect(ValidateControllerManagerConfiguration(conf)).To(BeEmpty())
		})

		It("should allow disabling leader election", func() {
			conf.LeaderElection.LeaderElect = ptr.To(false)

			Expect(ValidateControllerManagerConfiguration(conf)).To(BeEmpty())
		})

		It("should allow default leader election configuration with required fields", func() {
			Expect(ValidateControllerManagerConfiguration(conf)).To(BeEmpty())
		})

		It("should reject leader election config with missing required fields", func() {
			conf.LeaderElection.ResourceNamespace = ""

			Expect(ValidateControllerManagerConfiguration(conf)).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("leaderElection.resourceNamespace"),
				})),
			))
		})
	})

	Context("ProjectControllerConfiguration", func() {
		Context("ProjectQuotaConfiguration", func() {
			BeforeEach(func() {
				conf.Controllers.Project = &controllermanagerconfigv1alpha1.ProjectControllerConfiguration{}
			})

			It("should pass because no quota configuration is specified", func() {
				errorList := ValidateControllerManagerConfiguration(conf)
				Expect(errorList).To(BeEmpty())
			})
			It("should pass because quota configuration has correct label selector", func() {
				conf.Controllers.Project.Quotas = []controllermanagerconfigv1alpha1.QuotaConfiguration{
					{
						ProjectSelector: &metav1.LabelSelector{
							MatchExpressions: []metav1.LabelSelectorRequirement{
								{Key: "role", Operator: "In", Values: []string{"user"}},
							},
						},
						Config: corev1.ResourceQuota{},
					},
				}
				errorList := ValidateControllerManagerConfiguration(conf)
				Expect(errorList).To(BeEmpty())
			})
			It("should fail because quota configuration contains invalid label selector", func() {
				conf.Controllers.Project.Quotas = []controllermanagerconfigv1alpha1.QuotaConfiguration{
					{
						ProjectSelector: &metav1.LabelSelector{
							MatchExpressions: []metav1.LabelSelectorRequirement{
								{Key: "role", Operator: "In", Values: []string{"user"}},
							},
						},
						Config: corev1.ResourceQuota{},
					},
					{
						ProjectSelector: &metav1.LabelSelector{
							MatchExpressions: []metav1.LabelSelectorRequirement{
								{},
							},
						},
						Config: corev1.ResourceQuota{},
					},
				}
				errorList := ValidateControllerManagerConfiguration(conf)
				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("controllers.project.quotas[1].projectSelector.matchExpressions[0].operator"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("controllers.project.quotas[1].projectSelector.matchExpressions[0].key"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("controllers.project.quotas[1].projectSelector.matchExpressions[0].key"),
					})),
				))
			})
		})
	})

	Context("ShootStateControllerConfiguration", func() {
		Context("ConcurrentSyncs", func() {
			var (
				concurrentSyncs = 0
			)

			BeforeEach(func() {
				conf.Controllers.ShootState = &controllermanagerconfigv1alpha1.ShootStateControllerConfiguration{}
			})

			It("should not allow negative values", func() {
				concurrentSyncs = -1
				conf.Controllers.ShootState.ConcurrentSyncs = &concurrentSyncs
				errorList := ValidateControllerManagerConfiguration(conf)
				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("controllers.shootState.concurrentSyncs"),
					}))))
			})

			It("should allow 0 as a value", func() {
				concurrentSyncs = 0
				conf.Controllers.ShootState.ConcurrentSyncs = &concurrentSyncs
				errorList := ValidateControllerManagerConfiguration(conf)
				Expect(errorList).To(BeEmpty())
			})

			It("should allow positive values", func() {
				concurrentSyncs = 1
				conf.Controllers.ShootState.ConcurrentSyncs = &concurrentSyncs
				errorList := ValidateControllerManagerConfiguration(conf)
				Expect(errorList).To(BeEmpty())
			})
		})
	})
})
