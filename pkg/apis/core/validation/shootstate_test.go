// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	gomegatypes "github.com/onsi/gomega/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/gardener/gardener/pkg/apis/core"
	. "github.com/gardener/gardener/pkg/apis/core/validation"
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

		DescribeTable("ShootState metadata",
			func(objectMeta metav1.ObjectMeta, matcher gomegatypes.GomegaMatcher) {
				shootState.ObjectMeta = objectMeta

				errorList := ValidateShootState(shootState)

				Expect(errorList).To(matcher)
			},

			Entry("should forbid ShootState with empty metadata",
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
			Entry("should forbid ShootState with empty name",
				metav1.ObjectMeta{Name: "", Namespace: "project-foo"},
				ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("metadata.name"),
				}))),
			),
			Entry("should forbid ShootState with '.' in the name (not a DNS-1123 label compliant name)",
				metav1.ObjectMeta{Name: "shoot.test", Namespace: "project-foo"},
				ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("metadata.name"),
				}))),
			),
			Entry("should forbid ShootState with '_' in the name (not a DNS-1123 subdomain)",
				metav1.ObjectMeta{Name: "shoot_test", Namespace: "project-foo"},
				ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("metadata.name"),
				}))),
			),
		)

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
