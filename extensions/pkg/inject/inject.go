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

package inject

import (
	"context"

	"github.com/gardener/gardener/extensions/pkg/util"

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
	w.Context = util.ContextFromStopChannel(stopChan)
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
