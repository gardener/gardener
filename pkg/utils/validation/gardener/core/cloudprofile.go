// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"strings"

	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/gardener/gardener/pkg/apis/core"
)

// AreCapabilitiesDefined checks if the capabilitiesDefinition is set and not empty
// it is intended to be used only during the transition period to capabilities and should be removed after capabilitiesDefinition is required
// then only validateCapabilitiesDefinition should be used
func AreCapabilitiesDefined(capabilitiesDefinition core.Capabilities) bool {
	return len(capabilitiesDefinition) != 0
}

// ValidateCapabilitiesAgainstDefinition validates that the capabilities object is valid according to the capabilitiesDefinition
func ValidateCapabilitiesAgainstDefinition(capabilities core.Capabilities, capabilitiesDefinition core.Capabilities, path *field.Path) field.ErrorList {
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
			errList = append(errList, field.Invalid(path.Child(capabilityName), capabilityValues.ToSlice(), "must not be empty"))
			continue
		}
		if !capabilityValues.IsSubsetOf(parsedCapabilitiesDefinition[capabilityName]) {
			errList = append(errList, field.Invalid(path.Child(capabilityName), capabilityValues.ToSlice(), "must be a subset of spec.capabilitiesDefinition of the provider's cloudProfile"))
		}
	}

	return errList
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
func splitAndSanitize(capabilities string) []string {
	var sanitizedValues []string
	for _, v := range strings.Split(capabilities, ",") {
		sanitizedValues = append(sanitizedValues, strings.TrimSpace(v))
	}
	return sanitizedValues
}

// ParsedCapabilities is the internal representation of Capabilities with parsed values
type ParsedCapabilities map[string]CapabilityValues

// CapabilityValues is a set of capability values
type CapabilityValues []string

// ToCapabilities converts the ParsedCapabilities to a Capabilities object.
func (c ParsedCapabilities) ToCapabilities() core.Capabilities {
	var capabilities = core.Capabilities{}
	for capabilityName, capabilityValueSet := range c {
		capabilities[capabilityName] = strings.Join(capabilityValueSet, ",")
	}
	return capabilities
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

// IsSubsetOf checks if the CapabilityValues is a subset of another CapabilityValues
func (c *CapabilityValues) IsSubsetOf(other CapabilityValues) bool {
	for _, value := range *c {
		if !other.Contains(value) {
			return false
		}
	}
	return true
}

// ToSlice returns a copy of the CapabilityValues as a slice
func (c *CapabilityValues) ToSlice() []string {
	return append([]string{}, *c...)
}
