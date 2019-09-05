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
	"github.com/gardener/controller-manager-library/pkg/logger"
	"sync"
	"sync/atomic"
	"time"
)

var tickkeys = map[string]*string{}
var tickkeylock sync.Mutex

type Ticker struct {
	key  string
	last int64
	cur  chan struct{}
	old  chan struct{}
}

func tickkey(key string) *string {
	tickkeylock.Lock()
	defer tickkeylock.Unlock()
	k := tickkeys[key]
	if k == nil {
		k = &key
		tickkeys[key] = k
	}
	return k
}

func TickContext(ctx context.Context, key string) context.Context {
	ch := make(chan struct{})
	old := ticker(ctx, key)
	var t *Ticker
	if old != nil {
		t = &Ticker{key, 0, ch, old.cur}
	} else {
		t = &Ticker{key, 0, ch, nil}
	}
	t.start(ctx)
	return context.WithValue(ctx, tickkey(key), t)
}

func ticker(ctx context.Context, key string) *Ticker {
	t := ctx.Value(tickkey(key))
	if t != nil {
		return t.(*Ticker)
	}
	return nil
}

func Tick(ctx context.Context, key string) {
	ticker(ctx, key).cur <- struct{}{}
}

func CancelAfterInactivity(ctx context.Context, key string, d time.Duration) {
	ticker(ctx, key).CancelAfterInactivity(ctx, d)
}

func WaitForInactivity(ctx context.Context, key string, d time.Duration) {
	ticker(ctx, key).WaitForInactivity(ctx, d)
}

func (this *Ticker) start(ctx context.Context) {
	go func() {
		for {
			select {
			case t := <-this.cur:
				atomic.StoreInt64(&this.last, time.Now().Unix())
				if this.old != nil {
					this.old <- t
				}
			case <-ctx.Done():
				return
			}
		}
	}()
}

func (this *Ticker) CancelAfterInactivity(ctx context.Context, d time.Duration) {
	this.WaitForInactivity(ctx, d)
	logger.Infof("controller is beeing stopped after grace period")
	Cancel(ctx)
}

func (this *Ticker) WaitForInactivity(ctx context.Context, d time.Duration) {
	timer := time.NewTimer(d)
	logger.Infof("starting grace period timer for %s with %s", this.key, d)
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			last := atomic.LoadInt64(&this.last)
			if last <= 0 {
				return
			}
			now := time.Now().Unix()
			logger.Infof("check for grace period: last %s activity before %s",
				this.key, time.Duration(now-last)*time.Second)
			rest := d - time.Duration(now-last)*time.Second
			if rest <= 0 {
				return
			}
			logger.Infof("continue grace period for %s with %s", this.key, rest)
			timer.Reset(rest)

		}
	}
}
