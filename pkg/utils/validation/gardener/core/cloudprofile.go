// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"encoding/json"
	"strings"

	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/gardener/gardener/pkg/apis/core"
)

// ValidateCapabilitiesAgainstDefinition validates that the capabilities object is valid according to the capabilitiesDefinition
func ValidateCapabilitiesAgainstDefinition(capabilities core.Capabilities, capabilitiesDefinition core.Capabilities, path *field.Path) field.ErrorList {
	var errList field.ErrorList

	if !capabilitiesDefinition.HasEntries() {
		//  do not create errors if capabilitiesDefinition is not set, as it would pollute the error list
		//  capabilitiesDefinition must be checked before calling this function
		return errList
	}

	for capabilityName, capabilityValues := range capabilities {
		if len(capabilityValues.Values) == 0 {
			errList = append(errList, field.Invalid(path.Child(string(capabilityName)), capabilityValues.Values, "must not be empty"))
			continue
		}
		if !capabilityValues.IsSubsetOf(capabilitiesDefinition[capabilityName]) {
			errList = append(errList, field.Invalid(path.Child(string(capabilityName)), capabilityValues.Values, "must be a subset of spec.capabilitiesDefinition of the provider's cloudProfile"))
		}
	}

	return errList
}

// UnmarshalCapabilitiesSet unmarshals the raw JSON capabilities set into a list of capabilities.
func UnmarshalCapabilitiesSet(rawCapabilitiesSet core.CapabilitiesSet, path *field.Path) ([]core.Capabilities, field.ErrorList) {
	var allErrs field.ErrorList
	capabilitiesSet := make([]core.Capabilities, len(rawCapabilitiesSet))

	for i, rawCapabilities := range rawCapabilitiesSet {
		var temp map[string]string
		err := json.Unmarshal(rawCapabilities.Raw, &temp)
		if err != nil {
			allErrs = append(allErrs, field.Invalid(path.Index(i), string(rawCapabilities.Raw), "must be a valid capabilities: "+err.Error()))
		}

		capabilities := make(core.Capabilities)
		for k, v := range temp {
			capabilitiesValues := core.CapabilityValues{}
			rawValues := strings.Split(v, ",")

			// trim whitespaces from the values
			for _, value := range rawValues {
				capabilitiesValues.Values = append(capabilitiesValues.Values, strings.TrimSpace(value))
			}

			capabilities[core.CapabilityName(k)] = capabilitiesValues
		}
		capabilitiesSet[i] = capabilities
	}
	return capabilitiesSet, allErrs
}
