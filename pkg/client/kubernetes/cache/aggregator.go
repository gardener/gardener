// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package cache

import (
	"context"
	"strings"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var _ cache.Cache = &aggregator{}

// aggregator is a cache that can hold different cache implementations depending on the objects' GVKs.
type aggregator struct {
	fallbackCache cache.Cache
	gvkToCache    map[schema.GroupVersionKind]cache.Cache
	scheme        *runtime.Scheme
}

func (c *aggregator) cacheForObject(obj runtime.Object) cache.Cache {
	gvks, _, err := c.scheme.ObjectKinds(obj)
	if err != nil || len(gvks) != 1 {
		return c.fallbackCache
	}

	return c.cacheForKind(gvks[0])
}

func (c *aggregator) cacheForKind(kind schema.GroupVersionKind) cache.Cache {
	gvk := kind
	gvk.Kind = strings.TrimSuffix(gvk.Kind, "List")

	cache, ok := c.gvkToCache[gvk]
	if !ok {
		cache = c.fallbackCache
	}
	return cache
}

// Get retrieves an obj for the given object key from the Kubernetes Cluster.
// Every non-API related error is returned as a `CacheError`.
func (c *aggregator) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	return c.cacheForObject(obj).Get(ctx, key, obj, opts...)
}

// List retrieves list of objects for a given namespace and list options.
// Every non-API related error is returned as a `CacheError`.
func (c *aggregator) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	return c.cacheForObject(list).List(ctx, list, opts...)
}

func (c *aggregator) GetInformer(ctx context.Context, obj client.Object, opts ...cache.InformerGetOption) (cache.Informer, error) {
	return c.cacheForObject(obj).GetInformer(ctx, obj, opts...)
}

func (c *aggregator) RemoveInformer(ctx context.Context, obj client.Object) error {
	return c.cacheForObject(obj).RemoveInformer(ctx, obj)
}

func (c *aggregator) GetInformerForKind(ctx context.Context, gvk schema.GroupVersionKind, opts ...cache.InformerGetOption) (cache.Informer, error) {
	return c.cacheForKind(gvk).GetInformerForKind(ctx, gvk, opts...)
}

func (c *aggregator) Start(ctx context.Context) error {
	// NB: this function might leak goroutines, when the context for this aggregator cache is cancelled.
	// There is no way of waiting for caches to stop, so there's no point in waiting for the following
	// goroutines to finish, because there might still be goroutines running under the hood of caches.
	// However, this is not problematic, as long as the aggregator cache is not in any client set, that might
	// be invalidated during runtime.
	for gvk, runtimecache := range c.gvkToCache {
		go func(gvk schema.GroupVersionKind, runtimecache cache.Cache) {
			err := runtimecache.Start(ctx)
			if err != nil {
				logf.Log.Error(err, "Cache failed to start", "gvk", gvk.String())
			}
		}(gvk, runtimecache)
	}

	go func() {
		if err := c.fallbackCache.Start(ctx); err != nil {
			logf.Log.Error(err, "Fallback cache failed to start")
		}
	}()
	<-ctx.Done()

	return nil
}

func (c *aggregator) WaitForCacheSync(ctx context.Context) bool {
	for _, cache := range c.gvkToCache {
		if !cache.WaitForCacheSync(ctx) {
			return false
		}
	}
	return c.fallbackCache.WaitForCacheSync(ctx)
}

func (c *aggregator) IndexField(ctx context.Context, obj client.Object, field string, extractValue client.IndexerFunc) error {
	return c.cacheForObject(obj).IndexField(ctx, obj, field, extractValue)
}

// NewAggregator creates a new instance of an aggregated cache.
func NewAggregator(fallbackCache cache.Cache, gvkToCache map[schema.GroupVersionKind]cache.Cache, scheme *runtime.Scheme) cache.Cache {
	return &aggregator{
		fallbackCache: fallbackCache,
		gvkToCache:    gvkToCache,
		scheme:        scheme,
	}
}
