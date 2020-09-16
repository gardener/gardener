// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

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

// SeedClientMapBuilder can build a ClientMap which can be used to construct a ClientMap for requesting and storing
// clients for Seed clusters.
type SeedClientMapBuilder struct {
	gardenClientFunc       func(ctx context.Context) (kubernetes.Interface, error)
	inCluster              bool
	clientConnectionConfig *baseconfig.ClientConnectionConfiguration

	logger logrus.FieldLogger
}

// NewSeedClientMapBuilder constructs a new SeedClientMapBuilder.
func NewSeedClientMapBuilder() *SeedClientMapBuilder {
	return &SeedClientMapBuilder{}
}

// WithLogger sets the logger attribute of the builder.
func (b *SeedClientMapBuilder) WithLogger(logger logrus.FieldLogger) *SeedClientMapBuilder {
	b.logger = logger
	return b
}

// WithGardenClientMap sets the ClientMap that should be used to retrieve Garden clients.
func (b *SeedClientMapBuilder) WithGardenClientMap(clientMap clientmap.ClientMap) *SeedClientMapBuilder {
	b.gardenClientFunc = func(ctx context.Context) (kubernetes.Interface, error) {
		return clientMap.GetClient(ctx, keys.ForGarden())
	}
	return b
}

// WithGardenClientSet sets the ClientSet that should be used as the Garden client.
func (b *SeedClientMapBuilder) WithGardenClientSet(clientSet kubernetes.Interface) *SeedClientMapBuilder {
	b.gardenClientFunc = func(ctx context.Context) (kubernetes.Interface, error) {
		return clientSet, nil
	}
	return b
}

// WithInCluster sets the inCluster attribute of the builder. If true, the created ClientSets will use in-cluster communication
// (using the provided ClientConnectionConfig.Kubeconfig or fallback to mounted ServiceAccount if unset).
func (b *SeedClientMapBuilder) WithInCluster(inCluster bool) *SeedClientMapBuilder {
	b.inCluster = inCluster
	return b
}

// WithClientConnectionConfig sets the ClientConnectionConfiguration that should be used by ClientSets created by this ClientMap.
func (b *SeedClientMapBuilder) WithClientConnectionConfig(cfg *baseconfig.ClientConnectionConfiguration) *SeedClientMapBuilder {
	b.clientConnectionConfig = cfg
	return b
}

// Build builds the SeedClientMap using the provided attributes.
func (b *SeedClientMapBuilder) Build() (clientmap.ClientMap, error) {
	if b.logger == nil {
		return nil, fmt.Errorf("logger is required but not set")
	}
	if b.gardenClientFunc == nil {
		return nil, fmt.Errorf("garden client is required but not set")
	}
	if b.clientConnectionConfig == nil {
		return nil, fmt.Errorf("clientConnectionConfig is required but not set")
	}

	return internal.NewSeedClientMap(&internal.SeedClientSetFactory{
		GetGardenClient:        b.gardenClientFunc,
		InCluster:              b.inCluster,
		ClientConnectionConfig: *b.clientConnectionConfig,
	}, b.logger), nil
}
