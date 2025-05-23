// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	gomegatypes "github.com/onsi/gomega/types"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/gardener/gardener/pkg/apis/core"
	. "github.com/gardener/gardener/pkg/apis/core/validation"
)

var _ = Describe("SecretBinding Validation Tests", func() {
	Describe("#ValidateSecretBinding", func() {
		var secretBinding *core.SecretBinding

		BeforeEach(func() {
			secretBinding = &core.SecretBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "profile",
					Namespace: "garden",
				},
				SecretRef: corev1.SecretReference{
					Name:      "my-secret",
					Namespace: "my-namespace",
				},
			}
		})

		It("should not return any errors", func() {
			errorList := ValidateSecretBinding(secretBinding)

			Expect(errorList).To(BeEmpty())
		})

		DescribeTable("SecretBinding metadata",
			func(objectMeta metav1.ObjectMeta, matcher gomegatypes.GomegaMatcher) {
				secretBinding.ObjectMeta = objectMeta

				errorList := ValidateSecretBinding(secretBinding)

				Expect(errorList).To(matcher)
			},

			Entry("should forbid SecretBinding with empty metadata",
				metav1.ObjectMeta{},
				ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal("metadata.name"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal("metadata.namespace"),
					})),
				),
			),
			Entry("should forbid SecretBinding with empty name",
				metav1.ObjectMeta{Name: "", Namespace: "garden"},
				ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("metadata.name"),
				}))),
			),
			Entry("should allow SecretBinding with '.' in the name",
				metav1.ObjectMeta{Name: "binding.test", Namespace: "garden"},
				BeEmpty(),
			),
			Entry("should forbid SecretBinding with '_' in the name (not a DNS-1123 subdomain)",
				metav1.ObjectMeta{Name: "binding_test", Namespace: "garden"},
				ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("metadata.name"),
				}))),
			),
		)

		It("should forbid empty SecretBinding resources", func() {
			secretBinding.ObjectMeta = metav1.ObjectMeta{}
			secretBinding.SecretRef = corev1.SecretReference{}
			secretBinding.Provider = &core.SecretBindingProvider{}

			errorList := ValidateSecretBinding(secretBinding)
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
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("secretRef.name"),
				})),
			))
		})

		It("should forbid empty stated Quota names", func() {
			secretBinding.Quotas = []corev1.ObjectReference{
				{},
			}

			errorList := ValidateSecretBinding(secretBinding)

			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("quotas[0].name"),
				})),
			))
		})
	})

	Describe("#ValidateSecretBindingUpdate", func() {
		var secretBinding *core.SecretBinding

		BeforeEach(func() {
			secretBinding = &core.SecretBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "profile",
					Namespace: "garden",
				},
				SecretRef: corev1.SecretReference{
					Name:      "my-secret",
					Namespace: "my-namespace",
				},
			}
		})

		It("should forbid updating the SecretBinding spec", func() {
			newSecretBinding := prepareSecretBindingForUpdate(secretBinding)
			newSecretBinding.SecretRef.Name = "another-name"
			newSecretBinding.Quotas = append(newSecretBinding.Quotas, corev1.ObjectReference{
				Name:      "new-quota",
				Namespace: "new-quota-ns",
			})

			errorList := ValidateSecretBindingUpdate(newSecretBinding, secretBinding)

			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("secretRef"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("quotas"),
				})),
			))
		})

		It("should forbid updating the SecretBinding provider when the field is already set", func() {
			secretBinding.Provider = &core.SecretBindingProvider{
				Type: "old-type",
			}

			newSecretBinding := prepareSecretBindingForUpdate(secretBinding)
			newSecretBinding.Provider = &core.SecretBindingProvider{
				Type: "new-type",
			}

			errorList := ValidateSecretBindingUpdate(newSecretBinding, secretBinding)

			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("provider"),
				})),
			))
		})

		It("should allow updating the SecretBinding provider when the field is not set", func() {
			secretBinding.Provider = nil

			newSecretBinding := prepareSecretBindingForUpdate(secretBinding)
			newSecretBinding.Provider = &core.SecretBindingProvider{
				Type: "new-type",
			}

			errorList := ValidateSecretBindingUpdate(newSecretBinding, secretBinding)

			Expect(errorList).To(BeEmpty())
		})

		It("should allow updating SecretBinding when provider is nil", func() {
			now := metav1.Now()
			secretBinding.DeletionTimestamp = &now
			secretBinding.Finalizers = []string{core.GardenerName}
			secretBinding.Provider = nil

			newSecretBinding := prepareSecretBindingForUpdate(secretBinding)
			newSecretBinding.Finalizers = []string{}

			errorList := ValidateSecretBindingUpdate(newSecretBinding, secretBinding)

			Expect(errorList).To(BeEmpty())
		})
	})

	Describe("#ValidateSecretBindingProvider", func() {
		It("should return err when provider is nil or empty", func() {
			errorList := ValidateSecretBindingProvider(nil)
			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("provider"),
				})),
			))

			errorList = ValidateSecretBindingProvider(&core.SecretBindingProvider{})
			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("provider.type"),
				})),
			))
		})

		It("should succeed when provider is valid", func() {
			errorList := ValidateSecretBindingProvider(&core.SecretBindingProvider{
				Type: "foo",
			})
			Expect(errorList).To(BeEmpty())
		})

	})
})

func prepareSecretBindingForUpdate(secretBinding *core.SecretBinding) *core.SecretBinding {
	s := secretBinding.DeepCopy()
	s.ResourceVersion = "1"
	return s
}
