// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors
// SPDX-License-Identifier: Apache-2.0

package builder

import (
	"fmt"

	"github.com/go-logr/logr"
	componentbaseconfig "k8s.io/component-base/config"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/internal"
)

// GardenClientMapBuilder can build a ClientMap which can be used to
// request and store clients for virtual gardens.
type GardenClientMapBuilder struct {
	runtimeClient          client.Client
	clientConnectionConfig *componentbaseconfig.ClientConnectionConfiguration
	gardenNamespace        string
}

// NewGardenClientMapBuilder constructs a new GardenClientMapBuilder.
func NewGardenClientMapBuilder() *GardenClientMapBuilder {
	return &GardenClientMapBuilder{}
}

// WithRuntimeClient sets the garden client.
func (b *GardenClientMapBuilder) WithRuntimeClient(client client.Client) *GardenClientMapBuilder {
	b.runtimeClient = client
	return b
}

// WithClientConnectionConfig sets the ClientConnectionConfiguration that should be used by ClientSets created by this ClientMap.
func (b *GardenClientMapBuilder) WithClientConnectionConfig(cfg *componentbaseconfig.ClientConnectionConfiguration) *GardenClientMapBuilder {
	b.clientConnectionConfig = cfg
	return b
}

// WithGardenNamespace sets the GardenNamespace that should be used by ClientSets created by this ClientMap. Defaults to `garden` if not set.
func (b *GardenClientMapBuilder) WithGardenNamespace(namespace string) *GardenClientMapBuilder {
	b.gardenNamespace = namespace
	return b
}

// Build builds the GardenClientMap using the provided attributes.
func (b *GardenClientMapBuilder) Build(log logr.Logger) (clientmap.ClientMap, error) {
	if b.runtimeClient == nil {
		return nil, fmt.Errorf("runtime client is required but not set")
	}
	if b.clientConnectionConfig == nil {
		return nil, fmt.Errorf("clientConnectionConfig is required but not set")
	}

	return internal.NewGardenClientMap(log, &internal.GardenClientSetFactory{
		RuntimeClient:          b.runtimeClient,
		ClientConnectionConfig: *b.clientConnectionConfig,
		GardenNamespace:        b.gardenNamespace,
	}), nil
}
