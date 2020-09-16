// SPDX-FileCopyrightText: 2019 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package kubernetes

import (
	"errors"
	"time"

	"k8s.io/client-go/rest"
	baseconfig "k8s.io/component-base/config"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type config struct {
	clientOptions       client.Options
	restConfig          *rest.Config
	cacheResync         *time.Duration
	disableCachedClient bool
}

// ConfigFunc is a function that mutates a Config struct.
// It implements the functional options pattern. See
// https://github.com/tmrts/go-patterns/blob/master/idiom/functional-options.md.
type ConfigFunc func(config *config) error

// WithRESTConfig returns a ConfigFunc that sets the passed rest.Config on the config object.
func WithRESTConfig(restConfig *rest.Config) ConfigFunc {
	return func(config *config) error {
		config.restConfig = restConfig
		return nil
	}
}

// WithClientConnectionOptions returns a ConfigFunc that transfers settings from
// the passed ClientConnectionConfiguration.
// The kubeconfig location in ClientConnectionConfiguration is disregarded, though!
func WithClientConnectionOptions(cfg baseconfig.ClientConnectionConfiguration) ConfigFunc {
	return func(config *config) error {
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

// WithClientOptions returns a ConfigFunc that sets the passed Options on the config object.
func WithClientOptions(opt client.Options) ConfigFunc {
	return func(config *config) error {
		config.clientOptions = opt
		return nil
	}
}

// WithCacheResyncPeriod returns a ConfigFunc that set the client's cache's resync period to the given duration.
func WithCacheResyncPeriod(resync time.Duration) ConfigFunc {
	return func(config *config) error {
		config.cacheResync = &resync
		return nil
	}
}

// WithDisabledCachedClient disables the cache in the controller-runtime client, so Client() will be equivalent to
// DirectClient().
func WithDisabledCachedClient() ConfigFunc {
	return func(config *config) error {
		config.disableCachedClient = true
		return nil
	}
}
