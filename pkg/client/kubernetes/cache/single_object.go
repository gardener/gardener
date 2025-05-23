// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package cache

import (
	"context"
	"errors"
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
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ cache.Cache = &singleObject{}

type singleObject struct {
	parentCtx  context.Context
	log        logr.Logger
	restConfig *rest.Config
	newCache   cache.NewCacheFunc
	opts       func() cache.Options
	gvk        schema.GroupVersionKind

	started   bool
	startWait chan struct{} // startWait is a channel that is closed after the cache has been started

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
	gvk schema.GroupVersionKind,
	clock clock.Clock,
	maxIdleTime time.Duration,
	garbageCollectionInterval time.Duration,
) cache.Cache {
	return &singleObject{
		log:                       log,
		restConfig:                restConfig,
		newCache:                  newCache,
		opts:                      func() cache.Options { return opts },
		gvk:                       gvk,
		startWait:                 make(chan struct{}),
		store:                     make(map[client.ObjectKey]*objectCache),
		clock:                     clock,
		garbageCollectionInterval: garbageCollectionInterval,
		maxIdleTime:               maxIdleTime,
	}
}

func (s *singleObject) Start(ctx context.Context) error {
	if s.parentCtx != nil {
		return errors.New("the Start method cannot be called multiple times")
	}

	logger := s.log.WithName("garbage-collector").WithValues("interval", s.garbageCollectionInterval, "maxIdleTime", s.maxIdleTime)
	logger.V(1).Info("Starting")

	s.parentCtx = ctx
	s.started = true
	close(s.startWait)

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
					"lastAccessTime", ptr.Deref(lastAccessTime, time.Time{}),
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
	cache, err := s.getOrCreateCache(key)
	if err != nil {
		return err
	}
	return cache.Get(ctx, key, obj, opts...)
}

func (s *singleObject) GetInformer(ctx context.Context, obj client.Object, opts ...cache.InformerGetOption) (cache.Informer, error) {
	cache, err := s.getOrCreateCache(client.ObjectKeyFromObject(obj))
	if err != nil {
		return nil, err
	}
	return cache.GetInformer(ctx, obj, opts...)
}

func (s *singleObject) RemoveInformer(ctx context.Context, obj client.Object) error {
	cache, err := s.getOrCreateCache(client.ObjectKeyFromObject(obj))
	if err != nil {
		return err
	}
	return cache.RemoveInformer(ctx, obj)
}

func (s *singleObject) IndexField(ctx context.Context, obj client.Object, field string, extractValue client.IndexerFunc) error {
	cache, err := s.getOrCreateCache(client.ObjectKeyFromObject(obj))
	if err != nil {
		return err
	}
	return cache.IndexField(ctx, obj, field, extractValue)
}

func (s *singleObject) getOrCreateCache(key client.ObjectKey) (cache.Cache, error) {
	if !s.started {
		return nil, &cache.ErrCacheNotStarted{}
	}

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

		cache, err = s.createAndStartCache(log, key)
		if err != nil {
			return nil, err
		}
	} else {
		log.V(1).Info("Cache found, accessing it")
	}

	now := s.clock.Now().UTC()
	cache.lastAccessTime.Store(&now)
	log.V(1).Info("Cache was accessed, renewing last access time", "lastAccessTime", now)

	return cache.cache, nil
}

func (s *singleObject) createAndStartCache(log logr.Logger, key client.ObjectKey) (*objectCache, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	// Cache could have been created since last read lock was set, so we need to check again if the cache is available.
	if objCache, ok := s.store[key]; ok {
		log.V(1).Info("Cache created mid-air, no need to create it again")
		return objCache, nil
	}

	opts := s.opts()
	opts.DefaultNamespaces = map[string]cache.Config{key.Namespace: {}}
	opts.DefaultFieldSelector = fields.SelectorFromSet(fields.Set{metav1.ObjectNameField: key.Name})
	opts.ByObject = nil

	log.V(1).Info("Creating new cache")
	cache, err := s.newCache(s.restConfig, opts)
	if err != nil {
		return nil, fmt.Errorf("failed creating new cache: %w", err)
	}

	cacheCtx, cancel := context.WithCancel(s.parentCtx)

	go func() {
		log.V(1).Info("Starting new cache")
		if err := cache.Start(cacheCtx); err != nil {
			log.Error(err, "Cache failed to start")
		}
	}()

	waitForSyncCtx, waitForSyncCancel := context.WithTimeout(s.parentCtx, 5*time.Second)
	defer waitForSyncCancel()

	log.V(1).Info("Waiting for cache to be synced")
	if !cache.WaitForCacheSync(waitForSyncCtx) {
		cancel()
		return nil, errors.New("failed waiting for cache to be synced")
	}

	// The controller-runtime starts informers (which start the real WATCH on the API servers) only lazily with the
	// first call on the cache. Hence, after we have started the cache above and waited for its sync, in fact no
	// informer was started yet.
	// Hence, when we newly start a cache here, we need to perform a call on such cache to make it starting the
	// underlying informer. This is blocking because it implicitly waits for this informer to be synced. That's why we
	// use a context with a small timeout, especially to exit early in case of any permission errors.
	if _, err := cache.GetInformerForKind(waitForSyncCtx, s.gvk); err != nil {
		cancel()
		return nil, fmt.Errorf("failed getting informer: %w", err)
	}

	log.V(1).Info("Cache was synced successfully")
	s.store[key] = &objectCache{
		cache:  cache,
		cancel: cancel,
	}

	return s.store[key], nil
}

func (s *singleObject) WaitForCacheSync(ctx context.Context) bool {
	select {
	case <-s.startWait:
		return true
	case <-ctx.Done():
		return false
	}
}

func (s *singleObject) List(_ context.Context, _ client.ObjectList, _ ...client.ListOption) error {
	return errors.New("the List operation is not supported by singleObject cache")
}

func (s *singleObject) GetInformerForKind(_ context.Context, _ schema.GroupVersionKind, _ ...cache.InformerGetOption) (cache.Informer, error) {
	return nil, errors.New("the GetInformerForKind operation is not supported by singleObject cache")
}
