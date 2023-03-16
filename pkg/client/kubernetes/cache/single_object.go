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

	"github.com/gardener/gardener/pkg/utils"
)

var _ cache.Cache = &singleObject{}

type singleObject struct {
	log        logr.Logger
	restConfig *rest.Config
	newCache   cache.NewCacheFunc
	opts       func() cache.Options

	lock  sync.RWMutex
	store map[client.ObjectKey]*objectCache

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
// given `maxIdleTime`. A new cache for a particular object is added or re-added as soon as the caches `Get()` function
// is invoked. Please note that object types are not differentiated by this cache (only object keys), i.e. it must not
// be used with mixed GVKs.
func NewSingleObject(
	log logr.Logger,
	restConfig *rest.Config,
	newCache cache.NewCacheFunc,
	opts cache.Options,
	clock clock.Clock,
	maxIdleTime time.Duration,
	garbageCollectionInterval time.Duration,
) cache.Cache {
	return &singleObject{
		log:                       log,
		restConfig:                restConfig,
		newCache:                  newCache,
		opts:                      func() cache.Options { return opts },
		store:                     make(map[client.ObjectKey]*objectCache),
		clock:                     clock,
		garbageCollectionInterval: garbageCollectionInterval,
		maxIdleTime:               maxIdleTime,
	}
}

func (s *singleObject) Start(ctx context.Context) error {
	logger := s.log.WithName("garbage-collector").WithValues("interval", s.garbageCollectionInterval, "maxIdleTime", s.maxIdleTime)
	logger.V(1).Info("Starting")

	wait.Until(func() {
		s.lock.Lock()
		defer s.lock.Unlock()

		for key, objCache := range s.store {
			var (
				lastAccessTime, ok = (objCache.lastAccessTime.Load()).(*time.Time)
				now                = s.clock.Now().UTC()
				log                = logger.WithValues(
					"key", key,
					"now", now,
					"lastAccessTime", utils.TimePtrDeref(lastAccessTime, time.Time{}),
				)
			)

			if !ok || lastAccessTime == nil || !now.After(lastAccessTime.Add(s.maxIdleTime)) {
				log.V(1).Info("Cache was accessed recently, no need to close it")
				continue
			}

			// close cache for object because it was not accessed for at least the max idle time
			log.V(1).Info("Cache was not accessed recently, closing it")
			objCache.cancel()
			delete(s.store, key)
		}
	}, s.garbageCollectionInterval, ctx.Done())

	logger.V(1).Info("Stopping")
	return nil
}

func (s *singleObject) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	cache, err := s.getOrCreateCache(ctx, key)
	if err != nil {
		return err
	}
	return cache.Get(ctx, key, obj, opts...)
}

func (s *singleObject) GetInformer(ctx context.Context, obj client.Object) (cache.V(1).Informer, error) {
	cache, err := s.getOrCreateCache(ctx, client.ObjectKeyFromObject(obj))
	if err != nil {
		return nil, err
	}
	return cache.GetInformer(ctx, obj)
}

func (s *singleObject) IndexField(ctx context.Context, obj client.Object, field string, extractValue client.IndexerFunc) error {
	cache, err := s.getOrCreateCache(ctx, client.ObjectKeyFromObject(obj))
	if err != nil {
		return err
	}
	return cache.IndexField(ctx, obj, field, extractValue)
}

func (s *singleObject) getOrCreateCache(ctx context.Context, key client.ObjectKey) (cache.Cache, error) {
	log := s.log.WithValues("key", key)

	cache, found := func() (*objectCache, bool) {
		s.lock.RLock()
		defer s.lock.RUnlock()
		cache, found := s.store[key]
		return cache, found
	}()

	if !found {
		log.V(1).Info("Cache not found, creating it")

		var err error
		cache, err = s.createCache(ctx, key)
		if err != nil {
			return nil, err
		}
	}

	now := s.clock.Now().UTC()
	cache.lastAccessTime.Store(&now)
	log.V(1).Info("Cache was accessed, renewing last access time", "lastAccessTime", now)

	return cache.cache, nil
}

func (s *singleObject) createCache(ctx context.Context, key client.ObjectKey) (*objectCache, error) {
	log := s.log.WithValues("key", key)

	s.lock.Lock()
	defer s.lock.Unlock()

	// Cache could have been created since last read lock was set, so we need to check again if the cache is available.
	if objCache, ok := s.store[key]; ok {
		log.V(1).Info("Cache created mid-air, no need to create it again")
		return objCache, nil
	}

	opts := s.opts()
	opts.Namespace = key.Namespace
	opts.DefaultSelector = cache.ObjectSelector{Field: fields.SelectorFromSet(fields.Set{metav1.ObjectNameField: key.Name})}
	opts.SelectorsByObject = nil

	log.V(1).Info("Creating new cache")
	cache, err := s.newCache(s.restConfig, opts)
	if err != nil {
		return nil, fmt.Errorf("failed creating new cache: %w", err)
	}

	cacheCtx, cancel := context.WithCancel(ctx)

	go func() {
		log.V(1).Info("Starting new cache")
		if err := cache.Start(cacheCtx); err != nil {
			log.Error(err, "Cache failed to start")
		}
	}()

	log.V(1).Info("Waiting for cache to be synced")
	if !cache.WaitForCacheSync(ctx) {
		cancel()
		return nil, fmt.Errorf("failed waiting for cache to be synced")
	}

	log.V(1).Info("Cache was synced successfully")
	s.store[key] = &objectCache{
		cache:  cache,
		cancel: cancel,
	}

	return s.store[key], nil
}

func (s *singleObject) WaitForCacheSync(_ context.Context) bool {
	return true
}

func (s *singleObject) List(_ context.Context, _ client.ObjectList, _ ...client.ListOption) error {
	return fmt.Errorf("the List operation is not supported by singleObject cache")
}

func (s *singleObject) GetInformerForKind(_ context.Context, _ schema.GroupVersionKind) (cache.V(1).Informer, error) {
	return nil, fmt.Errorf("the GetInformerForKind operation is not supported by singleObject cache")
}
