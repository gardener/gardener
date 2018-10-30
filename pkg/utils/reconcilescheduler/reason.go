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

package reconcilescheduler

import (
	"fmt"
)

const (
	// CodeActivated is a reason code indicating that an element has been activated.
	CodeActivated = iota
	// CodeAlreadyActive is a reason code indicating that an element is active already.
	CodeAlreadyActive = iota

	// CodeChildActive is a reason code indicating that an element's child is currently active.
	CodeChildActive = iota

	// CodeParentPending is a reason code indicating that the element's parent has precedence.
	CodeParentPending = iota
	// CodeParentActive is a reason code indicating that the element's parent is currently active.
	CodeParentActive = iota
	// CodeParentNotReconciled is a reason code indicating that the element's parent has not yet been reconciled.
	CodeParentNotReconciled = iota
	// CodeParentUnknown is a reason code indicating that the element's parent is not (yet) known.
	CodeParentUnknown = iota

	// CodeOther is a reason code for unspecified reasons.
	CodeOther = iota
)

// Reason is a structure type that contains a reason code as well as a message. It is returned to users' requests
// in order to explain the made decision.
type Reason struct {
	code    int
	message string
}

// NewReason creates a new reason structure with the given <code> and the message.
func NewReason(code int, msgFmt string, args ...interface{}) *Reason {
	return &Reason{
		code:    code,
		message: fmt.Sprintf(msgFmt, args...),
	}
}

// String returns the string representation of Reason.
func (r *Reason) String() string {
	return r.message
}

// Code returns the code of the Reason.
func (r *Reason) Code() int {
	return r.code
}

// Message returns the message of the Reason.
func (r *Reason) Message() string {
	return r.message
}
