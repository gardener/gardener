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
	"errors"
	"fmt"
	"time"

	"github.com/gardener/gardener/pkg/logger"

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
)

const (
	defaultCacheResyncPeriod = 6 * time.Hour
)

// NewDirectClient creates a new client.Client which can be used to talk to the API directly (without a cache).
func NewDirectClient(config *rest.Config, options client.Options) (client.Client, error) {
	if err := setClientOptionsDefaults(config, &options); err != nil {
		return nil, err
	}

	return client.New(config, options)
}

// NewRuntimeClientWithCache creates a new client.client with the given config and options.
// The client uses a new cache, which will be started immediately using the given stop channel.
func NewRuntimeClientWithCache(config *rest.Config, options client.Options, stopCh <-chan struct{}) (client.Client, error) {
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

	runtimeClient, err := newRuntimeClientWithCache(config, options, clientCache)
	if err != nil {
		return nil, err
	}

	go func() {
		if err := clientCache.Start(stopCh); err != nil {
			logger.NewLogger(string(logrus.ErrorLevel)).Errorf("cache.Start returned error, which should never happen, ignoring.")
		}
	}()

	clientCache.WaitForCacheSync(stopCh)

	return runtimeClient, nil
}

func newRuntimeClientWithCache(config *rest.Config, options client.Options, cache cache.Cache) (client.Client, error) {
	c, err := client.New(config, options)
	if err != nil {
		return nil, err
	}

	return &client.DelegatingClient{
		Reader: &client.DelegatingReader{
			CacheReader:  cache,
			ClientReader: c,
		},
		Writer:       c,
		StatusClient: c,
	}, nil
}

func setClientOptionsDefaults(config *rest.Config, options *client.Options) error {
	if options.Mapper == nil {
		// default the client's REST mapper to a dynamic REST mapper (automatically rediscovers resources on NoMatchErrors)
		mapper, err := apiutil.NewDynamicRESTMapper(config, apiutil.WithLazyDiscovery)
		if err != nil {
			return fmt.Errorf("failed to create new DynamicRESTMapper: %w", err)
		}
		options.Mapper = mapper
	}

	return nil
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

// NewDirectClientFromSecret creates a new controller runtime Client struct for a given secret.
func NewDirectClientFromSecret(secret *corev1.Secret, fns ...ConfigFunc) (client.Client, error) {
	if kubeconfig, ok := secret.Data[KubeConfig]; ok {
		return NewDirectClientFromBytes(kubeconfig, fns...)
	}
	return nil, errors.New("no valid kubeconfig found")
}

// NewDirectClientFromBytes creates a new controller runtime Client struct for a given kubeconfig byte slice.
func NewDirectClientFromBytes(kubeconfig []byte, fns ...ConfigFunc) (client.Client, error) {
	clientConfig, err := clientcmd.NewClientConfigFromBytes(kubeconfig)
	if err != nil {
		return nil, err
	}

	if err := validateClientConfig(clientConfig); err != nil {
		return nil, err
	}

	config, err := clientConfig.ClientConfig()
	if err != nil {
		return nil, err
	}

	opts := append([]ConfigFunc{WithRESTConfig(config)}, fns...)
	return NewDirectClientWithConfig(opts...)
}

// NewDirectClientWithConfig returns a new controller runtime client from a config.
func NewDirectClientWithConfig(fns ...ConfigFunc) (client.Client, error) {
	conf := &config{}
	for _, f := range fns {
		if err := f(conf); err != nil {
			return nil, err
		}
	}
	return NewDirectClient(conf.restConfig, conf.clientOptions)
}
