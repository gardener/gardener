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

package builder

import (
	"fmt"

	"github.com/go-logr/logr"
	componentbaseconfig "k8s.io/component-base/config"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/internal"
)

// ShootClientMapBuilder can build a ClientMap which can be used to construct a ClientMap for requesting and storing
// clients for Shoot clusters.
type ShootClientMapBuilder struct {
	gardenClient           client.Client
	seedClient             client.Client
	clientConnectionConfig *componentbaseconfig.ClientConnectionConfiguration
}

// NewShootClientMapBuilder constructs a new ShootClientMapBuilder.
func NewShootClientMapBuilder() *ShootClientMapBuilder {
	return &ShootClientMapBuilder{}
}

// WithGardenClient sets the garden client.
func (b *ShootClientMapBuilder) WithGardenClient(client client.Client) *ShootClientMapBuilder {
	b.gardenClient = client
	return b
}

// WithSeedClient sets the garden client.
func (b *ShootClientMapBuilder) WithSeedClient(client client.Client) *ShootClientMapBuilder {
	b.seedClient = client
	return b
}

// WithClientConnectionConfig sets the ClientConnectionConfiguration that should be used by ClientSets created by this ClientMap.
func (b *ShootClientMapBuilder) WithClientConnectionConfig(cfg *componentbaseconfig.ClientConnectionConfiguration) *ShootClientMapBuilder {
	b.clientConnectionConfig = cfg
	return b
}

// Build builds the ShootClientMap using the provided attributes.
func (b *ShootClientMapBuilder) Build(log logr.Logger) (clientmap.ClientMap, error) {
	if b.gardenClient == nil {
		return nil, fmt.Errorf("garden client is required but not set")
	}
	if b.seedClient == nil {
		return nil, fmt.Errorf("seed client is required but not set")
	}
	if b.clientConnectionConfig == nil {
		return nil, fmt.Errorf("clientConnectionConfig is required but not set")
	}

	return internal.NewShootClientMap(log, &internal.ShootClientSetFactory{
		GardenClient:           b.gardenClient,
		SeedClient:             b.seedClient,
		ClientConnectionConfig: *b.clientConnectionConfig,
	}), nil
}
