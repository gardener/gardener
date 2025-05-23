// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package time

import (
	"time"
)

// Ops are time related operations.
type Ops interface {
	// Now returns the current time.
	Now() time.Time
}
