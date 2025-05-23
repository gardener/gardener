// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package kubernetes

import (
	"errors"
	"time"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	componentbaseconfigv1alpha1 "k8s.io/component-base/config/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Config carries options for new ClientSets.
type Config struct {
	runtimeAPIReader client.Reader
	runtimeClient    client.Client
	runtimeCache     cache.Cache

	newRuntimeCache   cache.NewCacheFunc
	clientOptions     client.Options
	restConfig        *rest.Config
	cacheSyncPeriod   *time.Duration
	disableCache      bool
	allowedUserFields []string
	clientConfig      clientcmd.ClientConfig
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

// WithRuntimeAPIReader returns a ConfigFunc that sets the passed runtimeAPIReader on the Config object.
func WithRuntimeAPIReader(runtimeAPIReader client.Reader) ConfigFunc {
	return func(config *Config) error {
		config.runtimeAPIReader = runtimeAPIReader
		return nil
	}
}

// WithRuntimeClient returns a ConfigFunc that sets the passed runtimeClient on the Config object.
func WithRuntimeClient(runtimeClient client.Client) ConfigFunc {
	return func(config *Config) error {
		config.runtimeClient = runtimeClient
		return nil
	}
}

// WithRuntimeCache returns a ConfigFunc that sets the passed runtimeCache on the Config object.
func WithRuntimeCache(runtimeCache cache.Cache) ConfigFunc {
	return func(config *Config) error {
		config.runtimeCache = runtimeCache
		return nil
	}
}

// WithClientConnectionOptions returns a ConfigFunc that transfers settings from
// the passed ClientConnectionConfiguration.
// The kubeconfig location in ClientConnectionConfiguration is disregarded, though!
func WithClientConnectionOptions(cfg componentbaseconfigv1alpha1.ClientConnectionConfiguration) ConfigFunc {
	return func(config *Config) error {
		if config.restConfig == nil {
			return errors.New("REST config must be set before setting connection options")
		}
		ApplyClientConnectionConfigurationToRESTConfig(&cfg, config.restConfig)
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

// WithCacheSyncPeriod returns a ConfigFunc that set the client's cache's sync period to the given duration.
func WithCacheSyncPeriod(sync time.Duration) ConfigFunc {
	return func(config *Config) error {
		config.cacheSyncPeriod = &sync
		return nil
	}
}

// WithDisabledCachedClient disables the cache in the controller-runtime client, so Client() will talk directly to the
// API server.
func WithDisabledCachedClient() ConfigFunc {
	return func(config *Config) error {
		config.disableCache = true
		return nil
	}
}

// WithNewCacheFunc allows to set the function which is used to create a new cache.
func WithNewCacheFunc(fn cache.NewCacheFunc) ConfigFunc {
	return func(config *Config) error {
		config.newRuntimeCache = fn
		return nil
	}
}

// WithAllowedUserFields allows to specify additional kubeconfig.user fields allowed during validation.
func WithAllowedUserFields(allowedUserFields []string) ConfigFunc {
	return func(config *Config) error {
		config.allowedUserFields = allowedUserFields
		return nil
	}
}

// WithClientConfig adds a ClientConfig for validation at a later stage.
func WithClientConfig(clientConfig clientcmd.ClientConfig) ConfigFunc {
	return func(config *Config) error {
		config.clientConfig = clientConfig
		return nil
	}
}
