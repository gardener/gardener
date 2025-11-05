// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	gomegatypes "github.com/onsi/gomega/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener/pkg/apis/core"
	. "github.com/gardener/gardener/pkg/apis/core/validation"
	"github.com/gardener/gardener/pkg/features"
	"github.com/gardener/gardener/pkg/utils/test"
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
		DescribeTable("ExposureClass metadata",
			func(objectMeta metav1.ObjectMeta, matcher gomegatypes.GomegaMatcher) {
				exposureClass.ObjectMeta = objectMeta

				errorList := ValidateExposureClass(exposureClass)

				Expect(errorList).To(matcher)
			},

			Entry("should forbid ExposureClass with empty metadata",
				metav1.ObjectMeta{},
				ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal("metadata.name"),
					})),
				),
			),
			Entry("should forbid ExposureClass with empty name",
				metav1.ObjectMeta{Name: ""},
				ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("metadata.name"),
				}))),
			),
			Entry("should forbid ExposureClass with '.' in the name (not a DNS-1123 label compliant name)",
				metav1.ObjectMeta{Name: "foo.test"},
				ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("metadata.name"),
				}))),
			),
			Entry("should forbid ExposureClass with '_' in the name (not a DNS-1123 subdomain)",
				metav1.ObjectMeta{Name: "foo_test"},
				ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("metadata.name"),
				}))),
			),
		)

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

		// TODO(georgibaltiev): rename this description back to "should fail as exposure class has an invalid seed selector" once the ForbidProviderTypesField feature gate has graduated.
		It("should fail as exposure class has invalid seed selector labels", func() {
			exposureClass.Scheduling.SeedSelector = &core.SeedSelector{
				LabelSelector: metav1.LabelSelector{MatchLabels: map[string]string{"foo": "no/slash/allowed"}},
			}
			errorList := ValidateExposureClass(exposureClass)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("scheduling.seedSelector.matchLabels"),
			}))))
		})

		// TODO(georgibaltiev): remove the providerTypes test cases once the ForbidProviderTypesField feature gate has graduated.
		It("should allow a non-empty providerTypes slice when the ForbidProviderTypesField feature gate is disabled", func() {
			DeferCleanup(test.WithFeatureGate(features.DefaultFeatureGate, features.ForbidProviderTypesField, false))

			exposureClass.Scheduling.SeedSelector.ProviderTypes = []string{"aws", "gcp"}
			errorList := ValidateExposureClass(exposureClass)

			Expect(errorList).To(BeEmpty())
		})

		It("should forbid a non-empty providerTypes slice when the ForbidProviderTypesField feature gate is enabled", func() {
			DeferCleanup(test.WithFeatureGate(features.DefaultFeatureGate, features.ForbidProviderTypesField, true))

			exposureClass.Scheduling.SeedSelector.ProviderTypes = []string{"aws", "gcp"}
			errorList := ValidateExposureClass(exposureClass)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(field.ErrorTypeForbidden),
				"Field":  Equal("scheduling.seedSelector.providerTypes"),
				"Detail": Equal("the 'seedSelector.providerTypes' field is no longer supported. Please use the 'seed.gardener.cloud/provider' and/or the 'seed.gardener.cloud/region' labels instead. "),
			}))))
		})

		It("should allow a nil providerTypes slice when the ForbidProviderTypesField feature gate is enabled", func() {
			DeferCleanup(test.WithFeatureGate(features.DefaultFeatureGate, features.ForbidProviderTypesField, true))

			exposureClass.Scheduling.SeedSelector.ProviderTypes = nil
			errorList := ValidateExposureClass(exposureClass)

			Expect(errorList).To(BeEmpty())
		})

		It("should forbid an empty providerTypes slice when the ForbidProviderTypesField feature gate is enabled", func() {
			DeferCleanup(test.WithFeatureGate(features.DefaultFeatureGate, features.ForbidProviderTypesField, true))

			exposureClass.Scheduling.SeedSelector.ProviderTypes = []string{}
			errorList := ValidateExposureClass(exposureClass)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(field.ErrorTypeForbidden),
				"Field":  Equal("scheduling.seedSelector.providerTypes"),
				"Detail": Equal("the 'seedSelector.providerTypes' field is no longer supported. Please use the 'seed.gardener.cloud/provider' and/or the 'seed.gardener.cloud/region' labels instead. "),
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
