// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
