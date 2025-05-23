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
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	. "github.com/gardener/gardener/pkg/apis/extensions/validation"
)

var _ = Describe("ControlPlane validation tests", func() {
	var cp *extensionsv1alpha1.ControlPlane

	BeforeEach(func() {
		cp = &extensionsv1alpha1.ControlPlane{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-cp",
				Namespace: "test-namespace",
			},
			Spec: extensionsv1alpha1.ControlPlaneSpec{
				DefaultSpec: extensionsv1alpha1.DefaultSpec{
					Type:           "provider",
					ProviderConfig: &runtime.RawExtension{},
				},
				Region: "region",
				SecretRef: corev1.SecretReference{
					Name: "test",
				},
				InfrastructureProviderStatus: &runtime.RawExtension{},
			},
		}
	})

	Describe("#ValidControlPlane", func() {
		It("should forbid empty ControlPlane resources", func() {
			errorList := ValidateControlPlane(&extensionsv1alpha1.ControlPlane{})

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("metadata.name"),
			})), PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("metadata.namespace"),
			})), PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("spec.type"),
			})), PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("spec.region"),
			})), PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("spec.secretRef.name"),
			}))))
		})

		It("should forbid unsupported purpose values", func() {
			cpCopy := cp.DeepCopy()

			p := extensionsv1alpha1.Purpose("does-not-exist")
			cpCopy.Spec.Purpose = &p

			errorList := ValidateControlPlane(cpCopy)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeNotSupported),
				"Field": Equal("spec.purpose"),
			}))))
		})

		It("should allow valid cp resources", func() {
			errorList := ValidateControlPlane(cp)

			Expect(errorList).To(BeEmpty())
		})
	})

	Describe("#ValidControlPlaneUpdate", func() {
		It("should prevent updating anything if deletion time stamp is set", func() {
			now := metav1.Now()
			cp.DeletionTimestamp = &now

			newControlPlane := prepareControlPlaneForUpdate(cp)
			newControlPlane.DeletionTimestamp = &now
			newControlPlane.Spec.SecretRef.Name = "changed-secretref-name"

			errorList := ValidateControlPlaneUpdate(newControlPlane, cp)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(field.ErrorTypeForbidden),
				"Field":  Equal("spec"),
				"Detail": Equal("SecretRef.Name: changed-secretref-name != test"),
			}))))
		})

		It("should prevent updating the type, purpose or region", func() {
			newControlPlane := prepareControlPlaneForUpdate(cp)

			p := extensionsv1alpha1.Normal
			newControlPlane.Spec.Type = "changed-type"
			newControlPlane.Spec.Region = "changed-region"
			newControlPlane.Spec.Purpose = &p

			errorList := ValidateControlPlaneUpdate(newControlPlane, cp)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("spec.type"),
			})), PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("spec.region"),
			})), PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("spec.purpose"),
			}))))
		})

		It("should allow updating the name of the referenced secret, the provider config, or the infrastructure provider status", func() {
			newControlPlane := prepareControlPlaneForUpdate(cp)
			newControlPlane.Spec.SecretRef.Name = "changed-secretref-name"
			newControlPlane.Spec.ProviderConfig = nil
			newControlPlane.Spec.InfrastructureProviderStatus = nil

			errorList := ValidateControlPlaneUpdate(newControlPlane, cp)

			Expect(errorList).To(BeEmpty())
		})
	})
})

func prepareControlPlaneForUpdate(obj *extensionsv1alpha1.ControlPlane) *extensionsv1alpha1.ControlPlane {
	newObj := obj.DeepCopy()
	newObj.ResourceVersion = "1"
	return newObj
}
