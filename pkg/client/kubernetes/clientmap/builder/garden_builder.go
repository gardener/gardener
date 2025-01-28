// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package builder

import (
	"errors"

	"github.com/go-logr/logr"
	componentbaseconfigv1alpha1 "k8s.io/component-base/config/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
)

// GardenClientMapBuilder can build a ClientMap which can be used to
// request and store clients for virtual gardens.
type GardenClientMapBuilder struct {
	runtimeClient          client.Client
	clientConnectionConfig *componentbaseconfigv1alpha1.ClientConnectionConfiguration
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
func (b *GardenClientMapBuilder) WithClientConnectionConfig(cfg *componentbaseconfigv1alpha1.ClientConnectionConfiguration) *GardenClientMapBuilder {
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
		return nil, errors.New("runtime client is required but not set")
	}
	if b.clientConnectionConfig == nil {
		return nil, errors.New("clientConnectionConfig is required but not set")
	}

	return clientmap.NewGardenClientMap(log, &clientmap.GardenClientSetFactory{
		RuntimeClient:          b.runtimeClient,
		ClientConnectionConfig: *b.clientConnectionConfig,
		GardenNamespace:        b.gardenNamespace,
	}), nil
}
