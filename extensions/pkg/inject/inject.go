// SPDX-FileCopyrightText: 2019 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package inject

import (
	"context"

	contextutil "github.com/gardener/gardener/pkg/utils/context"

	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// WithClient contains an instance of `client.Client`.
type WithClient struct {
	Client client.Client
}

// InjectClient implements `inject.InjectClient`.
func (w *WithClient) InjectClient(c client.Client) error {
	w.Client = c
	return nil
}

// WithStopChannel contains a stop channel.
type WithStopChannel struct {
	StopChannel <-chan struct{}
}

// InjectStopChannel implements `inject.InjectStopChannel`.
func (w *WithStopChannel) InjectStopChannel(stopChan <-chan struct{}) error {
	w.StopChannel = stopChan
	return nil
}

// WithContext contains a `context.Context`.
type WithContext struct {
	Context context.Context
}

// InjectStopChannel implements `inject.InjectStopChannel`.
func (w *WithContext) InjectStopChannel(stopChan <-chan struct{}) error {
	w.Context = contextutil.FromStopChannel(stopChan)
	return nil
}

// WithCache contains an instance of `cache.Cache`.
type WithCache struct {
	Cache cache.Cache
}

// InjectCache implements `inject.InjectCache`.
func (w *WithCache) InjectCache(cache cache.Cache) error {
	w.Cache = cache
	return nil
}
