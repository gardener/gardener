// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

//go:generate mockgen -package=mock -destination=mocks.go github.com/gardener/gardener/pkg/utils/flow/mock TaskFn

package mock

import (
	"context"
)

// TaskFn is an interface that allows mocking `flow.TaskFn`s.
type TaskFn interface {
	Do(context.Context) error
}
