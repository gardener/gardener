// Copyright 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
				"Detail": Equal("SecretRef.Name: changed-secretref-name != test"),
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
