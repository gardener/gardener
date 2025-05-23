// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package errors

import (
	"fmt"

	"github.com/hashicorp/go-multierror"
)

// NewErrorFormatFuncWithPrefix creates a new multierror.ErrorFormatFunc which can be used as an ErrorFormat on
// multierror.Error instances. The error string is prefixed with <prefix>, all errors are concatenated at the end.
// This is similar to multierror.ListFormatFunc but does not use any escape sequences, which will look weird in
// the status of Kubernetes objects or controller logs.
func NewErrorFormatFuncWithPrefix(prefix string) multierror.ErrorFormatFunc {
	return func(es []error) string {
		if len(es) == 1 {
			return fmt.Sprintf("%s: 1 error occurred: %s", prefix, es[0])
		}

		combinedMsg := ""
		for i, err := range es {
			if i > 0 {
				combinedMsg += ", "
			}
			combinedMsg += err.Error()
		}

		return fmt.Sprintf("%s: %d errors occurred: [%s]", prefix, len(es), combinedMsg)
	}
}
