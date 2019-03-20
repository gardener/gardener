/*
 * Copyright 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 *
 */

package ctxutil

import (
	"context"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
)

var synckey = ""

func SyncContext(ctx context.Context) context.Context {
	var wg sync.WaitGroup
	return context.WithValue(ctx, &synckey, &wg)
}

func get_wg(ctx context.Context) *sync.WaitGroup {
	return ctx.Value(&synckey).(*sync.WaitGroup)
}

func SyncPointAdd(ctx context.Context) {
	get_wg(ctx).Add(1)
}

func SyncPointDone(ctx context.Context) {
	get_wg(ctx).Done()
}

func SyncPointWait(ctx context.Context, duration time.Duration) {
	if duration <= 0 {
		get_wg(ctx).Wait()
	} else {
		shutdown, cancel := context.WithCancel(context.Background())
		timer := time.NewTimer(duration)

		go func() {
			get_wg(ctx).Wait()
			cancel()
		}()
		select {
		case <-shutdown.Done():
		case <-timer.C:
			cancel()
		}
	}
}

func SyncPointRun(ctx context.Context, f func()) {
	SyncPointAdd(ctx)
	go func() {
		defer SyncPointDone(ctx)
		f()
	}()
}

func SyncPointRunAndCancelOnExit(ctx context.Context, f func()) {
	SyncPointAdd(ctx)
	go func() {
		defer Cancel(ctx)
		defer SyncPointDone(ctx)
		f()
	}()
}

func SyncPointRunUntilCancelled(ctx context.Context, f func()) {
	SyncPointRun(ctx, func() { wait.Until(f, time.Second, ctx.Done()) })
}
