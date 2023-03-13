// Copyright (c) 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cache

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/rest"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ cache.Cache = &singleObject{}

type singleObject struct {
	log      logr.Logger
	config   *rest.Config
	newCache cache.NewCacheFunc
	opts     func() cache.Options

	lock             sync.RWMutex
	objectKeyToCache map[client.ObjectKey]*objectCache

	clock                     clock.Clock
	garbageCollectionInterval time.Duration
	maxIdleTime               time.Duration
}

type objectCache struct {
	cache          cache.Cache
	cancel         context.CancelFunc
	lastAccessTime atomic.Value
}

// NewSingleObject creates a new instance of the singleObject cache.Cache implementation.
// This cache maintains a separate cache per `client.ObjectKey` and invalidates them when not accessed for the
// given `maxIdleTime`. A new cache for a particular object is added or re-added as soon as the caches `Get()` function is invoked.
// Please note, that object types are not differentiated by this cache (only object keys), i.e. it must not be used with mixed GVKs.
func NewSingleObject(log logr.Logger, config *rest.Config, opts cache.Options, newCache cache.NewCacheFunc, clock clock.Clock, maxIdleTime, garbageCollectionInterval time.Duration) cache.Cache {
	return &singleObject{
		log:                       log,
		config:                    config,
		newCache:                  newCache,
		opts:                      func() cache.Options { return opts },
		objectKeyToCache:          make(map[client.ObjectKey]*objectCache),
		clock:                     clock,
		garbageCollectionInterval: garbageCollectionInterval,
		maxIdleTime:               maxIdleTime,
	}
}

func (s *singleObject) Start(ctx context.Context) error {
	wait.Until(func() {
		s.lock.Lock()
		defer s.lock.Unlock()

		for key, objectCache := range s.objectKeyToCache {
			lastAccessTime, ok := (objectCache.lastAccessTime.Load()).(*time.Time)
			if !ok || lastAccessTime == nil || !s.clock.Now().After(lastAccessTime.Add(s.maxIdleTime)) {
				continue
			}

			// close cache for object because it was not accessed for at least the max idle time
			objectCache.cancel()
			delete(s.objectKeyToCache, key)
		}
	}, s.garbageCollectionInterval, ctx.Done())
	return nil
}

func (s *singleObject) WaitForCacheSync(_ context.Context) bool {
	return true
}

func (s *singleObject) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	cache, err := s.getOrCreateCache(ctx, key)
	if err != nil {
		return err
	}

	now := s.clock.Now()
	cache.lastAccessTime.Store(&now)

	return cache.cache.Get(ctx, key, obj, opts...)
}

func (s *singleObject) getOrCreateCache(ctx context.Context, key client.ObjectKey) (*objectCache, error) {
	found, objectCache := func() (bool, *objectCache) {
		s.lock.RLock()
		defer s.lock.RUnlock()
		objectCache, ok := s.objectKeyToCache[key]
		return ok, objectCache
	}()

	if found {
		s.log.Info("Cache found", "key", key)
		return objectCache, nil
	}

	return s.createCache(ctx, key)
}

func (s *singleObject) createCache(ctx context.Context, key client.ObjectKey) (*objectCache, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	// Cache could have been created since last read lock was set, so we need to check again if the cache is available.
	if objectCache, ok := s.objectKeyToCache[key]; ok {
		s.log.Info("Cache found before creation", "key", key)
		return objectCache, nil
	}

	opts := s.opts()
	opts.Namespace = key.Namespace
	opts.DefaultSelector = cache.ObjectSelector{Field: fields.SelectorFromSet(fields.Set{metav1.ObjectNameField: key.Name})}
	opts.SelectorsByObject = nil

	s.log.Info("Creating new cache", "key", key)
	c, err := s.newCache(s.config, opts)
	if err != nil {
		return nil, err
	}

	cacheCtx, cancel := context.WithCancel(ctx)

	go func() {
		s.log.Info("Starting new cache", "key", key)
		if err := c.Start(cacheCtx); err != nil {
			s.log.Error(err, "Cache failed to start", "key", key)
		}
	}()

	if !c.WaitForCacheSync(ctx) {
		cancel()
		return nil, fmt.Errorf("failed waiting for cache to be synced")
	}

	s.objectKeyToCache[key] = &objectCache{
		cache:  c,
		cancel: cancel,
	}
	return s.objectKeyToCache[key], nil
}

func (s *singleObject) GetInformer(ctx context.Context, obj client.Object) (cache.Informer, error) {
	cache, err := s.getOrCreateCache(ctx, client.ObjectKeyFromObject(obj))
	if err != nil {
		return nil, err
	}
	return cache.cache.GetInformer(ctx, obj)
}

func (s *singleObject) IndexField(ctx context.Context, obj client.Object, field string, extractValue client.IndexerFunc) error {
	cache, err := s.getOrCreateCache(ctx, client.ObjectKeyFromObject(obj))
	if err != nil {
		return err
	}
	return cache.cache.IndexField(ctx, obj, field, extractValue)
}

func (s *singleObject) List(_ context.Context, _ client.ObjectList, _ ...client.ListOption) error {
	return fmt.Errorf("the List operation is not supported by singleObject cache")
}

func (s *singleObject) GetInformerForKind(_ context.Context, _ schema.GroupVersionKind) (cache.Informer, error) {
	return nil, fmt.Errorf("the GetInformerForKind operation is not supported by singleObject cache")
}
