// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package secretbinding_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/gardener/gardener/pkg/apis/core"
	secretbindingregistry "github.com/gardener/gardener/pkg/apiserver/registry/core/secretbinding"
)

var _ = Describe("Strategy", func() {
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

	Describe("#Validate", func() {
		It("should forbid creating SecretBinding when provider is nil or empty", func() {
			secretBinding.Provider = nil

			errorList := secretbindingregistry.Strategy.Validate(context.TODO(), secretBinding)
			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("provider"),
				})),
			))

			secretBinding.Provider = &core.SecretBindingProvider{}

			errorList = secretbindingregistry.Strategy.Validate(context.TODO(), secretBinding)
			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("provider.type"),
				})),
			))
		})

		It("should allow creating SecretBinding when provider is valid", func() {
			secretBinding.Provider = &core.SecretBindingProvider{
				Type: "foo",
			}

			errorList := secretbindingregistry.Strategy.Validate(context.TODO(), secretBinding)
			Expect(errorList).To(BeEmpty())
		})
	})
})
