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
	"k8s.io/apimachinery/pkg/runtime"

	. "github.com/gardener/gardener/pkg/apis/core/validation"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

var _ = Describe("validation", func() {

	Describe("#ValidateShootState, #ValidateShootStateUpdate", func() {
		var (
			shootState *core.ShootState
		)

		BeforeEach(func() {
			shootState = &core.ShootState{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "shoot-foo",
					Namespace: "project-foo",
				},
			}
		})

		It("should forbid shootState containing data required for gardener resource generation with empty name", func() {
			shootState.Spec.Gardener = []core.GardenerResourceData{
				{
					Data: runtime.RawExtension{},
				},
			}

			errorList := ValidateShootState(shootState)
			Expect(errorList).To(HaveLen(1))
			Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("spec.gardener[0].name"),
			}))
		})

		It("should forbid shootState containing extension resource with empty kind", func() {
			shootState.Spec.Extensions = append(shootState.Spec.Extensions, core.ExtensionResourceState{
				State: &runtime.RawExtension{},
			})

			errorList := ValidateShootState(shootState)
			Expect(errorList).To(HaveLen(1))
			Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("spec.extensions[0].kind"),
			}))
		})

		It("should forbid shootState containing extension resource with empty purpose", func() {
			purpose := ""
			shootState.Spec.Extensions = append(shootState.Spec.Extensions, core.ExtensionResourceState{
				State:   &runtime.RawExtension{},
				Kind:    "ControlPlane",
				Purpose: &purpose,
			})

			errorList := ValidateShootState(shootState)
			Expect(errorList).To(HaveLen(1))
			Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("spec.extensions[0].purpose"),
			}))
		})

		It("should forbid shootState containing extension resources w/o names or w/ invalid references", func() {
			purpose := "purpose"
			shootState.Spec.Extensions = append(shootState.Spec.Extensions, core.ExtensionResourceState{
				State:   &runtime.RawExtension{},
				Kind:    "ControlPlane",
				Purpose: &purpose,
				Resources: []core.NamedResourceReference{
					{},
				},
			})

			errorList := ValidateShootState(shootState)
			Expect(errorList).To(HaveLen(4))
			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.resources[0].name"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.resources[0].resourceRef.kind"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.resources[0].resourceRef.name"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.resources[0].resourceRef.apiVersion"),
				})),
			))
		})
	})
})
