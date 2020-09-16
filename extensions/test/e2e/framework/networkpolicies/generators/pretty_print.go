// SPDX-FileCopyrightText: 2019 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package generators

import (
	"fmt"
	"strings"
)

// Poor-man's pretty struct printer.
func prettyPrint(i interface{}) string {
	s1 := strings.ReplaceAll(fmt.Sprintf("%#v", i), ", ", ",\n")
	s2 := strings.ReplaceAll(s1, "{", "{\n")
	s3 := strings.ReplaceAll(s2, "} ", "}\n")
	s4 := strings.ReplaceAll(s3, "[", "[\n")
	s5 := strings.ReplaceAll(s4, "] ", "]\n")

	return s5
}
