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
	"fmt"

	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/internal"
)

// DelegatingClientMapBuilder can build a DelegatingClientMap which will delegate calls to different ClientMaps
// based on the type of the key (e.g. a call with keys.ForShoot() will be delegated to the ShootClientMap).
type DelegatingClientMapBuilder struct {
	gardenClientMapFunc func() (clientmap.ClientMap, error)
	seedClientMapFunc   func() (clientmap.ClientMap, error)
	shootClientMapFunc  func(gardenClients, seedClients clientmap.ClientMap) (clientmap.ClientMap, error)
	plantClientMapFunc  func(gardenClients clientmap.ClientMap) (clientmap.ClientMap, error)
}

// NewDelegatingClientMapBuilder creates a new DelegatingClientMapBuilder.
func NewDelegatingClientMapBuilder() *DelegatingClientMapBuilder {
	return &DelegatingClientMapBuilder{
		gardenClientMapFunc: func() (clientmap.ClientMap, error) {
			return nil, fmt.Errorf("garden ClientMap is required but not set")
		},
	}
}

// WithGardenClientMap sets the ClientMap that should be used for Garden clients.
func (b *DelegatingClientMapBuilder) WithGardenClientMap(clientMap clientmap.ClientMap) *DelegatingClientMapBuilder {
	b.gardenClientMapFunc = func() (clientmap.ClientMap, error) {
		return clientMap, nil
	}
	return b
}

// WithGardenClientMapBuilder sets a ClientMap builder that should be used to build a ClientMap for Garden clients.
func (b *DelegatingClientMapBuilder) WithGardenClientMapBuilder(builder *GardenClientMapBuilder) *DelegatingClientMapBuilder {
	b.gardenClientMapFunc = func() (clientmap.ClientMap, error) {
		return builder.
			Build()
	}
	return b
}

// WithSeedClientMap sets the ClientMap that should be used for Seed clients.
func (b *DelegatingClientMapBuilder) WithSeedClientMap(clientMap clientmap.ClientMap) *DelegatingClientMapBuilder {
	b.seedClientMapFunc = func() (clientmap.ClientMap, error) {
		return clientMap, nil
	}
	return b
}

// WithSeedClientMapBuilder sets a ClientMap builder that should be used to build a ClientMap for Seed clients.
func (b *DelegatingClientMapBuilder) WithSeedClientMapBuilder(builder *SeedClientMapBuilder) *DelegatingClientMapBuilder {
	b.seedClientMapFunc = func() (clientmap.ClientMap, error) {
		return builder.
			Build()
	}
	return b
}

// WithShootClientMap sets the ClientMap that should be used for Shoot clients.
func (b *DelegatingClientMapBuilder) WithShootClientMap(clientMap clientmap.ClientMap) *DelegatingClientMapBuilder {
	b.shootClientMapFunc = func(_, _ clientmap.ClientMap) (clientmap.ClientMap, error) {
		return clientMap, nil
	}
	return b
}

// WithShootClientMapBuilder sets a ClientMap builder that should be used to build a ClientMap for Shoot clients.
func (b *DelegatingClientMapBuilder) WithShootClientMapBuilder(builder *ShootClientMapBuilder) *DelegatingClientMapBuilder {
	b.shootClientMapFunc = func(gardenClients, seedClients clientmap.ClientMap) (clientmap.ClientMap, error) {
		return builder.
			WithGardenClientMap(gardenClients).
			WithSeedClientMap(seedClients).
			Build()
	}
	return b
}

// WithPlantClientMap sets the ClientMap that should be used for Plant clients.
func (b *DelegatingClientMapBuilder) WithPlantClientMap(clientMap clientmap.ClientMap) *DelegatingClientMapBuilder {
	b.plantClientMapFunc = func(_ clientmap.ClientMap) (clientmap.ClientMap, error) {
		return clientMap, nil
	}
	return b
}

// WithPlantClientMapBuilder sets a ClientMap builder that should be used to build a ClientMap for Plant clients.
func (b *DelegatingClientMapBuilder) WithPlantClientMapBuilder(builder *PlantClientMapBuilder) *DelegatingClientMapBuilder {
	b.plantClientMapFunc = func(gardenClients clientmap.ClientMap) (clientmap.ClientMap, error) {
		return builder.
			WithGardenClientMap(gardenClients).
			Build()
	}
	return b
}

// Build builds the DelegatingClientMap using the provided attributes.
func (b *DelegatingClientMapBuilder) Build() (clientmap.ClientMap, error) {
	gardenClients, err := b.gardenClientMapFunc()
	if err != nil {
		return nil, fmt.Errorf("failed to construct garden ClientMap: %w", err)
	}

	var seedClients, shootClients, plantClients clientmap.ClientMap

	if b.seedClientMapFunc != nil {
		seedClients, err = b.seedClientMapFunc()
		if err != nil {
			return nil, fmt.Errorf("failed to construct seed ClientMap: %w", err)
		}
	}

	if b.shootClientMapFunc != nil {
		if seedClients == nil {
			return nil, fmt.Errorf("failed to construct shoot ClientMap, seed ClientMap is required but not set")
		}

		shootClients, err = b.shootClientMapFunc(gardenClients, seedClients)
		if err != nil {
			return nil, fmt.Errorf("failed to construct shoot ClientMap: %w", err)
		}
	}

	if b.plantClientMapFunc != nil {
		plantClients, err = b.plantClientMapFunc(gardenClients)
		if err != nil {
			return nil, fmt.Errorf("failed to construct plant ClientMap: %w", err)
		}
	}

	return internal.NewDelegatingClientMap(gardenClients, seedClients, shootClients, plantClients), nil
}
