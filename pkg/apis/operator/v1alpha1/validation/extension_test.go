// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	gomegatypes "github.com/onsi/gomega/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
)

var _ = Describe("Validation Tests", func() {
	Describe("#ValidateExtensionUpdate", func() {
		var (
			extension *operatorv1alpha1.Extension

			test = func(oldPrimary, newPrimary *bool, matcher gomegatypes.GomegaMatcher) {
				GinkgoHelper()

				newExtension := extension.DeepCopy()
				extension.Spec.Resources[0].Primary = oldPrimary
				newExtension.Spec.Resources[0].Primary = newPrimary

				Expect(ValidateExtensionUpdate(extension, newExtension)).To(matcher)
			}
		)

		BeforeEach(func() {
			extension = &operatorv1alpha1.Extension{
				Spec: operatorv1alpha1.ExtensionSpec{
					Resources: []gardencorev1beta1.ControllerResource{
						{Kind: "kind-a", Type: "type-a"},
					},
				},
			}
		})

		It("should return no errors when field is unchanged", func() {
			test(ptr.To(true), ptr.To(true), BeEmpty())
		})

		It("should return no errors when field is set from nil to true", func() {
			test(nil, ptr.To(true), BeEmpty())
		})

		It("should return an error because the primary field is changed to false", func() {
			test(ptr.To(true), ptr.To(false), ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("spec.resources[0].primary"),
			}))))
		})

		It("should return an error because the primary field is changed to nil", func() {
			test(ptr.To(false), nil, ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("spec.resources[0].primary"),
			}))))
		})

		It("should return an error because the primary field is changed to true", func() {
			test(ptr.To(false), ptr.To(true), ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("spec.resources[0].primary"),
			}))))
		})
	})
})
