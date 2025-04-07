// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
)

var _ = Describe("Validation Tests", func() {
	Describe("#ValidateExtensionUpdate", func() {
		var extension *operatorv1alpha1.Extension

		BeforeEach(func() {
			extension = &operatorv1alpha1.Extension{
				Spec: operatorv1alpha1.ExtensionSpec{
					Resources: []gardencorev1beta1.ControllerResource{
						{Kind: "kind-a", Type: "type-a", Primary: ptr.To(true)},
					},
				},
			}
		})

		It("should return no errors because the config is valid", func() {
			new := extension.DeepCopy()

			Expect(ValidateExtensionUpdate(extension, new)).To(BeEmpty())
		})

		It("should return no errors when resource is added", func() {
			new := extension.DeepCopy()
			new.Spec.Resources = append(new.Spec.Resources, gardencorev1beta1.ControllerResource{
				Kind: "kind-b",
				Type: "type-b",
			})

			Expect(ValidateExtensionUpdate(extension, new)).To(BeEmpty())
		})

		It("should return no errors when field is set from nil to true", func() {
			new := extension.DeepCopy()
			extension.Spec.Resources[0].Primary = nil

			Expect(ValidateExtensionUpdate(extension, new)).To(BeEmpty())
		})

		It("should return an error because the primary field is changed to false", func() {
			new := extension.DeepCopy()
			new.Spec.Resources[0].Primary = ptr.To(false)

			Expect(ValidateExtensionUpdate(extension, new)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("spec.resources[0].primary"),
			}))))
		})

		It("should return an error because the primary field is changed to nil", func() {
			extension.Spec.Resources[0].Primary = ptr.To(false)
			new := extension.DeepCopy()
			new.Spec.Resources[0].Primary = nil

			Expect(ValidateExtensionUpdate(extension, new)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("spec.resources[0].primary"),
			}))))
		})

		It("should return an error because the primary field is changed to true", func() {
			extension.Spec.Resources[0].Primary = ptr.To(false)
			new := extension.DeepCopy()
			new.Spec.Resources[0].Primary = ptr.To(true)

			Expect(ValidateExtensionUpdate(extension, new)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("spec.resources[0].primary"),
			}))))
		})
	})
})
