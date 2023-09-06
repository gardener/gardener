// Copyright 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"golang.org/x/time/rate"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
	"k8s.io/utils/clock"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	kubernetescache "github.com/gardener/gardener/pkg/client/kubernetes/cache"
	thirdpartyapiutil "github.com/gardener/gardener/third_party/controller-runtime/pkg/apiutil"
)

const (
	defaultCacheSyncPeriod = 6 * time.Hour
)

// NewRuntimeCache creates a new cache.Cache with the given config and options. It can be used
// for creating new controller-runtime clients with caches.
func NewRuntimeCache(config *rest.Config, options cache.Options) (cache.Cache, error) {
	if err := setCacheOptionsDefaults(&options); err != nil {
		return nil, err
	}

	return cache.New(config, options)
}

func setCacheOptionsDefaults(options *cache.Options) error {
	if options.SyncPeriod == nil {
		options.SyncPeriod = pointer.Duration(defaultCacheSyncPeriod)
	}

	return nil
}

func setClientOptionsDefaults(config *rest.Config, options *client.Options) error {
	if options.Mapper == nil {
		// default the client's REST mapper to a dynamic REST mapper (automatically rediscovers resources on NoMatchErrors)
		// TODO(ary1992): The new rest mapper implementation doesn't return a NoKindMatchError but a ErrGroupDiscoveryFailed
		// when an API GroupVersion is not present in the cluster. Remove the old restmapper usage once the upstream issue
		// (https://github.com/kubernetes-sigs/controller-runtime/pull/2425) is fixed.
		mapper, err := thirdpartyapiutil.NewDynamicRESTMapper(
			config,
			thirdpartyapiutil.WithLazyDiscovery,
			thirdpartyapiutil.WithLimiter(rate.NewLimiter(rate.Every(5*time.Second), 1)),
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
