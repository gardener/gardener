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
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/pointer"
)

var _ = Describe("validation", func() {
	var (
		imageVector           core.ImageVector
		componentImageVectors core.ComponentImageVectors
	)

	BeforeEach(func() {
		imageVector = core.ImageVector{
			{
				Name:           "test-image1",
				Repository:     "test-repo",
				Tag:            pointer.StringPtr("test-tag"),
				RuntimeVersion: pointer.StringPtr(">= 1.6, < 1.8"),
				TargetVersion:  pointer.StringPtr(">= 1.8"),
			},
		}
		componentImageVectors = core.ComponentImageVectors{
			{
				Name:        "test-component1",
				ImageVector: imageVector,
			},
		}
	})

	Describe("#ValidateImageVector", func() {
		It("should allow valid image vectors", func() {
			errorList := ValidateImageVector(imageVector, field.NewPath("iv"))

			Expect(errorList).To(BeEmpty())
		})

		It("should forbid invalid image vectors", func() {
			imageVector[0].Name = ""
			imageVector[0].Repository = ""
			imageVector[0].Tag = pointer.StringPtr("")
			imageVector[0].RuntimeVersion = pointer.StringPtr("")
			imageVector[0].TargetVersion = pointer.StringPtr("!@#")

			errorList := ValidateImageVector(imageVector, field.NewPath("iv"))

			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("iv.images[0].name"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("iv.images[0].repository"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("iv.images[0].tag"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("iv.images[0].runtimeVersion"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("iv.images[0].targetVersion"),
				})),
			))
		})
	})

	Describe("#ValidateComponentImageVectors", func() {
		It("should allow valid component image vectors", func() {
			errorList := ValidateComponentImageVectors(componentImageVectors, field.NewPath("civs"))

			Expect(errorList).To(BeEmpty())
		})

		It("should forbid invalid component image vectors", func() {
			componentImageVectors[0].Name = ""
			componentImageVectors[0].ImageVector[0].Name = ""
			componentImageVectors[0].ImageVector[0].Repository = ""
			componentImageVectors[0].ImageVector[0].Tag = pointer.StringPtr("")
			componentImageVectors[0].ImageVector[0].RuntimeVersion = pointer.StringPtr("")
			componentImageVectors[0].ImageVector[0].TargetVersion = pointer.StringPtr("!@#")

			errorList := ValidateComponentImageVectors(componentImageVectors, field.NewPath("civs"))

			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("civs.components[0].name"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("civs.components[0].imageVector.images[0].name"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("civs.components[0].imageVector.images[0].repository"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("civs.components[0].imageVector.images[0].tag"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("civs.components[0].imageVector.images[0].runtimeVersion"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("civs.components[0].imageVector.images[0].targetVersion"),
				})),
			))
		})
	})
})
