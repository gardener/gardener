// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package validation_test

import (
	"github.com/gardener/gardener/pkg/apis/core"
	. "github.com/gardener/gardener/pkg/apis/core/validation"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	gomegatypes "github.com/onsi/gomega/types"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

var _ = Describe("SecretBinding Validation Tests", func() {
	Describe("#ValidateSecretBinding, #ValidateSecretBindingUpdate", func() {
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

			Expect(errorList).To(HaveLen(0))
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

			errorList := ValidateSecretBinding(secretBinding)

			Expect(errorList).To(HaveLen(3))
			Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("metadata.name"),
			}))
			Expect(*errorList[1]).To(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("metadata.namespace"),
			}))
			Expect(*errorList[2]).To(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("secretRef.name"),
			}))
		})

		It("should forbid empty stated Quota names", func() {
			secretBinding.Quotas = []corev1.ObjectReference{
				{},
			}

			errorList := ValidateSecretBinding(secretBinding)

			Expect(errorList).To(HaveLen(1))
			Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("quotas[0].name"),
			}))
		})

		It("should forbid updating the secret binding spec", func() {
			newSecretBinding := prepareSecretBindingForUpdate(secretBinding)
			newSecretBinding.SecretRef.Name = "another-name"
			newSecretBinding.Quotas = append(newSecretBinding.Quotas, corev1.ObjectReference{
				Name:      "new-quota",
				Namespace: "new-quota-ns",
			})

			errorList := ValidateSecretBindingUpdate(newSecretBinding, secretBinding)

			Expect(errorList).To(HaveLen(2))
			Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("secretRef"),
			}))
			Expect(*errorList[1]).To(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("quotas"),
			}))
		})
	})
})

func prepareSecretBindingForUpdate(secretBinding *core.SecretBinding) *core.SecretBinding {
	s := secretBinding.DeepCopy()
	s.ResourceVersion = "1"
	return s
}
