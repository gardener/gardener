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

package internal

import (
	"context"
	"fmt"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
)

var _ clientmap.ClientMap = &delegatingClientMap{}

// delegatingClientMap is a ClientMap, which will delegate calls to different ClientMaps based on
// the type of the key (e.g. a call with keys.ForShoot() will be delegated to the ShootClientMap).
type delegatingClientMap struct {
	GardenClients clientmap.ClientMap
	SeedClients   clientmap.ClientMap
	ShootClients  clientmap.ClientMap
	PlantClients  clientmap.ClientMap
}

// NewDelegatingClientMap constructs a new delegatingClientMap consisting of the given different ClientMaps.
// It will panic if `gardenClientMap` is nil.
func NewDelegatingClientMap(gardenClientMap, seedClientMap, shootClientMap, plantClientMap clientmap.ClientMap) clientmap.ClientMap {
	if gardenClientMap == nil {
		panic("delegatingClientMap must contain a non-nil gardenClientMap")
	}

	return &delegatingClientMap{
		GardenClients: gardenClientMap,
		SeedClients:   seedClientMap,
		ShootClients:  shootClientMap,
		PlantClients:  plantClientMap,
	}
}

// errUnknownKeyType is an error that will be returned by a delegatingClientMap if the type of the given key is unknown.
type errUnknownKeyType struct {
	calledFunc string
	key        clientmap.ClientSetKey
}

func (e *errUnknownKeyType) Error() string {
	return fmt.Sprintf("call to %s with unknown ClientSetKey type: %T", e.calledFunc, e.key)
}

// errUnsupportedKeyType is an error that will be returned by a delegatingClientMap if it doesn't contain a ClientMap
// that is responsible for the type of the given key.
type errUnsupportedKeyType struct {
	calledFunc string
	key        clientmap.ClientSetKey
}

func (e *errUnsupportedKeyType) Error() string {
	return fmt.Sprintf("call to %s with unsupported ClientSetKey type: %T, delegatingClientMap doesn't contain a ClientMap responsible for this key type", e.calledFunc, e.key)
}

// GetClient delegates the call to the ClientMap responsible for the type of the given key.
func (cm *delegatingClientMap) GetClient(ctx context.Context, key clientmap.ClientSetKey) (kubernetes.Interface, error) {
	switch key.(type) {
	case GardenClientSetKey:
		return cm.GardenClients.GetClient(ctx, key)
	case SeedClientSetKey:
		if cm.SeedClients != nil {
			return cm.SeedClients.GetClient(ctx, key)
		}
		return nil, &errUnsupportedKeyType{
			calledFunc: "GetClient",
			key:        key,
		}
	case ShootClientSetKey:
		if cm.ShootClients != nil {
			return cm.ShootClients.GetClient(ctx, key)
		}
		return nil, &errUnsupportedKeyType{
			calledFunc: "GetClient",
			key:        key,
		}
	case PlantClientSetKey:
		if cm.PlantClients != nil {
			return cm.PlantClients.GetClient(ctx, key)
		}
		return nil, &errUnsupportedKeyType{
			calledFunc: "GetClient",
			key:        key,
		}
	}

	return nil, &errUnknownKeyType{
		calledFunc: "GetClient",
		key:        key,
	}
}

// InvalidateClient delegates the call to the ClientMap responsible for the type of the given key.
func (cm *delegatingClientMap) InvalidateClient(key clientmap.ClientSetKey) error {
	switch key.(type) {
	case GardenClientSetKey:
		return cm.GardenClients.InvalidateClient(key)
	case SeedClientSetKey:
		if cm.SeedClients != nil {
			return cm.SeedClients.InvalidateClient(key)
		}
		return &errUnsupportedKeyType{
			calledFunc: "InvalidateClient",
			key:        key,
		}
	case ShootClientSetKey:
		if cm.ShootClients != nil {
			return cm.ShootClients.InvalidateClient(key)
		}
		return &errUnsupportedKeyType{
			calledFunc: "InvalidateClient",
			key:        key,
		}
	case PlantClientSetKey:
		if cm.PlantClients != nil {
			return cm.PlantClients.InvalidateClient(key)
		}
		return &errUnsupportedKeyType{
			calledFunc: "InvalidateClient",
			key:        key,
		}
	}

	return &errUnknownKeyType{
		calledFunc: "InvalidateClient",
		key:        key,
	}
}

// Start delegates the call to all contained non-nil ClientMaps.
func (cm *delegatingClientMap) Start(stopCh <-chan struct{}) error {
	if err := cm.GardenClients.Start(stopCh); err != nil {
		return fmt.Errorf("failed to start garden ClientMap: %w", err)
	}

	if cm.SeedClients != nil {
		if err := cm.SeedClients.Start(stopCh); err != nil {
			return fmt.Errorf("failed to start seed ClientMap: %w", err)
		}
	}

	if cm.ShootClients != nil {
		if err := cm.ShootClients.Start(stopCh); err != nil {
			return fmt.Errorf("failed to start shoot ClientMap: %w", err)
		}
	}

	if cm.PlantClients != nil {
		if err := cm.PlantClients.Start(stopCh); err != nil {
			return fmt.Errorf("failed to start plant ClientMap: %w", err)
		}
	}
	return nil
}
