// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package kubernetes

import (
	"context"
	"fmt"
	"time"

	kcache "github.com/gardener/gardener/pkg/client/kubernetes/cache"
	"github.com/gardener/gardener/pkg/logger"

	"golang.org/x/time/rate"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
)

const (
	defaultCacheResyncPeriod = 6 * time.Hour
)

// NewRuntimeClientWithCache creates a new client.client with the given config and options.
// The client uses a new cache, which will be started immediately using the given context.
func NewRuntimeClientWithCache(ctx context.Context, config *rest.Config, options client.Options, uncachedObjects ...client.Object) (client.Client, error) {
	if err := setClientOptionsDefaults(config, &options); err != nil {
		return nil, err
	}

	clientCache, err := NewRuntimeCache(config, cache.Options{
		Scheme: options.Scheme,
		Mapper: options.Mapper,
	})
	if err != nil {
		return nil, fmt.Errorf("could not create new client cache: %w", err)
	}

	runtimeClient, err := newRuntimeClientWithCache(config, options, clientCache, uncachedObjects...)
	if err != nil {
		return nil, err
	}

	go func() {
		if err := clientCache.Start(ctx); err != nil {
			logger.NewLogger(logger.ErrorLevel, "").Errorf("cache.Start returned error, which should never happen, ignoring.")
		}
	}()

	clientCache.WaitForCacheSync(ctx)

	return runtimeClient, nil
}

func newRuntimeClientWithCache(config *rest.Config, options client.Options, cache cache.Cache, uncachedObjects ...client.Object) (client.Client, error) {
	return cluster.DefaultNewClient(cache, config, options, uncachedObjects...)
}

func setClientOptionsDefaults(config *rest.Config, options *client.Options) error {
	if options.Mapper == nil {
		// default the client's REST mapper to a dynamic REST mapper (automatically rediscovers resources on NoMatchErrors)
		mapper, err := apiutil.NewDynamicRESTMapper(
			config,
			apiutil.WithLazyDiscovery,
			apiutil.WithLimiter(rate.NewLimiter(rate.Every(5*time.Second), 1)),
		)
		if err != nil {
			return fmt.Errorf("failed to create new DynamicRESTMapper: %w", err)
		}
		options.Mapper = mapper
	}

	return nil
}

// AggregatorCacheFunc returns a `cache.NewCacheFunc` which creates a cache that holds different cache implementations depending on the objects' GVKs.
func AggregatorCacheFunc(newCache cache.NewCacheFunc, typeToNewCache map[client.Object]cache.NewCacheFunc, scheme *runtime.Scheme) cache.NewCacheFunc {
	return func(config *rest.Config, options cache.Options) (cache.Cache, error) {
		if err := setCacheOptionsDefaults(&options); err != nil {
			return nil, err
		}

		fallbackCache, err := newCache(config, options)
		if err != nil {
			return nil, err
		}

		gvkToCache := make(map[schema.GroupVersionKind]cache.Cache)
		for object, fn := range typeToNewCache {
			gvk, err := apiutil.GVKForObject(object, scheme)
			if err != nil {
				return nil, err
			}

			cache, err := fn(config, options)
			if err != nil {
				return nil, err
			}

			gvkToCache[gvk] = cache
		}

		return kcache.NewAggregator(fallbackCache, gvkToCache, scheme), nil
	}
}

// NewRuntimeCache creates a new cache.Cache with the given config and options. It can be used
// for creating new controller-runtime clients with caches.
func NewRuntimeCache(config *rest.Config, options cache.Options) (cache.Cache, error) {
	if err := setCacheOptionsDefaults(&options); err != nil {
		return nil, err
	}

	return cache.New(config, options)
}

func setCacheOptionsDefaults(options *cache.Options) error {
	if options.Resync == nil {
		resync := defaultCacheResyncPeriod
		options.Resync = &resync
	}

	return nil
}
