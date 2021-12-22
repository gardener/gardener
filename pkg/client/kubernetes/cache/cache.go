// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"strings"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	runtimecache "sigs.k8s.io/controller-runtime/pkg/cache"
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

func processError(err error) error {
	if !IsAPIError(err) {
		// Return every other, unspecified error as a `CacheError` to allow users to follow up with a proper error handling.
		// For instance, a `Multinamespace` cache returns an unspecified error for unknown namespaces.
		// https://github.com/kubernetes-sigs/controller-runtime/blob/b5065bd85190e92864522fcc85aa4f6a3cce4f82/pkg/cache/multi_namespace_cache.go#L132
		return NewCacheError(err)
	}
	return err
}

// Get retrieves an obj for the given object key from the Kubernetes Cluster.
// Every non-API related error is returned as a `CacheError`.
func (c *aggregator) Get(ctx context.Context, key client.ObjectKey, obj client.Object) error {
	if err := c.cacheForObject(obj).Get(ctx, key, obj); err != nil {
		return processError(err)
	}
	return nil
}

// List retrieves list of objects for a given namespace and list options.
// Every non-API related error is returned as a `CacheError`.
func (c *aggregator) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	if err := c.cacheForObject(list).List(ctx, list, opts...); err != nil {
		return processError(err)
	}
	return nil
}

func (c *aggregator) GetInformer(ctx context.Context, obj client.Object) (cache.Informer, error) {
	return c.cacheForObject(obj).GetInformer(ctx, obj)
}

func (c *aggregator) GetInformerForKind(ctx context.Context, gvk schema.GroupVersionKind) (cache.Informer, error) {
	return c.cacheForKind(gvk).GetInformerForKind(ctx, gvk)
}

func (c *aggregator) Start(ctx context.Context) error {
	// NB: this function might leak goroutines, when the context for this aggregator cache is cancelled.
	// There is no way of waiting for caches to stop, so there's no point in waiting for the following
	// goroutines to finish, because there might still be goroutines running under the hood of caches.
	// However, this is not problematic, as long as the aggregator cache is not in any client set, that might
	// be invalidated during runtime.
	for gvk, cache := range c.gvkToCache {
		go func(gvk schema.GroupVersionKind, cache runtimecache.Cache) {
			err := cache.Start(ctx)
			if err != nil {
				logf.Log.Error(err, "Cache failed to start", "gvk", gvk.String())
			}
		}(gvk, cache)
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
