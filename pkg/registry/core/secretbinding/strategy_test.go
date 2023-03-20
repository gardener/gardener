// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	secretbindingregistry "github.com/gardener/gardener/pkg/registry/core/secretbinding"
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
