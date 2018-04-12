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

package errors

import (
	"regexp"

	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
)

// New creates a new Error object from a given Golang error. It extracts 'CODE:<some-code>' from the beginning
// of the error description. An error without a CODE is also valid.
func New(err error) *Error {
	regex := regexp.MustCompile(`(?s)(CODE\:([^ ]*) )?(.*)`)
	match := regex.FindStringSubmatch(err.Error())

	e := &Error{
		Description: match[3],
	}

	if len(match[2]) != 0 {
		code := gardenv1beta1.ErrorCode(match[2])
		e.Code = &code
	}

	return e
}
