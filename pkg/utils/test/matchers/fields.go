// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
	var m []interface{}
	for _, f := range fields {
		m = append(m, HaveFields(f))
	}
	return gomega.ConsistOf(m...)
}
