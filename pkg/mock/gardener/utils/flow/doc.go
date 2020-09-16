// SPDX-FileCopyrightText: 2018 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

//go:generate mockgen -package=flow -destination=funcs.go github.com/gardener/gardener/pkg/mock/gardener/utils/flow TaskFn

package flow

import (
	"context"
)

// TaskFn is an interface that allows mocking `flow.TaskFn`s.
type TaskFn interface {
	Do(context.Context) error
}
