// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	"fmt"
	"strings"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener/pkg/apis/core"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
)

// ValidateMachineImageCapabilities validates the given list of machine images for valid capabilities and architecture values.
func ValidateMachineImageCapabilities(machineImages []core.MachineImage, capabilitiesDefinition core.Capabilities, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	for i, image := range machineImages {
		idxPath := fldPath.Index(i)
		for index, machineVersion := range image.Versions {
			versionsPath := idxPath.Child("versions").Index(index)
			if AreCapabilitiesDefined(capabilitiesDefinition) {
				allErrs = append(allErrs, validateMachineImageVersionCapabilities(machineVersion, capabilitiesDefinition, versionsPath)...)
			} else {
				allErrs = append(allErrs, validateMachineImageVersionArchitecture(machineVersion.Architectures, versionsPath.Child(v1beta1constants.ArchitectureKey))...)
				if machineVersion.CapabilitiesSet != nil {
					allErrs = append(allErrs, field.Forbidden(versionsPath.Child("capabilitiesSet"), "must not provide CapabilitiesSet when no capabilitiesDefinition is defined"))
				}
			}
		}
	}

	return allErrs
}

func validateMachineImageVersionCapabilities(machineImageVersion core.MachineImageVersion, capabilitiesDefinition core.Capabilities, path *field.Path) field.ErrorList {
	errList := field.ErrorList{}

	if machineImageVersion.Architectures != nil {
		errList = append(errList, field.Forbidden(path.Child("architectures"), "must not be set when capabilities are defined"))
	}

	capabilitiesSet, unmarshalErrorList := UnmarshalCapabilitiesSet(machineImageVersion.CapabilitiesSet, path)
	if unmarshalErrorList != nil {
		errList = append(errList, unmarshalErrorList...)
	} else {
		parsedCapabilitiesSet := ParseCapabilitiesSet(capabilitiesSet)
		for i, parsedCapabilities := range parsedCapabilitiesSet {
			errList = append(errList, validateCapabilitiesAgainstDefinition(parsedCapabilities.ToCapabilities(), capabilitiesDefinition, path.Child("capabilitiesSet").Index(i))...)
		}
	}
	return errList
}

// validateMachineTypesCapabilities validates the given list of machine types for valid capabilities and architecture values.
func validateMachineTypesCapabilities(machineTypes []core.MachineType, capabilitiesDefinition core.Capabilities, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	for i, machineType := range machineTypes {
		idxPath := fldPath.Index(i)
		archPath := idxPath.Child(v1beta1constants.ArchitectureKey)

		if AreCapabilitiesDefined(capabilitiesDefinition) {
			allErrs = append(allErrs, ValidateMachineTypeCapabilities(machineType, capabilitiesDefinition, idxPath)...)
		} else {
			allErrs = append(allErrs, validateMachineTypeArchitecture(machineType.Architecture, archPath)...)
			if machineType.Capabilities != nil {
				allErrs = append(allErrs, field.Forbidden(idxPath.Child("capabilities"), "must not provide capabilities when no capabilitiesDefinition is defined"))
			}
		}
	}
	return allErrs
}

// ValidateCapabilitiesDefinition validates the capabilitiesDefinition of a cloudProfile, ensures that the architecture is set and that no capability is empty
func ValidateCapabilitiesDefinition(capabilitiesDefinition core.Capabilities, path *field.Path) field.ErrorList {
	var errList field.ErrorList

	parsedCapabilitiesDefinition := ParseCapabilities(capabilitiesDefinition)
	errList = validateCapabilitiesDefinition(parsedCapabilitiesDefinition, path)

	return errList
}

// ValidateMachineTypeCapabilities validates the capabilities of a machineType, ensures that the architecture is not set and that no capability is empty
func ValidateMachineTypeCapabilities(machineType core.MachineType, capabilitiesDefinition core.Capabilities, path *field.Path) field.ErrorList {
	errList := field.ErrorList{}

	errList = append(errList, validateCapabilitiesAgainstDefinition(machineType.Capabilities, capabilitiesDefinition, path)...)
	errList = append(errList, validateMachineTypeArchitectureCapability(machineType, capabilitiesDefinition, path)...)

	if len(ptr.Deref(machineType.Architecture, "")) > 0 {
		errList = append(errList, field.Forbidden(path.Child(v1beta1constants.ArchitectureKey), "must not be set when capabilities are defined"))
	}

	return errList
}

