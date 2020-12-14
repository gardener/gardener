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

package clientmap

import (
	"context"

	"github.com/gardener/gardener/pkg/client/kubernetes"
)

// Invalidate is an interface used to invalidate client specific information.
type Invalidate interface {
	// InvalidateClient removes cached client information identified by the given `ClientSetKey`.
	InvalidateClient(key ClientSetKey) error
}

// A ClientMap is a collection of kubernetes ClientSets, which can be used to dynamically create and lookup different
// ClientSets during runtime. ClientSets are identified by a ClientSetKey, which can have different forms, as there
// are different kinds of ClientMaps (for example one for Seed clients and one for Shoot clients).
// Implementations will provide suitable mechanisms to create a ClientSet for a given key and should come with some
// easy ways of constructing ClientSetKeys that their callers can use to lookup ClientSets in the map.
type ClientMap interface {
	Invalidate
	// GetClient returns the corresponding ClientSet for the given key in the ClientMap or creates a new ClientSet for
	// it if the map does not contain a corresponding ClientSet. If the ClientMap was started before by a call to Start,
	// newly created ClientSets will be started automatically using the stop channel provided to Start.
	GetClient(ctx context.Context, key ClientSetKey) (kubernetes.Interface, error)

	// Start starts the ClientMap, i.e. starts all ClientSets already contained in the map and saves the stop channel
	// for starting new ClientSets, when they are created.
	Start(stopCh <-chan struct{}) error
}

// A ClientSetKey is used to identify a ClientSet within a ClientMap. There can be different implementations for
// ClientSetKey as there are different kinds of ClientSets (e.g. for Shoot and Seed clusters) and therefore also
// different means of identifying the cluster to which a ClientSet belongs.
type ClientSetKey interface {
	// Key is the string representation of the ClientSetKey.
	Key() string
}

// A ClientSetFactory can be used by ClientMaps to provide the individual mechanism for
// constructing new ClientSets for a given ClientSetKey
type ClientSetFactory interface {
	// NewClientSet constructs a new ClientSet for the given key.
	NewClientSet(ctx context.Context, key ClientSetKey) (kubernetes.Interface, error)
	// CalculateClientSetHash calculates a hash for the configuration that is used to construct a ClientSet
	// (e.g. kubeconfig secret) to detect if it has changed mid-air and the ClientSet should be refreshed.
	CalculateClientSetHash(ctx context.Context, key ClientSetKey) (string, error)
}
