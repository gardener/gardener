// SPDX-FileCopyrightText: 2019 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation_test

import (
	"github.com/gardener/gardener/pkg/apis/core"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"

	. "github.com/gardener/gardener/pkg/apis/core/validation"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
)

var _ = Describe("validation", func() {
	var plant *core.Plant

	BeforeEach(func() {
		plant = &core.Plant{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-plant",
				Namespace: "test-namespace",
			},
			Spec: core.PlantSpec{
				SecretRef: v1.LocalObjectReference{
					Name: "test",
				},
			},
		}
	})

	Describe("#ValidPlant", func() {
		It("should forbid empty Plant resources", func() {
			errorList := ValidatePlant(&core.Plant{
				ObjectMeta: metav1.ObjectMeta{},
				Spec:       core.PlantSpec{},
			})

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("metadata.name"),
			})), PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("metadata.namespace"),
			})), PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("spec.secretRef.name"),
			}))))
		})

		It("should allow valid plant resources", func() {
			errorList := ValidatePlant(plant)

			Expect(errorList).To(BeEmpty())
		})
	})

	Describe("#ValidPlantUpdate", func() {
		It("should prevent updating anything if deletion time stamp is set", func() {
			now := metav1.Now()
			plant.DeletionTimestamp = &now

			newPlant := preparePlantForUpdate(plant)
			newPlant.DeletionTimestamp = &now
			newPlant.Spec.SecretRef.Name = "changedName"

			errorList := ValidatePlantUpdate(newPlant, plant)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("spec"),
			}))))
		})
	})
})

func preparePlantForUpdate(obj *core.Plant) *core.Plant {
	newObj := obj.DeepCopy()
	newObj.ResourceVersion = "1"
	return newObj
}
