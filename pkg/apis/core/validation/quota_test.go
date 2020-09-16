// SPDX-FileCopyrightText: 2018 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation_test

import (
	"github.com/gardener/gardener/pkg/apis/core"
	. "github.com/gardener/gardener/pkg/apis/core/validation"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

var _ = Describe("Quota Validation Tests ", func() {
	Describe("#ValidateQuota", func() {
		var quota *core.Quota

		BeforeEach(func() {
			quota = &core.Quota{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "quota-1",
					Namespace: "my-namespace",
				},
				Spec: core.QuotaSpec{
					Scope: corev1.ObjectReference{
						APIVersion: "v1",
						Kind:       "Secret",
					},
					Metrics: corev1.ResourceList{
						"cpu":    resource.MustParse("200"),
						"memory": resource.MustParse("4000Gi"),
					},
				},
			}
		})

		It("should not return any errors", func() {
			errorList := ValidateQuota(quota)

			Expect(errorList).To(HaveLen(0))
		})

		It("should forbid Quota specification with empty or invalid keys", func() {
			quota.ObjectMeta = metav1.ObjectMeta{}
			quota.Spec.Scope = corev1.ObjectReference{}
			quota.Spec.Metrics["key"] = resource.MustParse("-100")

			errorList := ValidateQuota(quota)

			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("metadata.name"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("metadata.namespace"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeNotSupported),
					"Field": Equal("spec.scope"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.metrics[key]"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.metrics[key]"),
				})),
			))
		})
	})
})
