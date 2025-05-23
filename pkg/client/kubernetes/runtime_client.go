// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package kubernetes

import (
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
	"k8s.io/utils/clock"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	kubernetescache "github.com/gardener/gardener/pkg/client/kubernetes/cache"
)

const (
	defaultCacheSyncPeriod = 6 * time.Hour
)

// NewRuntimeCache creates a new cache.Cache with the given config and options. It can be used
// for creating new controller-runtime clients with caches.
func NewRuntimeCache(config *rest.Config, options cache.Options) (cache.Cache, error) {
	setCacheOptionsDefaults(&options)

	return cache.New(config, options)
}

func setCacheOptionsDefaults(options *cache.Options) {
	if options.SyncPeriod == nil {
		options.SyncPeriod = ptr.To(defaultCacheSyncPeriod)
	}
}

func setClientOptionsDefaults(config *rest.Config, options *client.Options) error {
	if options.Mapper == nil {
		httpClient, err := rest.HTTPClientFor(config)
		if err != nil {
			return fmt.Errorf("failed to get HTTP client for config: %w", err)
		}

		mapper, err := apiutil.NewDynamicRESTMapper(
			config,
			httpClient,
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
		setCacheOptionsDefaults(&options)

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

		return kubernetescache.NewAggregator(fallbackCache, gvkToCache, scheme), nil
	}
}

// SingleObjectCacheFunc returns a cache.NewCacheFunc for the SingleObject implementation.
func SingleObjectCacheFunc(log logr.Logger, scheme *runtime.Scheme, obj client.Object) cache.NewCacheFunc {
	return func(restConfig *rest.Config, options cache.Options) (cache.Cache, error) {
		gvk, err := apiutil.GVKForObject(obj, scheme)
		if err != nil {
			return nil, err
		}

		logger := log.
			WithName("single-object-cache").
			WithValues("groupVersion", gvk.GroupVersion().String(), "kind", gvk.Kind)

		return kubernetescache.NewSingleObject(logger, restConfig, cache.New, options, gvk, clock.RealClock{}, 10*time.Minute, time.Minute), nil
	}
}
