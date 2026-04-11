// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package test

import (
	"time"

	"k8s.io/client-go/util/workqueue"
)

// FakeQueue is a test fake for workqueue.TypedRateLimitingInterface that records
// Add, AddAfter, and Forget calls for use in assertions.
type FakeQueue[T comparable] struct {
	workqueue.TypedRateLimitingInterface[T]

	Added      []T
	AddedAfter []AddAfterArgs[T]
	Forgotten  []T
}

// AddAfterArgs holds the arguments passed to a single AddAfter call.
type AddAfterArgs[T comparable] struct {
	Item     T
	Duration time.Duration
}

// Add records the item as immediately enqueued.
func (f *FakeQueue[T]) Add(item T) {
	f.Added = append(f.Added, item)
}

// AddAfter records the item and the delay.
func (f *FakeQueue[T]) AddAfter(item T, d time.Duration) {
	f.AddedAfter = append(f.AddedAfter, AddAfterArgs[T]{Item: item, Duration: d})
}

// Forget records the item as forgotten.
func (f *FakeQueue[T]) Forget(item T) {
	f.Forgotten = append(f.Forgotten, item)
}
