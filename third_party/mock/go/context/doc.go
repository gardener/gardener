// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

//go:generate mockgen -package=context -destination=funcs.go github.com/gardener/gardener/third_party/mock/go/context WithTimeout,CancelFunc
//go:generate mockgen -package=context -destination=mocks.go github.com/gardener/gardener/third_party/mock/go/context Context

package context

import (
	"context"
	"time"
)

// Context allows mocking context.Context. The interface is necessary due to an issue with
// golang/mock not being able to generate code for go's core context package.
type Context interface {
	context.Context
}

// WithTimeout is an interface that allows mocking `WithTimeout`.
type WithTimeout interface {
	Do(parent context.Context, timeout time.Duration) (context.Context, context.CancelFunc)
}

// CancelFunc is an interface that allows mocking `CancelFunc`.
type CancelFunc interface {
	Do()
}
