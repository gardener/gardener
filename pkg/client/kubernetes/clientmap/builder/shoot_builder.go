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

package builder

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"
	baseconfig "k8s.io/component-base/config"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/internal"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
)

// ShootClientMapBuilder can build a ClientMap which can be used to construct a ClientMap for requesting and storing
// clients for Shoot clusters.
type ShootClientMapBuilder struct {
	gardenClientFunc       func(ctx context.Context) (kubernetes.Interface, error)
	seedClientFunc         func(ctx context.Context, name string) (kubernetes.Interface, error)
	clientConnectionConfig *baseconfig.ClientConnectionConfiguration

	logger logrus.FieldLogger
}

// NewShootClientMapBuilder constructs a new ShootClientMapBuilder.
func NewShootClientMapBuilder() *ShootClientMapBuilder {
	return &ShootClientMapBuilder{}
}

// WithLogger sets the logger attribute of the builder.
func (b *ShootClientMapBuilder) WithLogger(logger logrus.FieldLogger) *ShootClientMapBuilder {
	b.logger = logger
	return b
}

// WithGardenClientMap sets the ClientMap that should be used to retrieve Garden clients.
func (b *ShootClientMapBuilder) WithGardenClientMap(clientMap clientmap.ClientMap) *ShootClientMapBuilder {
	b.gardenClientFunc = func(ctx context.Context) (kubernetes.Interface, error) {
		return clientMap.GetClient(ctx, keys.ForGarden())
	}
	return b
}

// WithGardenClientSet sets the ClientSet that should be used as the Garden client.
func (b *ShootClientMapBuilder) WithGardenClientSet(clientSet kubernetes.Interface) *ShootClientMapBuilder {
	b.gardenClientFunc = func(ctx context.Context) (kubernetes.Interface, error) {
		return clientSet, nil
	}
	return b
}

// WithSeedClientMap sets the ClientMap that should be used to retrieve Seed clients.
func (b *ShootClientMapBuilder) WithSeedClientMap(clientMap clientmap.ClientMap) *ShootClientMapBuilder {
	b.seedClientFunc = func(ctx context.Context, name string) (kubernetes.Interface, error) {
		return clientMap.GetClient(ctx, keys.ForSeedWithName(name))
	}
	return b
}

// WithClientConnectionConfig sets the ClientConnectionConfiguration that should be used by ClientSets created by this ClientMap.
func (b *ShootClientMapBuilder) WithClientConnectionConfig(cfg *baseconfig.ClientConnectionConfiguration) *ShootClientMapBuilder {
	b.clientConnectionConfig = cfg
	return b
}

// Build builds the ShootClientMap using the provided attributes.
func (b *ShootClientMapBuilder) Build() (clientmap.ClientMap, error) {
	if b.logger == nil {
		return nil, fmt.Errorf("logger is required but not set")
	}
	if b.gardenClientFunc == nil {
		return nil, fmt.Errorf("garden client is required but not set")
	}
	if b.seedClientFunc == nil {
		return nil, fmt.Errorf("seed client is required but not set")
	}
	if b.clientConnectionConfig == nil {
		return nil, fmt.Errorf("clientConnectionConfig is required but not set")
	}

	return internal.NewShootClientMap(&internal.ShootClientSetFactory{
		GetGardenClient:        b.gardenClientFunc,
		GetSeedClient:          b.seedClientFunc,
		ClientConnectionConfig: *b.clientConnectionConfig,
		Log:                    b.logger,
	}, b.logger), nil
}
