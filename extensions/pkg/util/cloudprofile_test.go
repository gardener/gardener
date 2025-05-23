// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package util_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/gardener/gardener/extensions/pkg/util"
	"github.com/gardener/gardener/pkg/apis/core"
	"github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

var _ = Describe("ImagesContext", func() {
	Describe("#NewCoreImagesContext", func() {
		It("should successfully construct an ImagesContext from core.MachineImage slice", func() {
			imagesContext := util.NewCoreImagesContext([]core.MachineImage{
				{Name: "image-1", Versions: []core.MachineImageVersion{
					{ExpirableVersion: core.ExpirableVersion{Version: "1.0.0"}},
					{ExpirableVersion: core.ExpirableVersion{Version: "2.0.0"}},
				}},
				{Name: "image-2", Versions: []core.MachineImageVersion{
					{ExpirableVersion: core.ExpirableVersion{Version: "3.0.0"}},
				}},
			})

			i, exists := imagesContext.GetImage("image-1")
			Expect(exists).To(BeTrue())
			Expect(i.Name).To(Equal("image-1"))
			Expect(i.Versions).To(ConsistOf(
				core.MachineImageVersion{ExpirableVersion: core.ExpirableVersion{Version: "1.0.0"}},
				core.MachineImageVersion{ExpirableVersion: core.ExpirableVersion{Version: "2.0.0"}},
			))

			i, exists = imagesContext.GetImage("image-2")
			Expect(exists).To(BeTrue())
			Expect(i.Name).To(Equal("image-2"))
			Expect(i.Versions).To(ConsistOf(
				core.MachineImageVersion{ExpirableVersion: core.ExpirableVersion{Version: "3.0.0"}},
			))

			i, exists = imagesContext.GetImage("image-99")
			Expect(exists).To(BeFalse())
			Expect(i.Name).To(Equal(""))
			Expect(i.Versions).To(BeEmpty())

			v, exists := imagesContext.GetImageVersion("image-1", "1.0.0")
			Expect(exists).To(BeTrue())
			Expect(v).To(Equal(core.MachineImageVersion{ExpirableVersion: core.ExpirableVersion{Version: "1.0.0"}}))

			v, exists = imagesContext.GetImageVersion("image-1", "99.0.0")
			Expect(exists).To(BeFalse())
			Expect(v).To(Equal(core.MachineImageVersion{}))

			v, exists = imagesContext.GetImageVersion("image-99", "99.0.0")
			Expect(exists).To(BeFalse())
			Expect(v).To(Equal(core.MachineImageVersion{}))
		})
	})

	Describe("#NewV1beta1ImagesContext", func() {
		It("should successfully construct an ImagesContext from v1beta1.MachineImage slice", func() {
			imagesContext := util.NewV1beta1ImagesContext([]v1beta1.MachineImage{
				{Name: "image-1", Versions: []v1beta1.MachineImageVersion{
					{ExpirableVersion: v1beta1.ExpirableVersion{Version: "1.0.0"}},
					{ExpirableVersion: v1beta1.ExpirableVersion{Version: "2.0.0"}},
				}},
				{Name: "image-2", Versions: []v1beta1.MachineImageVersion{
					{ExpirableVersion: v1beta1.ExpirableVersion{Version: "3.0.0"}},
				}},
			})

			i, exists := imagesContext.GetImage("image-1")
			Expect(exists).To(BeTrue())
			Expect(i.Name).To(Equal("image-1"))
			Expect(i.Versions).To(ConsistOf(
				v1beta1.MachineImageVersion{ExpirableVersion: v1beta1.ExpirableVersion{Version: "1.0.0"}},
				v1beta1.MachineImageVersion{ExpirableVersion: v1beta1.ExpirableVersion{Version: "2.0.0"}},
			))

			i, exists = imagesContext.GetImage("image-2")
			Expect(exists).To(BeTrue())
			Expect(i.Name).To(Equal("image-2"))
			Expect(i.Versions).To(ConsistOf(
				v1beta1.MachineImageVersion{ExpirableVersion: v1beta1.ExpirableVersion{Version: "3.0.0"}},
			))

			i, exists = imagesContext.GetImage("image-99")
			Expect(exists).To(BeFalse())
			Expect(i.Name).To(Equal(""))
			Expect(i.Versions).To(BeEmpty())

			v, exists := imagesContext.GetImageVersion("image-1", "1.0.0")
			Expect(exists).To(BeTrue())
			Expect(v).To(Equal(v1beta1.MachineImageVersion{ExpirableVersion: v1beta1.ExpirableVersion{Version: "1.0.0"}}))

			v, exists = imagesContext.GetImageVersion("image-1", "99.0.0")
			Expect(exists).To(BeFalse())
			Expect(v).To(Equal(v1beta1.MachineImageVersion{}))

			v, exists = imagesContext.GetImageVersion("image-99", "99.0.0")
			Expect(exists).To(BeFalse())
			Expect(v).To(Equal(v1beta1.MachineImageVersion{}))
		})
	})
})

