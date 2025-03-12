// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package core

import (
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
			errList = append(errList, field.Invalid(path.Child(capabilityName), strings.Join(capabilityValues.Values, ", "), "must not be empty"))
			continue
		}
		if !capabilityValues.IsSubsetOf(capabilitiesDefinition[capabilityName]) {
			errList = append(errList, field.Invalid(path.Child(capabilityName), strings.Join(capabilityValues.Values, ", "), "must be a subset of spec.capabilitiesDefinition of the provider's cloudProfile"))
		}
	}

	return errList
}
