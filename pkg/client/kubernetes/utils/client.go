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

package utils

import (
	"context"
	"strings"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kubernetesscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

// NewClientFuncWithSpecificallyCachedReader returns a manager.NewClientFunc that creates a new client.Client that either
// - reads directly from the API server by default but reads the specified objects from the cache (readSpecifiedFromCache=true)
// - or reads from the cache by default but reads the specified objects directly from the API server (readSpecifiedFromCache=true).
func NewClientFuncWithSpecificallyCachedReader(readSpecifiedFromCache bool, specifiedObjects ...runtime.Object) manager.NewClientFunc {
	return func(cache cache.Cache, config *rest.Config, options client.Options) (client.Client, error) {
		return NewClientWithSpecificallyCachedReader(cache, config, options, readSpecifiedFromCache, specifiedObjects...)
	}
}

// NewClientFuncWithDisabledCacheFor returns a manager.NewClientFunc that creates a new client.Client that reads from
// the cache by default but reads the specified objects directly from the API server.
func NewClientFuncWithDisabledCacheFor(directlyReadingObjects ...runtime.Object) manager.NewClientFunc {
	return NewClientFuncWithSpecificallyCachedReader(false, directlyReadingObjects...)
}

// NewClientFuncWithEnabledCacheFor returns a manager.NewClientFunc that creates a new client.Client that reads directly
// from the API server by default but reads the specified objects from the cache.
func NewClientFuncWithEnabledCacheFor(cachedObjects ...runtime.Object) manager.NewClientFunc {
	return NewClientFuncWithSpecificallyCachedReader(true, cachedObjects...)
}

// NewClientWithSpecificallyCachedReader creates a new client.Client that either
// - reads directly from the API server by default but reads the specified objects from the cache (readSpecifiedFromCache=true)
// - or reads from the cache by default but reads the specified objects directly from the API server (readSpecifiedFromCache=false).
func NewClientWithSpecificallyCachedReader(cache cache.Cache, config *rest.Config, options client.Options, readSpecifiedFromCache bool, specifiedObjects ...runtime.Object) (client.Client, error) {
	// create the default Client
	c, err := client.New(config, options)
	if err != nil {
		return nil, err
	}

	// create cache Reader that decides which objects to read from the cache and which to read directly from the API server
	cacheReader, err := NewSpecificallyCachedReaderFor(
		cache,
		c,
		options.Scheme,
		readSpecifiedFromCache,
		specifiedObjects...,
	)
	if err != nil {
		return nil, err
	}

	return &client.DelegatingClient{
		Reader: &client.DelegatingReader{
			CacheReader:  cacheReader,
			ClientReader: c,
		},
		Writer:       c,
		StatusClient: c,
	}, nil
}

// NewSpecificallyCachedReaderFor creates a new client.Reader that either
// - reads directly from the API server by default but reads the specified objects from the cache (readSpecifiedFromCache=true)
// - or reads from the cache by default but reads the specified objects directly from the API server (readSpecifiedFromCache=false).
func NewSpecificallyCachedReaderFor(cacheReader, clientReader client.Reader, scheme *runtime.Scheme, readSpecifiedFromCache bool, objectKinds ...runtime.Object) (client.Reader, error) {
	if scheme == nil {
		scheme = kubernetesscheme.Scheme
	}

	delegatingRule, err := newDelegationRule(scheme, readSpecifiedFromCache, objectKinds...)
	if err != nil {
		return nil, err
	}

	return &specificallyCachedReader{
		shouldReadObjectFromCache: delegatingRule,
		cacheReader:               cacheReader,
		clientReader:              clientReader,
	}, nil
}

// NewReaderWithDisabledCacheFor creates a new client.Reader that reads from the cache by default but reads the
// specified objects directly from the API server.
func NewReaderWithDisabledCacheFor(cacheReader, clientReader client.Reader, scheme *runtime.Scheme, directlyReadingObjects ...runtime.Object) (client.Reader, error) {
	return NewSpecificallyCachedReaderFor(cacheReader, clientReader, scheme, false, directlyReadingObjects...)
}

// NewReaderWithEnabledCacheFor creates a new client.Reader that reads directly from the API server by default but reads
// the specified objects from the cache.
func NewReaderWithEnabledCacheFor(cacheReader, clientReader client.Reader, scheme *runtime.Scheme, cachedObjects ...runtime.Object) (client.Reader, error) {
	return NewSpecificallyCachedReaderFor(cacheReader, clientReader, scheme, true, cachedObjects...)
}

// specificallyCachedReader implements client.Reader and delegates calls to the cache or direct client according to the
// given delegationRule.
type specificallyCachedReader struct {
	shouldReadObjectFromCache delegationRule

	cacheReader  client.Reader
	clientReader client.Reader
}

// delegationRule is a function that decides whether a given object should be read from the cache or directly from the
// API server.
type delegationRule func(obj runtime.Object) (readFromCache bool, err error)

// newDelegationRule returns a delegationRule, that will either
// - only allow to read the specified object kinds from the cache (readSpecifiedFromCache=true)
// - or only read the specified object kinds directly from the API server (readSpecifiedFromCache=false)
func newDelegationRule(scheme *runtime.Scheme, readSpecifiedFromCache bool, objs ...runtime.Object) (delegationRule, error) {
	specifiedGKs := make(map[schema.GroupKind]struct{})

	for _, obj := range objs {
		gvk, err := apiutil.GVKForObject(obj, scheme)
		if err != nil {
			return nil, err
		}
		specifiedGKs[gvk.GroupKind()] = struct{}{}
	}

	return func(obj runtime.Object) (readFromCache bool, err error) {
		gvk, err := apiutil.GVKForObject(obj, scheme)
		if err != nil {
			return false, err
		}

		if strings.HasSuffix(gvk.Kind, "List") && meta.IsListType(obj) {
			// if this is a list, treat it as a request for the item's resource
			gvk.Kind = strings.TrimSuffix(gvk.Kind, "List")
		}

		_, ok := specifiedGKs[gvk.GroupKind()]
		// read from cache if GroupKind was specified and readSpecifiedFromCache=true
		//   or GroupKind was not specified and readSpecifiedFromCache=false
		return ok == readSpecifiedFromCache, nil
	}, nil
}

// Get implements client.Reader by delegating calls either to the cache or the direct client based on the decision of
// the delegationRule.
func (s *specificallyCachedReader) Get(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
	useCache, err := s.shouldReadObjectFromCache(obj)
	if err != nil {
		return err
	}
	if useCache {
		return s.cacheReader.Get(ctx, key, obj)
	}
	return s.clientReader.Get(ctx, key, obj)
}

// List implements client.Reader by delegating calls either to the cache or the direct client based on the decision of
// the delegationRule.
func (s *specificallyCachedReader) List(ctx context.Context, list runtime.Object, opts ...client.ListOption) error {
	useCache, err := s.shouldReadObjectFromCache(list)
	if err != nil {
		return err
	}
	if useCache {
		return s.cacheReader.List(ctx, list, opts...)
	}
	return s.clientReader.List(ctx, list, opts...)
}
