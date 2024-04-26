// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
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

	"github.com/gardener/gardener/pkg/apis/authentication"
	. "github.com/gardener/gardener/pkg/apis/authentication/validation"
)

var _ = Describe("CredentialsBinding Validation Tests", func() {
	Describe("#ValidateCredentialsBinding", func() {
		var credentialsBinding *authentication.CredentialsBinding

		BeforeEach(func() {
			credentialsBinding = &authentication.CredentialsBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "binding",
					Namespace: "garden",
				},
				Provider: authentication.CredentialsBindingProvider{
					Type: "foo",
				},
				Credentials: authentication.Credentials{
					SecretRef: &corev1.SecretReference{
						Name:      "my-secret",
						Namespace: "my-namespace",
					},
				},
			}
		})

		It("[Secret] should not return any errors", func() {
			errorList := ValidateCredentialsBinding(credentialsBinding)

			Expect(errorList).To(BeEmpty())
		})

		It("[WorkloadIdentity] should not return any errors", func() {
			credentialsBinding.Credentials = authentication.Credentials{
				WorkloadIdentityRef: &authentication.WorkloadIdentityReference{
					Name:      "my-workloadidentity",
					Namespace: "my-namespace",
				},
			}
			errorList := ValidateCredentialsBinding(credentialsBinding)

			Expect(errorList).To(BeEmpty())
		})

		DescribeTable("CredentialsBinding metadata",
			func(objectMeta metav1.ObjectMeta, matcher gomegatypes.GomegaMatcher) {
				credentialsBinding.ObjectMeta = objectMeta

				errorList := ValidateCredentialsBinding(credentialsBinding)

				Expect(errorList).To(matcher)
			},

			Entry("should forbid CredentialsBinding with empty metadata",
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
			Entry("should forbid CredentialsBinding with empty name",
				metav1.ObjectMeta{Name: "", Namespace: "garden"},
				ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("metadata.name"),
				}))),
			),
			Entry("should allow CredentialsBinding with '.' in the name",
				metav1.ObjectMeta{Name: "binding.test", Namespace: "garden"},
				BeEmpty(),
			),
			Entry("should forbid CredentialsBinding with '_' in the name (not a DNS-1123 subdomain)",
				metav1.ObjectMeta{Name: "binding_test", Namespace: "garden"},
				ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("metadata.name"),
				}))),
			),
		)

		It("[Secret] should forbid empty CredentialsBinding resources", func() {
			credentialsBinding.ObjectMeta = metav1.ObjectMeta{}
			credentialsBinding.Credentials.SecretRef = &corev1.SecretReference{}
			credentialsBinding.Provider = authentication.CredentialsBindingProvider{}

			errorList := ValidateCredentialsBinding(credentialsBinding)
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
					"Field": Equal("credentials.secretRef.name"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("provider.type"),
				})),
			))
		})

		It("[WorkloadIdentity] should forbid empty CredentialsBinding resources", func() {
			credentialsBinding.ObjectMeta = metav1.ObjectMeta{}
			credentialsBinding.Credentials.SecretRef = nil
			credentialsBinding.Credentials.WorkloadIdentityRef = &authentication.WorkloadIdentityReference{}
			credentialsBinding.Provider = authentication.CredentialsBindingProvider{}

			errorList := ValidateCredentialsBinding(credentialsBinding)
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
					"Field": Equal("credentials.workloadIdentityRef.name"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("provider.type"),
				})),
			))
		})

		It("Should forbid CredentialBinding with no credential provider", func() {
			credentialsBinding.Credentials.SecretRef = nil
			errList := ValidateCredentialsBinding(credentialsBinding)
			Expect(errList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeForbidden),
					"Field":  Equal("credentials"),
					"Detail": Equal("must specify credentials provider"),
				})),
			))
		})

		It("Should forbid CredentialBinding with multiple credential provider", func() {
			credentialsBinding.Credentials.WorkloadIdentityRef = &authentication.WorkloadIdentityReference{
				Name:      "my-workloadidentity",
				Namespace: "my-namespace",
			}
			errList := ValidateCredentialsBinding(credentialsBinding)
			Expect(errList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeForbidden),
					"Field":  Equal("credentials"),
					"Detail": Equal("must specify exactly one credentials provider"),
				})),
			))
		})

		It("should forbid empty stated Quota names", func() {
			credentialsBinding.Quotas = []corev1.ObjectReference{
				{},
			}

			errorList := ValidateCredentialsBinding(credentialsBinding)

			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("quotas[0].name"),
				})),
			))
		})
	})

	Describe("#ValidateCredentialsBindingUpdate", func() {
		var credentialsBinding *authentication.CredentialsBinding

		BeforeEach(func() {
			credentialsBinding = &authentication.CredentialsBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "binding",
					Namespace: "garden",
				},
				Provider: authentication.CredentialsBindingProvider{
					Type: "foo",
				},
				Credentials: authentication.Credentials{
					SecretRef: &corev1.SecretReference{
						Name:      "my-secret",
						Namespace: "my-namespace",
					},
				},
			}
		})

		It("should forbid updating the CredentialsBinding quota fields", func() {
			newCredentialsBinding := prepareCredentialsBindingForUpdate(credentialsBinding)
			newCredentialsBinding.Quotas = append(newCredentialsBinding.Quotas, corev1.ObjectReference{
				Name:      "new-quota",
				Namespace: "new-quota-ns",
			})

			errorList := ValidateCredentialsBindingUpdate(newCredentialsBinding, credentialsBinding)

			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("quotas"),
				})),
			))
		})

		It("should forbid updating the CredentialsBinding provider when the field is already set", func() {
			newCredentialsBinding := prepareCredentialsBindingForUpdate(credentialsBinding)
			newCredentialsBinding.Provider = authentication.CredentialsBindingProvider{
				Type: "new-type",
			}

			errorList := ValidateCredentialsBindingUpdate(newCredentialsBinding, credentialsBinding)

			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("provider"),
				})),
			))
		})
	})

	Describe("#ValidateCredentialsBindingProvider", func() {
		path := field.NewPath("provider")
		It("should return err when provider is empty", func() {
			errorList := ValidateCredentialsBindingProvider(authentication.CredentialsBindingProvider{}, path)
			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("provider.type"),
				})),
			))
		})

		It("should succeed when provider is valid", func() {
			errorList := ValidateCredentialsBindingProvider(authentication.CredentialsBindingProvider{
				Type: "foo",
			}, path)
			Expect(errorList).To(BeEmpty())
		})

		It("should forbid multiple providers", func() {
			errList := ValidateCredentialsBindingProvider(
				authentication.CredentialsBindingProvider{
					Type: "foo,bar",
				},
				path,
			)
			Expect(errList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("provider.type"),
				})),
			))
		})

	})
})

func prepareCredentialsBindingForUpdate(credentialsBinding *authentication.CredentialsBinding) *authentication.CredentialsBinding {
	c := credentialsBinding.DeepCopy()
	c.ResourceVersion = "1"
	return c
}
