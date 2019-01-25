// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"

	. "github.com/gardener/gardener/pkg/apis/core/validation"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
)

var _ = Describe("validation", func() {
	var controllerRegistration *core.ControllerRegistration

	BeforeEach(func() {
		controllerRegistration = &core.ControllerRegistration{
			ObjectMeta: metav1.ObjectMeta{
				Name: "extension-abc",
			},
			Spec: core.ControllerRegistrationSpec{
				Resources: []core.ControllerResource{
					{
						Kind: "OperatingSystemConfig",
						Type: "my-os",
					},
				},
			},
		}
	})

	Describe("#ValidateControllerRegistration", func() {
		It("should forbid empty ControllerRegistration resources", func() {
			errorList := ValidateControllerRegistration(&core.ControllerRegistration{})

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("metadata.name"),
			}))))
		})

		It("should forbid empty values in a given resource", func() {
			controllerRegistration.Spec.Resources[0].Type = ""

			errorList := ValidateControllerRegistration(controllerRegistration)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("spec.resources[0].type"),
			}))))
		})

		It("should forbid duplicates in given resources", func() {
			controllerRegistration.Spec.Resources = append(controllerRegistration.Spec.Resources, controllerRegistration.Spec.Resources[0])

			errorList := ValidateControllerRegistration(controllerRegistration)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeDuplicate),
				"Field": Equal("spec.resources[1]"),
			}))))
		})

		It("should allow specifying no resources", func() {
			controllerRegistration.Spec.Resources = nil

			errorList := ValidateControllerRegistration(controllerRegistration)

			Expect(errorList).To(BeEmpty())
		})

		It("should allow valid ControllerRegistration resources", func() {
			errorList := ValidateControllerRegistration(controllerRegistration)

			Expect(errorList).To(BeEmpty())
		})
	})

	Describe("#ValidateControllerRegistrationUpdate", func() {
		It("should prevent updating anything if deletion time stamp is set", func() {
			now := metav1.Now()

			newControllerRegistration := prepareControllerRegistrationForUpdate(controllerRegistration)
			controllerRegistration.DeletionTimestamp = &now
			newControllerRegistration.DeletionTimestamp = &now
			newControllerRegistration.Spec.Resources[0].Type = "another-os"

			errorList := ValidateControllerRegistrationUpdate(newControllerRegistration, controllerRegistration)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("spec"),
			}))))
		})
	})
})

func prepareControllerRegistrationForUpdate(obj *core.ControllerRegistration) *core.ControllerRegistration {
	newObj := obj.DeepCopy()
	newObj.ResourceVersion = "1"
	return newObj
}
