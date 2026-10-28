// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package validation_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"

	. "github.com/gardener/gardener/pkg/api/core/validation"
	"github.com/gardener/gardener/pkg/apis/core"
)

var _ = Describe("ExtensionProfile Validation Tests", func() {
	var extensionProfile *core.ExtensionProfile

	BeforeEach(func() {
		extensionProfile = &core.ExtensionProfile{
			ObjectMeta: metav1.ObjectMeta{
				Name: "shoot-traefik",
			},
			Spec: core.ExtensionProfileSpec{
				UpdateStrategy: core.ExtensionUpdateStrategy{
					Default:   core.UpdateStrategyPatch,
					Supported: []core.MachineImageUpdateStrategy{core.UpdateStrategyPatch, core.UpdateStrategyMinor},
				},
				Components: []core.ExtensionProfileComponent{
					{
						Name: "traefik",
						Versions: []core.ExtensionVersion{
							{
								Version: "3.1.4",
								Lifecycle: []core.LifecycleStage{
									{Classification: core.ClassificationSupported},
								},
							},
						},
					},
				},
			},
		}
	})

	Describe("#ValidateExtensionProfile", func() {
		It("should accept a valid ExtensionProfile", func() {
			Expect(ValidateExtensionProfile(extensionProfile)).To(BeEmpty())
		})

		It("should reject a name that is not a valid extension type (DNS1123 label)", func() {
			extensionProfile.Name = "Not_A_Valid_Type"

			Expect(ValidateExtensionProfile(extensionProfile)).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("metadata.name"),
			}))))
		})

		It("should reject a default update strategy not in the supported list", func() {
			extensionProfile.Spec.UpdateStrategy.Supported = []core.MachineImageUpdateStrategy{core.UpdateStrategyMinor}

			Expect(ValidateExtensionProfile(extensionProfile)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("spec.updateStrategy.default"),
			}))))
		})

		It("should reject an unsupported update strategy value", func() {
			extensionProfile.Spec.UpdateStrategy.Default = "weekly"
			extensionProfile.Spec.UpdateStrategy.Supported = []core.MachineImageUpdateStrategy{"weekly"}

			Expect(ValidateExtensionProfile(extensionProfile)).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeNotSupported),
				"Field": Equal("spec.updateStrategy.default"),
			}))))
		})

		It("should reject duplicate component names", func() {
			extensionProfile.Spec.Components = append(extensionProfile.Spec.Components, extensionProfile.Spec.Components[0])

			Expect(ValidateExtensionProfile(extensionProfile)).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeDuplicate),
				"Field": Equal("spec.components[1].name"),
			}))))
		})

		It("should reject a non-semver version", func() {
			extensionProfile.Spec.Components[0].Versions[0].Version = "not-a-version"

			Expect(ValidateExtensionProfile(extensionProfile)).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("spec.components[0].versions[0].version"),
			}))))
		})

		It("should reject an invalid CEL compatibility rule", func() {
			extensionProfile.Spec.Components[0].Versions[0].Compatibility = &core.ExtensionCompatibility{
				Validations: []core.ExtensionCompatibilityValidation{
					{Rule: "shoot.spec.kubernetes.version ===", Message: "bad"},
				},
			}

			Expect(ValidateExtensionProfile(extensionProfile)).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("spec.components[0].versions[0].compatibility.validations[0].rule"),
			}))))
		})

		It("should accept a valid CEL compatibility rule", func() {
			extensionProfile.Spec.Components[0].Versions[0].Compatibility = &core.ExtensionCompatibility{
				Validations: []core.ExtensionCompatibilityValidation{
					{Rule: "shoot.spec.kubernetes.version.matches('^1\\\\.3[2-9]')", Message: "needs 1.32+"},
				},
			}

			Expect(ValidateExtensionProfile(extensionProfile)).To(BeEmpty())
		})
	})
})
