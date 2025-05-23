// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package builder

import (
	"fmt"

	"github.com/go-logr/logr"
	componentbaseconfigv1alpha1 "k8s.io/component-base/config/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
)

// ShootClientMapBuilder can build a ClientMap which can be used to construct a ClientMap for requesting and storing
// clients for Shoot clusters.
type ShootClientMapBuilder struct {
	gardenClient           client.Client
	seedClient             client.Client
	clientConnectionConfig *componentbaseconfigv1alpha1.ClientConnectionConfiguration
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
func (b *ShootClientMapBuilder) WithClientConnectionConfig(cfg *componentbaseconfigv1alpha1.ClientConnectionConfiguration) *ShootClientMapBuilder {
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

	return clientmap.NewShootClientMap(log, &clientmap.ShootClientSetFactory{
		GardenClient:           b.gardenClient,
		SeedClient:             b.seedClient,
		ClientConnectionConfig: *b.clientConnectionConfig,
	}), nil
}
