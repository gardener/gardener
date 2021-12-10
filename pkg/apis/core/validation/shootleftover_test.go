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
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	gomegatypes "github.com/onsi/gomega/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/pointer"
)

const (
	name        = "foo-bar"
	namespace   = "garden"
	seedName    = "foo"
	shootName   = "bar"
	technicalID = "shoot--foo--bar"
	uid         = "abcdefgh"
)

var _ = Describe("ShootLeftover Validation Tests", func() {
	var (
		shootLeftover *core.ShootLeftover
	)

	BeforeEach(func() {
		shootLeftover = &core.ShootLeftover{
			ObjectMeta: metav1.ObjectMeta{
				Name:       name,
				Namespace:  namespace,
				Generation: 1,
			},
			Spec: core.ShootLeftoverSpec{
				SeedName:    seedName,
				ShootName:   shootName,
				TechnicalID: pointer.String(technicalID),
				UID:         func(v types.UID) *types.UID { return &v }(uid),
			},
			Status: core.ShootLeftoverStatus{
				ObservedGeneration: 1,
			},
		}
	})

	Describe("#ValidateShootLeftover", func() {
		It("should allow valid resources", func() {
			errorList := ValidateShootLeftover(shootLeftover)

			Expect(errorList).To(BeEmpty())
		})

		DescribeTable("ShootLeftover metadata",
			func(objectMeta metav1.ObjectMeta, matcher gomegatypes.GomegaMatcher) {
				shootLeftover.ObjectMeta = objectMeta

				errorList := ValidateShootLeftover(shootLeftover)

				Expect(errorList).To(matcher)
			},

			Entry("should forbid empty metadata",
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
			Entry("should forbid empty name",
				metav1.ObjectMeta{Name: "", Namespace: namespace},
				ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("metadata.name"),
				}))),
			),
			Entry("should forbid '.' in the name (not a DNS-1123 label compliant name)",
				metav1.ObjectMeta{Name: "managedseed.test", Namespace: namespace},
				ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("metadata.name"),
				}))),
			),
			Entry("should forbid '_' in the name (not a DNS-1123 subdomain)",
				metav1.ObjectMeta{Name: "managedseed_test", Namespace: namespace},
				ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("metadata.name"),
				}))),
			),
		)

		It("should forbid empty spec fields", func() {
			shootLeftover.Spec.SeedName = ""
			shootLeftover.Spec.ShootName = ""
			shootLeftover.Spec.TechnicalID = nil
			shootLeftover.Spec.UID = nil

			errorList := ValidateShootLeftover(shootLeftover)

			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.seedName"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.shootName"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.technicalID"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.uid"),
				})),
			))
		})
	})

	Describe("#ValidateShootLeftoverUpdate", func() {
		var (
			newShootLeftover *core.ShootLeftover
		)

		BeforeEach(func() {
			newShootLeftover = shootLeftover.DeepCopy()
			newShootLeftover.ResourceVersion = "1"
		})

		It("should allow valid updates", func() {
			newShootLeftover.Spec.SeedName = seedName + "-new"
			newShootLeftover.Spec.ShootName = shootName + "-new"
			newShootLeftover.Spec.TechnicalID = pointer.String(technicalID + "-new")

			errorList := ValidateShootLeftoverUpdate(newShootLeftover, shootLeftover)

			Expect(errorList).To(BeEmpty())
		})

		It("should forbid changes to immutable metadata fields", func() {
			newShootLeftover.Name = name + "x"

			errorList := ValidateShootLeftoverUpdate(newShootLeftover, shootLeftover)

			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("metadata.name"),
					"Detail": Equal("field is immutable"),
				})),
			))
		})

		It("should forbid changes to spec fields if the deletion timestamp is set", func() {
			now := metav1.Now()
			shootLeftover.DeletionTimestamp = &now
			newShootLeftover.DeletionTimestamp = &now
			newShootLeftover.Spec.SeedName = seedName + "-new"

			errorList := ValidateShootLeftoverUpdate(newShootLeftover, shootLeftover)

			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("spec"),
					"Detail": Equal("field is immutable"),
				})),
			))
		})
	})

	Describe("#ValidateShootLeftoverStatusUpdate", func() {
		var (
			newShootLeftover *core.ShootLeftover
		)

		BeforeEach(func() {
			newShootLeftover = shootLeftover.DeepCopy()
			newShootLeftover.ResourceVersion = "1"
		})

		It("should allow valid status updates", func() {
			errorList := ValidateShootLeftoverStatusUpdate(newShootLeftover, shootLeftover)

			Expect(errorList).To(HaveLen(0))
		})

		It("should forbid negative observed generation", func() {
			newShootLeftover.Status.ObservedGeneration = -1

			errorList := ValidateShootLeftoverStatusUpdate(newShootLeftover, shootLeftover)

			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("status.observedGeneration"),
				})),
			))
		})
	})
})
