// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package healthz

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/utils/clock"
	"k8s.io/utils/ptr"
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

// DefaultCacheSyncDeadline is a default deadline for the cache sync healthz check.
const DefaultCacheSyncDeadline = 3 * time.Minute

// NewCacheSyncHealthzWithDeadline is like NewCacheSyncHealthz, however, it fails when at least one informer in the
// given cacheSyncWaiter is not synced for at least the given deadline.
func NewCacheSyncHealthzWithDeadline(log logr.Logger, clock clock.Clock, cacheSyncWaiter cacheSyncWaiter, deadline time.Duration) healthz.Checker {
	var notSyncedSince *time.Time

	return func(_ *http.Request) error {
		if err := cacheSyncCheckFunc(cacheSyncWaiter); err != nil {
			if notSyncedSince == nil {
				notSyncedSince = ptr.To(clock.Now())
			}

			if clock.Now().Sub(*notSyncedSince) >= deadline {
				return err
			}

			log.WithName("cache-sync-healthz").Info("Cache sync check failed, but deadline not yet exceeded", "notSyncedSince", notSyncedSince, "deadline", deadline, "error", err)
			return nil
		}

		notSyncedSince = nil
		return nil
	}
}
