// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener/pkg/apis/core"
	. "github.com/gardener/gardener/pkg/apis/core/validation"
)

var _ = Describe("ExposureClass Validation Tests ", func() {
	var (
		exposureClass          *core.ExposureClass
		defaultTestTolerations = []core.Toleration{
			{Key: "test", Value: ptr.To("foo")},
		}
	)

	BeforeEach(func() {
		exposureClass = makeDefaultExposureClass()
	})

	Describe("#ValidateExposureClass", func() {
		It("should pass as exposure class is valid", func() {
			errorList := ValidateExposureClass(exposureClass)
			Expect(errorList).To(BeEmpty())
		})

		It("should fail as exposure class handler is no DNS1123 label with zero length", func() {
			exposureClass.Handler = ""
			errorList := ValidateExposureClass(exposureClass)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("handler"),
			}))))
		})

		It("should fail as exposure class handler is no DNS1123 label", func() {
			exposureClass.Handler = "TES:T"
			errorList := ValidateExposureClass(exposureClass)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("handler"),
			}))))
		})

		It("should fail as exposure class handler contains more than 34 characters", func() {
			exposureClass.Handler = "izqissuczonxfeq346ce5exr9rhkcmb398t"
			errorList := ValidateExposureClass(exposureClass)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("handler"),
			}))))
		})

		It("should pass as exposure class handler contains less than 34 characters", func() {
			exposureClass.Handler = "izqissuczonxfeq346ce5exr9rhkcmb398"
			errorList := ValidateExposureClass(exposureClass)
			Expect(errorList).To(BeEmpty())
		})

		It("should fail as exposure class has an invalid seed selector", func() {
			exposureClass.Scheduling.SeedSelector = &core.SeedSelector{
				LabelSelector: metav1.LabelSelector{MatchLabels: map[string]string{"foo": "no/slash/allowed"}},
			}
			errorList := ValidateExposureClass(exposureClass)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("scheduling.seedSelector.matchLabels"),
			}))))
		})

		It("should fail as exposure class has invalid tolerations", func() {
			exposureClass.Scheduling.Tolerations = []core.Toleration{
				{},
				{Key: "foo"},
				{Key: "foo"},
				{Key: "bar", Value: ptr.To("baz")},
				{Key: "bar", Value: ptr.To("baz")},
			}
			errorList := ValidateExposureClass(exposureClass)

			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("scheduling.tolerations[0].key"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeDuplicate),
					"Field": Equal("scheduling.tolerations[2]"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeDuplicate),
					"Field": Equal("scheduling.tolerations[4]"),
				})),
			))
		})
	})

	Describe("#ValidateExposureClassUpdate", func() {
		var exposureClassNew *core.ExposureClass

		BeforeEach(func() {
			exposureClassNew = makeDefaultExposureClass()
		})

		It("should pass as exposure class is valid", func() {
			errorList := ValidateExposureClassUpdate(exposureClassNew, exposureClass)
			Expect(errorList).To(BeEmpty())
		})

		It("should fail as exposure class handlers are different", func() {
			exposureClassNew.Handler = "new-test-exposure-class-handler-name"
			errorList := ValidateExposureClassUpdate(exposureClassNew, exposureClass)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("handler"),
			}))))
		})

		It("should fail as exposure class scheduling fields are different", func() {
			exposureClassNew.Scheduling = &core.ExposureClassScheduling{
				Tolerations: defaultTestTolerations,
			}
			errorList := ValidateExposureClassUpdate(exposureClassNew, exposureClass)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("scheduling"),
			}))))
		})

	})
})

func makeDefaultExposureClass() *core.ExposureClass {
	return &core.ExposureClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
		},
		Handler: "test-exposure-class-handler-name",
		Scheduling: &core.ExposureClassScheduling{
			SeedSelector: &core.SeedSelector{
				LabelSelector: metav1.LabelSelector{
					MatchLabels: map[string]string{
						"test": "foo",
					},
				},
			},
		},
	}
}