func validateMachineTypeArchitectureCapability(machineType core.MachineType, capabilitiesDefinition core.Capabilities, path *field.Path) field.ErrorList {
	errList := field.ErrorList{}

	// if there are multiple values for architecture, the architecture for machineTypes must be set and must contain exactly one value
	parsedCapabilitiesDefinition := ParseCapabilities(capabilitiesDefinition)
	parsedCapabilities := ParseCapabilities(machineType.Capabilities)

	allowedArchitectures := parsedCapabilitiesDefinition[v1beta1constants.ArchitectureKey]
	if len(allowedArchitectures) > 1 {
		if value, ok := parsedCapabilities[v1beta1constants.ArchitectureKey]; !ok {
			errList = append(errList, field.Required(path.Child("capabilities", v1beta1constants.ArchitectureKey), fmt.Sprintf("multiple architectures are supported in the cloud profile. So it must be defined and contain exactly one of: %+v", allowedArchitectures)))
		} else if len(value) != 1 {
			errList = append(errList, field.Invalid(path.Child("capabilities", v1beta1constants.ArchitectureKey), value, fmt.Sprintf("must contain exactly one of: %+v", allowedArchitectures)))
		}
	}
	return errList
}

func validateCapabilitiesAgainstDefinition(capabilities core.Capabilities, capabilitiesDefinition core.Capabilities, path *field.Path) field.ErrorList {
	parsedCapabilities := ParseCapabilities(capabilities)
	parsedCapabilitiesDefinition := ParseCapabilities(capabilitiesDefinition)
	var errList field.ErrorList

	if !AreCapabilitiesDefined(capabilitiesDefinition) {
		//  do not create errors if capabilitiesDefinition is not set, as it would pollute the error list
		//  capabilitiesDefinition must be checked before calling this function
		return errList
	}

	for capabilityName, capabilityValues := range parsedCapabilities {
		if len(capabilityValues) == 0 {
			errList = append(errList, field.Invalid(path.Child(capabilityName), capabilityValues, "must not be empty"))
			continue
		}
		if !capabilityValues.IsSubsetOf(parsedCapabilitiesDefinition[capabilityName]) {
			errList = append(errList, field.Invalid(path.Child(capabilityName), capabilityValues, "must be a subset of spec.capabilitiesDefinition of the provider's cloudProfile"))
		}
	}

	return errList
}

func validateCapabilitiesDefinition(definition ParsedCapabilities, path *field.Path) field.ErrorList {
	var errList field.ErrorList

	// during the transition period to capabilities, capabilitiesDefinition is optional thus the empty definition is allowed
	// this check must be removed after capabilitiesDefinition is required
	if len(definition) == 0 {
		return errList
	}

	// architecture is a required capability
	val, ok := definition[v1beta1constants.ArchitectureKey]
	if ok {
		errList = append(errList, validateMachineImageVersionArchitecture(val, path.Child(v1beta1constants.ArchitectureKey))...)
	} else {
		errList = append(errList, field.Required(path.Child(v1beta1constants.ArchitectureKey),
			"allowed architectures are: "+strings.Join(v1beta1constants.ValidArchitectures, ", ")))
	}

	// No empty capabilities allowed
	for capabilityName, capabilityValues := range definition {
		if len(capabilityName) == 0 {
			errList = append(errList, field.Invalid(path, "", "empty capability name is not allowed"))
		}
		if len(capabilityValues) == 0 {
			errList = append(errList, field.Required(path.Child(capabilityName), "must not be empty"))
		} else {
			for _, capabilityValue := range capabilityValues {
				if len(capabilityValue) == 0 {
					errList = append(errList, field.Invalid(path.Child(capabilityName), capabilityValues, "must not contain empty capability values"))
				}
			}
		}
	}
	return errList
}

// UnmarshalCapabilitiesSet unmarshals the raw JSON capabilities set into a list of capabilities.
func UnmarshalCapabilitiesSet(rawCapabilitiesSet []apiextensionsv1.JSON, path *field.Path) ([]core.Capabilities, field.ErrorList) {
	var allErrs field.ErrorList
	capabilitiesSet := make([]core.Capabilities, len(rawCapabilitiesSet))
	for i, rawCapabilities := range rawCapabilitiesSet {
		// unmarshal the raw JSON capabilities set into a list of capabilities
		capabilities := core.Capabilities{}
		err := json.Unmarshal(rawCapabilities.Raw, &capabilities)
		if err != nil {
			allErrs = append(allErrs, field.Invalid(path.Index(i), rawCapabilities, "must be a valid capabilities: "+err.Error()))
		}

		capabilitiesSet[i] = capabilities
	}

	return capabilitiesSet, allErrs
}

// MarshalCapabilitiesSets marshals the capabilities sets into a list of raw JSON capabilities
func MarshalCapabilitiesSets(capabilitiesSets []core.Capabilities, path *field.Path) ([]apiextensionsv1.JSON, field.ErrorList) {
	var allErrs field.ErrorList
	returnJSONs := make([]apiextensionsv1.JSON, len(capabilitiesSets))

	for _, capabilities := range capabilitiesSets {
		rawJSON, err := json.Marshal(capabilities)
		if err != nil {
			allErrs = append(allErrs, field.Invalid(path, capabilities, "must be a valid capabilities definition: "+err.Error()))
		}
		returnJSONs = append(returnJSONs, apiextensionsv1.JSON{Raw: rawJSON})
	}
	return returnJSONs, allErrs
}

