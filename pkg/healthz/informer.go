// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package healthz

import (
	"context"
	"errors"
	"net/http"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/healthz"
)

func cacheSyncCheckFunc(cacheSyncWaiter cacheSyncWaiter) error {
	// cache.Cache.WaitForCacheSync is racy for closed context, so use context with 1ms timeout instead.
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	if !cacheSyncWaiter.WaitForCacheSync(ctx) {
		return errors.New("informers not synced yet")
	}
	return nil
}

type cacheSyncWaiter interface {
	WaitForCacheSync(ctx context.Context) bool
}

// NewCacheSyncHealthz returns a new healthz.Checker that will pass only if all informers in the given cacheSyncWaiter sync.
func NewCacheSyncHealthz(cacheSyncWaiter cacheSyncWaiter) healthz.Checker {
	return func(_ *http.Request) error { return cacheSyncCheckFunc(cacheSyncWaiter) }
}