var _ = Describe("Capabilities Functions", func() {
	Describe("#ValidateCapabilities", func() {
		fieldPath := field.NewPath("spec", "machineImages[0]", "capabilities")
		It("should return no errors for valid capabilities", func() {
			capabilities := core.Capabilities{
				"architecture": {"amd64"},
				"feature":      {"enabled"},
			}
			capabilitiesDefinition := []core.CapabilityDefinition{
				{Name: "architecture", Values: []string{"amd64", "arm64"}},
				{Name: "feature", Values: []string{"enabled", "disabled"}},
			}

			allErrs := util.ValidateCapabilities(capabilities, capabilitiesDefinition, fieldPath)
			Expect(allErrs).To(BeEmpty())
		})

		It("should return an error for unsupported capability keys", func() {
			capabilities := core.Capabilities{
				"unsupportedKey": {"value"},
			}
			capabilitiesDefinition := []core.CapabilityDefinition{
				{Name: "architecture", Values: []string{"amd64"}},
				{Name: "supportedKey", Values: []string{"value"}},
			}

			allErrs := util.ValidateCapabilities(capabilities, capabilitiesDefinition, fieldPath)

			Expect(allErrs).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":     Equal(field.ErrorTypeNotSupported),
				"Field":    Equal(fieldPath.String()),
				"BadValue": Equal("unsupportedKey"),
				"Detail":   ContainSubstring("supported values:"),
			}))))
		})

		It("should return an error for unsupported capability values", func() {
			capabilities := core.Capabilities{
				"architecture": {"unsupportedValue"},
			}
			capabilitiesDefinition := []core.CapabilityDefinition{
				{Name: "architecture", Values: []string{"amd64", "arm64"}},
			}

			allErrs := util.ValidateCapabilities(capabilities, capabilitiesDefinition, field.NewPath("spec", "capabilities"))
			Expect(allErrs).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":     Equal(field.ErrorTypeNotSupported),
				"Field":    Equal("spec.capabilities.architecture[0]"),
				"BadValue": Equal("unsupportedValue"),
				"Detail":   ContainSubstring("supported values:"),
			}))))
		})

		Context("architecture validation", func() {

			It("should return an error when multiple architectures are supported but none is defined", func() {
				capabilities := core.Capabilities{}
				capabilitiesDefinition := []core.CapabilityDefinition{
					{Name: "architecture", Values: []string{"amd64", "arm64"}},
				}

				allErrs := util.ValidateCapabilities(capabilities, capabilitiesDefinition, field.NewPath("spec", "capabilities"))
				Expect(allErrs).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":     Equal(field.ErrorTypeInvalid),
					"Field":    Equal("spec.capabilities.architecture"),
					"BadValue": BeNil(),
					"Detail":   ContainSubstring("must define exactly one architecture"),
				}))))
			})

			It("should return an error when multiple architectures are supported but more than one is defined", func() {
				capabilities := core.Capabilities{
					"architecture": {"amd64", "arm64"},
				}
				capabilitiesDefinition := []core.CapabilityDefinition{
					{Name: "architecture", Values: []string{"amd64", "arm64"}},
				}

				allErrs := util.ValidateCapabilities(capabilities, capabilitiesDefinition, field.NewPath("spec", "capabilities"))
				Expect(allErrs).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("spec.capabilities.architecture"),
					"Detail": ContainSubstring("must define exactly one architecture"),
				}))))
			})

			It("should return no errors when only one architecture is supported and none is defined", func() {
				capabilities := core.Capabilities{}
				capabilitiesDefinition := []core.CapabilityDefinition{
					{Name: "architecture", Values: []string{"amd64"}},
				}

				allErrs := util.ValidateCapabilities(capabilities, capabilitiesDefinition, field.NewPath("spec", "capabilities"))
				Expect(allErrs).To(BeEmpty())
			})
		})

	})

	Describe("#GetVersionCapabilitySets", func() {
		It("should return the defined capability sets if present", func() {
			version := core.MachineImageVersion{
				CapabilitySets: []core.CapabilitySet{
					{Capabilities: core.Capabilities{"key1": {"value1"}}},
				},
			}
			capabilitiesDefinition := []core.CapabilityDefinition{
				{Name: "key1", Values: []string{"value1"}},
			}

			result := util.GetVersionCapabilitySets(version, capabilitiesDefinition)
			Expect(result).To(Equal(version.CapabilitySets))
		})

		It("should return a default capability set if none are defined and only one architecture is supported", func() {
			version := core.MachineImageVersion{}
			capabilitiesDefinition := []core.CapabilityDefinition{
				{Name: "architecture", Values: []string{"amd64"}},
			}

			result := util.GetVersionCapabilitySets(version, capabilitiesDefinition)
			Expect(result).To(Equal([]core.CapabilitySet{{Capabilities: core.Capabilities{"architecture": {"amd64"}}}}))
		})

		It("should return an empty slice if no capability sets are defined and multiple architectures are supported", func() {
			version := core.MachineImageVersion{}
			capabilitiesDefinition := []core.CapabilityDefinition{
				{Name: "architecture", Values: []string{"amd64", "arm64"}},
			}

			result := util.GetVersionCapabilitySets(version, capabilitiesDefinition)
			Expect(result).To(BeEmpty())
		})
	})

	Describe("#AreCapabilitiesEqual", func() {
		capabilitiesDefinition := []core.CapabilityDefinition{
			{Name: "key1", Values: []string{"value1", "value2"}},
			{Name: "key2", Values: []string{"valueA", "valueB"}},
			{Name: "architecture", Values: []string{"amd64"}},
		}

		It("should return true for equal capabilities", func() {
			a := core.Capabilities{"key1": {"value1"}}
			b := core.Capabilities{"key1": {"value1"}}

			result := util.AreCapabilitiesEqual(a, b, capabilitiesDefinition)
			Expect(result).To(BeTrue())
		})

		It("should return false for capabilities with different keys", func() {
			a := core.Capabilities{"key1": {"value1"}}
			b := core.Capabilities{"key2": {"value1"}}

			result := util.AreCapabilitiesEqual(a, b, capabilitiesDefinition)
			Expect(result).To(BeFalse())
		})

		It("should return false for capabilities with different values", func() {
			a := core.Capabilities{"key1": {"value1"}}
			b := core.Capabilities{"key1": {"value2"}}

			result := util.AreCapabilitiesEqual(a, b, capabilitiesDefinition)
			Expect(result).To(BeFalse())
		})

		It("should return true for capabilities with different values that are equal to those in the capabilitiesDefinition", func() {
			a := core.Capabilities{"key1": {"value1", "value2"}}
			b := core.Capabilities{"key2": {"valueA", "valueB"}}

			result := util.AreCapabilitiesEqual(a, b, capabilitiesDefinition)
			Expect(result).To(BeTrue())
		})
	})

	Describe("#ApplyDefaultCapabilities", func() {
		var capabilitiesDefinition []core.CapabilityDefinition
		BeforeEach(func() {
			capabilitiesDefinition = []core.CapabilityDefinition{
				{Name: "key1", Values: []string{"value1", "value2"}},
				{Name: "key2", Values: []string{"valueA", "valueB"}},
				{Name: "architecture", Values: []string{"amd64"}},
			}
		})
		It("should set default capabilities if none are defined", func() {
			capabilities := core.Capabilities{}

			result := util.ApplyDefaultCapabilities(capabilities, capabilitiesDefinition)
			Expect(result).To(Equal(core.Capabilities{
				"key1":         {"value1", "value2"},
				"key2":         {"valueA", "valueB"},
				"architecture": {"amd64"},
			}))
		})

		It("should not overwrite existing capabilities and add missing capabilities from the definition", func() {
			capabilities := core.Capabilities{"key1": {"value1"}}

			result := util.ApplyDefaultCapabilities(capabilities, capabilitiesDefinition)
			Expect(result["key1"]).To(Equal(capabilities["key1"]))
			Expect([]string(result["key2"])).To(Equal([]string{"valueA", "valueB"})) // Convert to []string
			Expect([]string(result["architecture"])).To(Equal([]string{"amd64"}))    // Convert to []string
		})
	})
})
