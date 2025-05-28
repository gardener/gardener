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
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	. "github.com/gardener/gardener/pkg/apis/extensions/validation"
)

var _ = Describe("Infrastructure validation tests", func() {
	var infra *extensionsv1alpha1.Infrastructure

	BeforeEach(func() {
		infra = &extensionsv1alpha1.Infrastructure{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-infra",
				Namespace: "test-namespace",
			},
			Spec: extensionsv1alpha1.InfrastructureSpec{
				DefaultSpec: extensionsv1alpha1.DefaultSpec{
					Type:           "provider",
					ProviderConfig: &runtime.RawExtension{},
				},
				Region: "region",
				SecretRef: corev1.SecretReference{
					Name: "test",
				},
				SSHPublicKey: []byte("key"),
			},
		}
	})

	Describe("#ValidInfrastructure", func() {
		It("should forbid empty Infrastructure resources", func() {
			errorList := ValidateInfrastructure(&extensionsv1alpha1.Infrastructure{})

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

		It("should allow valid infra resources", func() {
			errorList := ValidateInfrastructure(infra)

			Expect(errorList).To(BeEmpty())
		})
	})

	Describe("#ValidInfrastructureUpdate", func() {
		It("should prevent updating anything if deletion time stamp is set", func() {
			now := metav1.Now()
			infra.DeletionTimestamp = &now

			newInfrastructure := prepareInfrastructureForUpdate(infra)
			newInfrastructure.DeletionTimestamp = &now
			newInfrastructure.Spec.SecretRef.Name = "changed-secretref-name"

			errorList := ValidateInfrastructureUpdate(newInfrastructure, infra)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(field.ErrorTypeForbidden),
				"Field":  Equal("spec"),
				"Detail": Equal("cannot update infrastructure spec if deletion timestamp is set. Requested changes: SecretRef.Name: changed-secretref-name != test"),
			}))))
		})

		It("should prevent updating the type and region", func() {
			newInfrastructure := prepareInfrastructureForUpdate(infra)
			newInfrastructure.Spec.Type = "changed-type"
			newInfrastructure.Spec.Region = "changed-region"

			errorList := ValidateInfrastructureUpdate(newInfrastructure, infra)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("spec.type"),
			})), PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("spec.region"),
			}))))
		})

		It("should allow updating the name of the referenced secret, the provider config, or the ssh public key", func() {
			newInfrastructure := prepareInfrastructureForUpdate(infra)
			newInfrastructure.Spec.SecretRef.Name = "changed-secretref-name"
			newInfrastructure.Spec.ProviderConfig = nil
			newInfrastructure.Spec.SSHPublicKey = []byte("other-key")

			errorList := ValidateInfrastructureUpdate(newInfrastructure, infra)

			Expect(errorList).To(BeEmpty())
		})
	})
})

func prepareInfrastructureForUpdate(obj *extensionsv1alpha1.Infrastructure) *extensionsv1alpha1.Infrastructure {
	newObj := obj.DeepCopy()
	newObj.ResourceVersion = "1"
	return newObj
}
