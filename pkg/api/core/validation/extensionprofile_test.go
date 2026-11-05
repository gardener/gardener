// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package validation_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"

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
					Default:   ptr.To(core.UpdateStrategyPatch),
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
				"Type":   Equal(field.ErrorTypeInvalid),
				"Field":  Equal("metadata.name"),
				"Detail": ContainSubstring("a lowercase RFC 1123 label must consist of"),
			}))))
		})

		It("should reject a default update strategy not in the supported list", func() {
			extensionProfile.Spec.UpdateStrategy.Supported = []core.MachineImageUpdateStrategy{core.UpdateStrategyMinor}

			Expect(ValidateExtensionProfile(extensionProfile)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(field.ErrorTypeInvalid),
				"Field":  Equal("spec.updateStrategy.default"),
				"Detail": Equal("default update strategy must be present in the supported strategies"),
			}))))
		})

		It("should reject an unsupported update strategy value for both default and supported", func() {
			extensionProfile.Spec.UpdateStrategy.Default = ptr.To[core.UpdateStrategy]("weekly")
			extensionProfile.Spec.UpdateStrategy.Supported = []core.MachineImageUpdateStrategy{"weekly"}

			Expect(ValidateExtensionProfile(extensionProfile)).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeNotSupported),
					"Field":  Equal("spec.updateStrategy.default"),
					"Detail": ContainSubstring("supported values:"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeNotSupported),
					"Field":  Equal("spec.updateStrategy.supported[0]"),
					"Detail": ContainSubstring("supported values:"),
				})),
			))
		})

		It("should reject duplicate component names", func() {
			extensionProfile.Spec.Components = append(extensionProfile.Spec.Components, extensionProfile.Spec.Components[0])

			Expect(ValidateExtensionProfile(extensionProfile)).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(field.ErrorTypeDuplicate),
				"Field":  Equal("spec.components[1].name"),
				"Detail": Equal(""),
			}))))
		})

		It("should reject a non-semver version", func() {
			extensionProfile.Spec.Components[0].Versions[0].Version = "not-a-version"

			Expect(ValidateExtensionProfile(extensionProfile)).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(field.ErrorTypeInvalid),
				"Field":  Equal("spec.components[0].versions[0].version"),
				"Detail": ContainSubstring("could not parse version:"),
			}))))
		})

		It("should reject an invalid CEL compatibility rule", func() {
			extensionProfile.Spec.Components[0].Versions[0].Compatibility = &core.ExtensionCompatibility{
				Validations: []core.ExtensionCompatibilityValidation{
					{Rule: "shoot.spec.kubernetes.version ===", Message: ptr.To("bad")},
				},
			}

			Expect(ValidateExtensionProfile(extensionProfile)).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(field.ErrorTypeInvalid),
				"Field":  Equal("spec.components[0].versions[0].compatibility.validations[0].rule"),
				"Detail": ContainSubstring("invalid CEL rule:"),
			}))))
		})

		It("should accept a valid CEL compatibility rule", func() {
			extensionProfile.Spec.Components[0].Versions[0].Compatibility = &core.ExtensionCompatibility{
				Validations: []core.ExtensionCompatibilityValidation{
					{Rule: "shoot.spec.kubernetes.version.matches('^1\\\\.3[2-9]')", Message: ptr.To("needs 1.32+")},
				},
			}

			Expect(ValidateExtensionProfile(extensionProfile)).To(BeEmpty())
		})

		It("should reject an empty supported list", func() {
			extensionProfile.Spec.UpdateStrategy.Supported = nil

			Expect(ValidateExtensionProfile(extensionProfile)).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeRequired),
					"Field":  Equal("spec.updateStrategy.supported"),
					"Detail": Equal("must list at least one supported update strategy"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("spec.updateStrategy.default"),
					"Detail": Equal("default update strategy must be present in the supported strategies"),
				})),
			))
		})

		It("should reject a version without any lifecycle stage", func() {
			extensionProfile.Spec.Components[0].Versions[0].Lifecycle = nil

			Expect(ValidateExtensionProfile(extensionProfile)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(field.ErrorTypeRequired),
				"Field":  Equal("spec.components[0].versions[0].lifecycle"),
				"Detail": Equal("must provide at least one lifecycle stage"),
			}))))
		})

		It("should reject a compatibility envelope without any validation", func() {
			extensionProfile.Spec.Components[0].Versions[0].Compatibility = &core.ExtensionCompatibility{}

			Expect(ValidateExtensionProfile(extensionProfile)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(field.ErrorTypeRequired),
				"Field":  Equal("spec.components[0].versions[0].compatibility.validations"),
				"Detail": Equal("compatibility must list at least one validation; omit the compatibility field entirely if there are none"),
			}))))
		})

		It("should accept a component-level update strategy that omits the default (inherited from the profile)", func() {
			extensionProfile.Spec.Components[0].UpdateStrategy = &core.ExtensionUpdateStrategy{
				Supported: []core.MachineImageUpdateStrategy{core.UpdateStrategyPatch},
			}

			Expect(ValidateExtensionProfile(extensionProfile)).To(BeEmpty())
		})
	})

	Describe("#ValidateExtensionProfileUpdate", func() {
		BeforeEach(func() {
			extensionProfile.ResourceVersion = "1"
		})

		It("should reject turning an already-advanced version unavailable", func() {
			newProfile := extensionProfile.DeepCopy()
			// A single stage with a future start time makes the current classification unavailable.
			newProfile.Spec.Components[0].Versions[0].Lifecycle = []core.LifecycleStage{
				{Classification: core.ClassificationSupported, StartTime: &metav1.Time{Time: time.Now().Add(time.Hour)}},
			}

			Expect(ValidateExtensionProfileUpdate(newProfile, extensionProfile)).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(field.ErrorTypeForbidden),
				"Field":  Equal("spec.components[0].versions[0]"),
				"Detail": ContainSubstring("a version cannot be turned unavailable if it was already moved into a later lifecycle stage"),
			}))))
		})

		It("should accept an unchanged profile", func() {
			Expect(ValidateExtensionProfileUpdate(extensionProfile.DeepCopy(), extensionProfile)).To(BeEmpty())
		})
	})
})