// GetCapabilitiesIntersection creates intersection of two parsed capabilities
func GetCapabilitiesIntersection(capabilities ParsedCapabilities, otherCapabilities ParsedCapabilities) ParsedCapabilities {
	intersection := make(ParsedCapabilities)
	for capabilityName, capabilityValues := range capabilities {
		intersection[capabilityName] = capabilityValues.Intersection(otherCapabilities[capabilityName])
	}
	return intersection
}

// ParseCapabilitiesSet parses the value string of each capability in the capabilities set
func ParseCapabilitiesSet(capabilitiesSet []core.Capabilities) []ParsedCapabilities {
	parsedImageCapabilitiesSet := make([]ParsedCapabilities, len(capabilitiesSet))
	for j, capabilitySet := range capabilitiesSet {
		parsedImageCapabilitiesSet[j] = ParseCapabilities(capabilitySet)
	}
	return parsedImageCapabilitiesSet
}

// ParseCapabilities parses the capabilities values string into a map of capability name and capability values
func ParseCapabilities(capabilities core.Capabilities) ParsedCapabilities {
	parsedCapabilities := make(ParsedCapabilities)
	for capabilityName, capabilityValuesString := range capabilities {
		parsedCapabilities[capabilityName] = splitAndSanitize(capabilityValuesString)
	}
	return parsedCapabilities
}

// function to return sanitized values of a comma separated string
// e.g. ",a ,b, c" -> ["a", "b", "c"]
func splitAndSanitize(valueString string) []string {
	values := strings.Split(valueString, ",")
	for i := 0; i < len(values); i++ {
		values[i] = strings.TrimSpace(values[i])
	}
	return values
}

// ParsedCapabilities is the internal runtime representation of Capabilities
type ParsedCapabilities map[string]CapabilityValues

// CapabilityValues is a set of capability values
type CapabilityValues []string

// DeepCopy creates a deep copy of the ParsedCapabilities
func (c ParsedCapabilities) DeepCopy() ParsedCapabilities {
	capabilities := make(ParsedCapabilities)
	for capabilityName, capabilityValues := range c {
		capabilities[capabilityName] = append(CapabilityValues{}, capabilityValues...)
	}
	return capabilities
}

// ToCapabilities converts the ParsedCapabilities to a Capabilities object.
func (c ParsedCapabilities) ToCapabilities() core.Capabilities {
	var capabilities = core.Capabilities{}
	for capabilityName, capabilityValueSet := range c {
		capabilities[capabilityName] = strings.Join(capabilityValueSet, ",")
	}
	return capabilities
}

// HasEmptyCapabilityValue checks if any capability value is empty
func (c ParsedCapabilities) HasEmptyCapabilityValue() bool {
	for _, capabilityValues := range c {
		if len(capabilityValues) == 0 {
			return true
		}
	}
	return false
}

// Intersection creates the intersection of two ParsedCapabilities
func (c ParsedCapabilities) Intersection(other ParsedCapabilities) ParsedCapabilities {
	intersection := make(ParsedCapabilities)
	for capabilityName, capabilityValues := range c {
		intersection[capabilityName] = capabilityValues.Intersection(other[capabilityName])
	}
	return intersection
}

// Add adds values to the CapabilityValues
func (c *CapabilityValues) Add(values ...string) {
	for _, value := range values {
		if !contains(*c, value) {
			*c = append(*c, value)
		}
	}
}

// contains checks if an array contains a specific element
func contains(arr []string, target string) bool {
	for _, element := range arr {
		if element == target {
			return true
		}
	}
	return false
}

// Contains checks if the CapabilityValues contains all values
func (c *CapabilityValues) Contains(values ...string) bool {
	for _, value := range values {
		if !contains(*c, value) {
			return false
		}
	}
	return true
}

// Remove removes values from the CapabilityValues
func (c *CapabilityValues) Remove(value string) {
	for i, v := range *c {
		if v == value {
			*c = append((*c)[:i], (*c)[i+1:]...)
			return
		}
	}
}

// Intersection creates the intersection of two CapabilityValueSets
func (c *CapabilityValues) Intersection(other CapabilityValues) CapabilityValues {
	intersection := CapabilityValues{}
	for _, value := range *c {
		if other.Contains(value) {
			intersection = append(intersection, value)
		}
	}
	return intersection
}

// IsSubsetOf checks if the CapabilityValues is a subset of another CapabilityValues
func (c *CapabilityValues) IsSubsetOf(other CapabilityValues) bool {
	for _, value := range *c {
		if !other.Contains(value) {
			return false
		}
	}
	return true
}
