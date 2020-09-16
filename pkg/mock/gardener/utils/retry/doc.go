// SPDX-FileCopyrightText: 2019 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

//go:generate mockgen -destination=funcs.go -package=retry github.com/gardener/gardener/pkg/mock/gardener/utils/retry WaitFunc,Func
//go:generate mockgen -destination=mocks.go -package=retry github.com/gardener/gardener/pkg/utils/retry ErrorAggregator,ErrorAggregatorFactory,IntervalFactory

package retry

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
