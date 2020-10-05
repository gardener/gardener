// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	baseconfig "k8s.io/component-base/config"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Config carries options for new ClientSets.
type Config struct {
	clientOptions       client.Options
	restConfig          *rest.Config
	cacheResync         *time.Duration
	disableCachedClient bool
	cacheReaderOptions  *cacheReaderOptions
}

// cacheReaderOptions configures the specificallyCachedReader
type cacheReaderOptions struct {
	readSpecifiedFromCache    bool
	specificallyCachedObjects []runtime.Object
}

// NewConfig returns a new Config with an empty REST config to allow testing ConfigFuncs without exporting
// the fields of the Config type.
func NewConfig() *Config {
	return &Config{restConfig: &rest.Config{}}
}

// ConfigFunc is a function that mutates a Config struct.
// It implements the functional options pattern. See
// https://github.com/tmrts/go-patterns/blob/master/idiom/functional-options.md.
type ConfigFunc func(config *Config) error

// WithRESTConfig returns a ConfigFunc that sets the passed rest.Config on the Config object.
func WithRESTConfig(restConfig *rest.Config) ConfigFunc {
	return func(config *Config) error {
		config.restConfig = restConfig
		return nil
	}
}

// WithClientConnectionOptions returns a ConfigFunc that transfers settings from
// the passed ClientConnectionConfiguration.
// The kubeconfig location in ClientConnectionConfiguration is disregarded, though!
func WithClientConnectionOptions(cfg baseconfig.ClientConnectionConfiguration) ConfigFunc {
	return func(config *Config) error {
		if config.restConfig == nil {
			return errors.New("REST config must be set before setting connection options")
		}
		config.restConfig.Burst = int(cfg.Burst)
		config.restConfig.QPS = cfg.QPS
		config.restConfig.AcceptContentTypes = cfg.AcceptContentTypes
		config.restConfig.ContentType = cfg.ContentType
		return nil
	}
}

// WithClientOptions returns a ConfigFunc that sets the passed Options on the Config object.
func WithClientOptions(opt client.Options) ConfigFunc {
	return func(config *Config) error {
		config.clientOptions = opt
		return nil
	}
}

// WithCacheResyncPeriod returns a ConfigFunc that set the client's cache's resync period to the given duration.
func WithCacheResyncPeriod(resync time.Duration) ConfigFunc {
	return func(config *Config) error {
		config.cacheResync = &resync
		return nil
	}
}

// WithDisabledCachedClient disables the cache in the controller-runtime client, so Client() will be equivalent to
// DirectClient().
func WithDisabledCachedClient() ConfigFunc {
	return func(config *Config) error {
		config.disableCachedClient = true
		return nil
	}
}

// WithDisabledCacheFor disables the cached client for the specified objects' GroupKinds.
func WithDisabledCacheFor(objects ...runtime.Object) ConfigFunc {
	return func(config *Config) error {
		config.cacheReaderOptions = &cacheReaderOptions{
			readSpecifiedFromCache:    false,
			specificallyCachedObjects: objects,
		}
		return nil
	}
}

// WithEnabledCacheFor enables the cached client only for the specified objects' GroupKinds.
func WithEnabledCacheFor(objects ...runtime.Object) ConfigFunc {
	return func(config *Config) error {
		config.cacheReaderOptions = &cacheReaderOptions{
			readSpecifiedFromCache:    true,
			specificallyCachedObjects: objects,
		}
		return nil
	}
}
