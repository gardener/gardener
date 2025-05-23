// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

//go:generate mockgen -destination=funcs.go -package=mock github.com/gardener/gardener/pkg/utils/retry/mock WaitFunc,Func
//go:generate mockgen -destination=mocks.go -package=mock github.com/gardener/gardener/pkg/utils/retry ErrorAggregator,ErrorAggregatorFactory,IntervalFactory

package mock

import (
	"context"
)

// WaitFunc allows mocking retry.WaitFunc.
type WaitFunc interface {
	Do(ctx context.Context) (context.Context, context.CancelFunc)
}

// Func allows mocking retry.Func.
type Func interface {
	Do(ctx context.Context) (done bool, err error)
}
