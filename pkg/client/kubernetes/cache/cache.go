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

	"github.com/gardener/gardener/pkg/logger"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	runtimecache "sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ cache.Cache = &Aggregator{}

// Aggregator is a cache that can hold different cache implementations depending on the objects' GVKs.
type Aggregator struct {
	fallback   cache.Cache
	gvkToCache map[schema.GroupVersionKind]cache.Cache
	scheme     *runtime.Scheme
}

func (c *Aggregator) cacheForObject(obj runtime.Object) cache.Cache {
	gvks, _, err := c.scheme.ObjectKinds(obj)
	if err != nil || len(gvks) != 1 {
		return c.fallback
	}

	return c.cacheForKind(gvks[0])
}

func (c *Aggregator) cacheForKind(kind schema.GroupVersionKind) cache.Cache {
	gvk := kind
	if strings.HasSuffix(gvk.Kind, "List") {
		gvk.Kind = gvk.Kind[:len(gvk.Kind)-4]
	}

	cache, ok := c.gvkToCache[gvk]
	if !ok {
		cache = c.fallback
	}
	return cache
}

func (c *Aggregator) Get(ctx context.Context, key client.ObjectKey, obj client.Object) error {
	err := c.cacheForObject(obj).Get(ctx, key, obj)
	if err != nil && !APIError(err) {
		return NewCacheError(err)
	}
	return err
}

func (c *Aggregator) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	err := c.cacheForObject(list).List(ctx, list, opts...)
	if err != nil && !APIError(err) {
		return NewCacheError(err)
	}
	return err
}

func (c *Aggregator) GetInformer(ctx context.Context, obj client.Object) (cache.Informer, error) {
	return c.cacheForObject(obj).GetInformer(ctx, obj)
}

func (c *Aggregator) GetInformerForKind(ctx context.Context, gvk schema.GroupVersionKind) (cache.Informer, error) {
	return c.cacheForKind(gvk).GetInformerForKind(ctx, gvk)
}

func (c *Aggregator) Start(ctx context.Context) error {
	for gvk, cache := range c.gvkToCache {
		go func(gvk schema.GroupVersionKind, cache runtimecache.Cache) {
			err := cache.Start(ctx)
			if err != nil {
				logger.Logger.Errorf("cache failed to start for %q: %v", gvk.String(), err)
			}
		}(gvk, cache)
	}
	go func() {
		if err := c.fallback.Start(ctx); err != nil {
			logger.Logger.Error(err)
		}
	}()
	<-ctx.Done()

	return nil
}

func (c *Aggregator) WaitForCacheSync(ctx context.Context) bool {
	synced := true
	for _, cache := range c.gvkToCache {
		if s := cache.WaitForCacheSync(ctx); !s {
			synced = s
		}
	}
	synced = synced && c.fallback.WaitForCacheSync(ctx)
	return synced
}

func (c *Aggregator) IndexField(ctx context.Context, obj client.Object, field string, extractValue client.IndexerFunc) error {
	return c.cacheForObject(obj).IndexField(ctx, obj, field, extractValue)
}

// NewAggregator creates a new instance of an aggregated cache.
func NewAggregator(fallback cache.Cache, gvkToCache map[schema.GroupVersionKind]cache.Cache, scheme *runtime.Scheme) *Aggregator {
	return &Aggregator{
		fallback:   fallback,
		gvkToCache: gvkToCache,
		scheme:     scheme,
	}
}
