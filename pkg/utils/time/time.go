// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package time

import (
	"time"
)

type ops struct{}

// Now implements Ops.
func (ops) Now() time.Time {
	return time.Now()
}

// DefaultOps returns the default, `time` based implementation of Ops.
func DefaultOps() Ops {
	return ops{}
}
