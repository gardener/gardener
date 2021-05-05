// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

var _ = Describe("validation", func() {

	Describe("#ValidateShootExtensionStatus", func() {
		var (
			ShootExtensionStatus *core.ShootExtensionStatus
		)

		BeforeEach(func() {
			ShootExtensionStatus = &core.ShootExtensionStatus{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "shoot-foo",
					Namespace:       "project-foo",
					ResourceVersion: "1",
				},
			}
		})

		It("should forbid empty kind field in ExtensionStatus", func() {
			ShootExtensionStatus.Statuses = []core.ExtensionStatus{
				{
					Kind: "",
					Type: "xy",
				},
			}

			errorList := ValidateShootExtensionStatus(ShootExtensionStatus)
			Expect(errorList).To(HaveLen(1))
			Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("statuses[0].kind"),
			}))
		})

		It("should forbid empty type field in ExtensionStatus", func() {
			ShootExtensionStatus.Statuses = []core.ExtensionStatus{
				{
					Type: "",
					Kind: "dd",
				},
			}

			errorList := ValidateShootExtensionStatus(ShootExtensionStatus)
			Expect(errorList).To(HaveLen(1))
			Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("statuses[0].type"),
			}))
		})

		Context("#ValidateShootExtensionStatusExtensionsUpdate", func() {

			It("should forbid updating the type field in ExtensionStatus", func() {
				new := *ShootExtensionStatus
				ShootExtensionStatus.Statuses = []core.ExtensionStatus{
					{
						Kind: "a",
						Type: "xy",
					},
				}

				new.Statuses = []core.ExtensionStatus{
					{
						Kind: "a",
						Type: "bc",
					},
				}

				errorList := ValidateShootExtensionStatusUpdate(&new, ShootExtensionStatus)
				Expect(errorList).To(HaveLen(1))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":     Equal(field.ErrorTypeInvalid),
					"BadValue": Equal("bc"),
					"Field":    Equal("statuses[0].type"),
				}))
			})
		})
	})
})
