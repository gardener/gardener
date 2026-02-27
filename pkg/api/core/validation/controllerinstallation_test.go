// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	gomegatypes "github.com/onsi/gomega/types"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"

	. "github.com/gardener/gardener/pkg/api/core/validation"
	"github.com/gardener/gardener/pkg/apis/core"
)

var _ = Describe("validation", func() {
	var controllerInstallation *core.ControllerInstallation

	BeforeEach(func() {
		controllerInstallation = &core.ControllerInstallation{
			ObjectMeta: metav1.ObjectMeta{
				Name: "extension-abc",
			},
			Spec: core.ControllerInstallationSpec{
				RegistrationRef: corev1.ObjectReference{
					Name:            "extension",
					ResourceVersion: "1",
				},
				SeedRef: &corev1.ObjectReference{
					Name:            "aws",
					ResourceVersion: "1",
				},
			},
		}
	})

	Describe("#ValidateControllerInstallation", func() {
		DescribeTable("ControllerInstallation metadata",
			func(objectMeta metav1.ObjectMeta, matcher gomegatypes.GomegaMatcher) {
				controllerInstallation.ObjectMeta = objectMeta

				Expect(ValidateControllerInstallation(controllerInstallation)).To(matcher)
			},

			Entry("should forbid ControllerInstallation with empty metadata",
				metav1.ObjectMeta{},
				ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("metadata.name"),
				}))),
			),
			Entry("should forbid ControllerInstallation with empty name",
				metav1.ObjectMeta{Name: ""},
				ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("metadata.name"),
				}))),
			),
			Entry("should forbid ControllerInstallation with '.' in the name (not a DNS-1123 label compliant name)",
				metav1.ObjectMeta{Name: "extension-abc.test"},
				ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("metadata.name"),
				}))),
			),
			Entry("should forbid ControllerInstallation with '_' in the name (not a DNS-1123 subdomain)",
				metav1.ObjectMeta{Name: "extension-abc_test"},
				ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("metadata.name"),
				}))),
			),
		)

		It("should forbid empty ControllerInstallation resources", func() {
			Expect(ValidateControllerInstallation(&core.ControllerInstallation{})).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("metadata.name"),
			})), PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("spec.registrationRef.name"),
			})), PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(field.ErrorTypeForbidden),
				"Field":  Equal("spec.seedRef"),
				"Detail": Equal("either seedRef or shootRef must be set"),
			}))))
		})

		It("should forbid specifying both seedRef and shootRef", func() {
			controllerInstallation.Spec.ShootRef = &corev1.ObjectReference{Name: "foo"}

			Expect(ValidateControllerInstallation(controllerInstallation)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(field.ErrorTypeForbidden),
				"Field":  Equal("spec.seedRef"),
				"Detail": Equal("cannot set both seedRef and shootRef"),
			}))))
		})

		It("should forbid not specifying required seedRef fields", func() {
			controllerInstallation.Spec.SeedRef = &corev1.ObjectReference{}

			Expect(ValidateControllerInstallation(controllerInstallation)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("spec.seedRef.name"),
			}))))
		})

		It("should forbid not specifying required shootRef fields", func() {
			controllerInstallation.Spec.SeedRef = nil
			controllerInstallation.Spec.ShootRef = &corev1.ObjectReference{}

			Expect(ValidateControllerInstallation(controllerInstallation)).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.shootRef.name"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.shootRef.namespace"),
				})),
			))
		})

		It("should allow valid ControllerInstallation resources", func() {
			Expect(ValidateControllerInstallation(controllerInstallation)).To(BeEmpty())
		})
	})

	Describe("#ValidateControllerInstallationUpdate", func() {
		It("should prevent updating anything if deletion time stamp is set", func() {
			now := metav1.Now()

			newControllerInstallation := prepareControllerInstallationForUpdate(controllerInstallation)
			controllerInstallation.DeletionTimestamp = &now
			newControllerInstallation.DeletionTimestamp = &now
			newControllerInstallation.Spec.RegistrationRef.APIVersion = "another-api-version"

			Expect(ValidateControllerInstallationUpdate(newControllerInstallation, controllerInstallation)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(field.ErrorTypeForbidden),
				"Field":  Equal("spec"),
				"Detail": Equal("cannot update controller installation spec if deletion timestamp is set. Requested changes: RegistrationRef.APIVersion: another-api-version != "),
			}))))
		})

		It("should prevent updating immutable fields", func() {
			newControllerInstallation := prepareControllerInstallationForUpdate(controllerInstallation)
			newControllerInstallation.Spec.RegistrationRef.Name = "another-name"
			newControllerInstallation.Spec.RegistrationRef.ResourceVersion = "2"
			newControllerInstallation.Spec.SeedRef.Name = "another-name"
			newControllerInstallation.Spec.SeedRef.ResourceVersion = "2"

			Expect(ValidateControllerInstallationUpdate(newControllerInstallation, controllerInstallation)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("spec.registrationRef.name"),
			})), PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("spec.seedRef.name"),
			}))))
		})

		It("should forbid resetting seedRef when it was set before", func() {
			newControllerInstallation := prepareControllerInstallationForUpdate(controllerInstallation)
			newControllerInstallation.Spec.SeedRef = nil

			Expect(ValidateControllerInstallationUpdate(newControllerInstallation, controllerInstallation)).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("spec.seedRef"),
			}))))
		})

		It("should forbid resetting shootRef when it was set before", func() {
			controllerInstallation.Spec.ShootRef = &corev1.ObjectReference{Name: "foo", Namespace: "bar"}
			newControllerInstallation := prepareControllerInstallationForUpdate(controllerInstallation)
			newControllerInstallation.Spec.ShootRef = nil

			Expect(ValidateControllerInstallationUpdate(newControllerInstallation, controllerInstallation)).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("spec.shootRef"),
			}))))
		})

		It("should forbid updating the name/namespace of the shootRef", func() {
			controllerInstallation.Spec.ShootRef = &corev1.ObjectReference{Name: "foo", Namespace: "bar"}
			newControllerInstallation := prepareControllerInstallationForUpdate(controllerInstallation)
			newControllerInstallation.Spec.ShootRef.Name = "bar"
			newControllerInstallation.Spec.ShootRef.Namespace = "foo"

			Expect(ValidateControllerInstallationUpdate(newControllerInstallation, controllerInstallation)).To(ContainElements(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.shootRef.name"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.shootRef.namespace"),
				})),
			))
		})
	})
})

func prepareControllerInstallationForUpdate(obj *core.ControllerInstallation) *core.ControllerInstallation {
	newObj := obj.DeepCopy()
	newObj.ResourceVersion = "1"
	return newObj
}
