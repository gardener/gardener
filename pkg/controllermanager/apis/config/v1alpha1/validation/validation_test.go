// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
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
						Config: &corev1.ResourceQuota{},
					},
				}
				errorList := ValidateControllerManagerConfiguration(conf)
				Expect(errorList).To(BeEmpty())
			})
			It("should fail because quota config is not specified", func() {
				conf.Controllers.Project.Quotas = []controllermanagerconfigv1alpha1.QuotaConfiguration{
					{
						ProjectSelector: &metav1.LabelSelector{
							MatchExpressions: []metav1.LabelSelectorRequirement{
								{Key: "role", Operator: "In", Values: []string{"user"}},
							},
						},
						Config: nil,
					},
				}
				errorList := ValidateControllerManagerConfiguration(conf)
				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal("controllers.project.quotas[0].config"),
					})),
				))
			})
			It("should fail because quota configuration contains invalid label selector", func() {
				conf.Controllers.Project.Quotas = []controllermanagerconfigv1alpha1.QuotaConfiguration{
					{
						ProjectSelector: &metav1.LabelSelector{
							MatchExpressions: []metav1.LabelSelectorRequirement{
								{Key: "role", Operator: "In", Values: []string{"user"}},
							},
						},
						Config: &corev1.ResourceQuota{},
					},
					{
						ProjectSelector: &metav1.LabelSelector{
							MatchExpressions: []metav1.LabelSelectorRequirement{
								{},
							},
						},
						Config: &corev1.ResourceQuota{},
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
})
