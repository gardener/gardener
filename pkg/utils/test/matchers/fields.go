// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package matchers

import (
	"github.com/onsi/gomega"
	"github.com/onsi/gomega/gstruct"
	"github.com/onsi/gomega/types"
)

// HaveFields succeeds if actual is a pointer and has a specific fields.
// Ignores extra elements or fields.
func HaveFields(fields gstruct.Fields) types.GomegaMatcher {
	return gstruct.PointTo(gstruct.MatchFields(gstruct.IgnoreExtras, fields))
}

// ConsistOfFields succeeds if actual matches all selected fields.
// Actual must be an array, slice or map.  For maps, ConsistOfFields matches against the map's values.
// Actual's elements must be pointers.
func ConsistOfFields(fields ...gstruct.Fields) types.GomegaMatcher {
	var m []any
	for _, f := range fields {
		m = append(m, HaveFields(f))
	}
	return gomega.ConsistOf(m...)
}
