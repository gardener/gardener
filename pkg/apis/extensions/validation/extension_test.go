// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	. "github.com/gardener/gardener/pkg/apis/extensions/validation"
)

var _ = Describe("Extension validation tests", func() {
	var ext *extensionsv1alpha1.Extension

	BeforeEach(func() {
		ext = &extensionsv1alpha1.Extension{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-ext",
				Namespace: "test-namespace",
			},
			Spec: extensionsv1alpha1.ExtensionSpec{
				DefaultSpec: extensionsv1alpha1.DefaultSpec{
					Type:           "provider",
					ProviderConfig: &runtime.RawExtension{},
				},
			},
		}
	})

	Describe("#ValidExtension", func() {
		It("should forbid empty Extension resources", func() {
			errorList := ValidateExtension(&extensionsv1alpha1.Extension{})

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("metadata.name"),
			})), PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("metadata.namespace"),
			})), PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("spec.type"),
			}))))
		})

		It("should allow valid ext resources", func() {
			errorList := ValidateExtension(ext)

			Expect(errorList).To(BeEmpty())
		})
	})

	Describe("#ValidExtensionUpdate", func() {
		It("should prevent updating anything if deletion time stamp is set", func() {
			now := metav1.Now()
			ext.DeletionTimestamp = &now

			newExtension := prepareExtensionForUpdate(ext)
			newExtension.DeletionTimestamp = &now
			newExtension.Spec.ProviderConfig = nil

			errorList := ValidateExtensionUpdate(newExtension, ext)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(field.ErrorTypeForbidden),
				"Field":  Equal("spec"),
				"Detail": Equal("cannot update extension spec if deletion timestamp is set. Requested changes: DefaultSpec.ProviderConfig: <nil pointer> != runtime.RawExtension"),
			}))))
		})

		It("should prevent updating the type and region", func() {
			newExtension := prepareExtensionForUpdate(ext)
			newExtension.Spec.Type = "changed-type"

			errorList := ValidateExtensionUpdate(newExtension, ext)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("spec.type"),
			}))))
		})

		It("should allow updating the provider config", func() {
			newExtension := prepareExtensionForUpdate(ext)
			newExtension.Spec.ProviderConfig = nil

			errorList := ValidateExtensionUpdate(newExtension, ext)

			Expect(errorList).To(BeEmpty())
		})
	})
})

func prepareExtensionForUpdate(obj *extensionsv1alpha1.Extension) *extensionsv1alpha1.Extension {
	newObj := obj.DeepCopy()
	newObj.ResourceVersion = "1"
	return newObj
}
