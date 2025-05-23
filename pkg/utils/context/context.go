// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package context

import (
	"context"
	"time"
)

// FromStopChannel creates a new context from a given stop channel.
func FromStopChannel(stopCh <-chan struct{}) context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		defer cancel()
		<-stopCh
	}()

	return ctx
}

type ops struct{}

// WithTimeout returns the context with the given timeout and a CancelFunc to cleanup its resources.
func (ops) WithTimeout(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, timeout)
}

// DefaultOps returns the default Ops implementation.
func DefaultOps() Ops {
	return ops{}
}
