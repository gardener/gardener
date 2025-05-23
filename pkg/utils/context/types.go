// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package context

import (
	"context"
	"time"
)

// Ops are operations to do with a context. They mimic the functions from the context package.
type Ops interface {
	// WithTimeout returns a new context with the given timeout that can be canceled with the returned function.
	WithTimeout(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc)
}
