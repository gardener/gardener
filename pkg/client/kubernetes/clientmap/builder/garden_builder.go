// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package builder

import (
	"fmt"

	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/internal"

	"github.com/sirupsen/logrus"
	"k8s.io/client-go/rest"
)

// GardenClientMapBuilder can build a ClientMap which can be used to construct a ClientMap for requesting and storing
// a client to the garden cluster. Most probably, this ClientMap will only contain one ClientSet, but this can be used
// to retrieve a client to the garden cluster via the same mechanisms as the other types of ClientSets (e.g. through
// a DelegatingClientMap).
type GardenClientMapBuilder struct {
	restConfig *rest.Config
	logger     logrus.FieldLogger
}

// NewGardenClientMapBuilder creates a new GardenClientMapBuilder.
func NewGardenClientMapBuilder() *GardenClientMapBuilder {
	return &GardenClientMapBuilder{}
}

// WithLogger sets the logger attribute of the builder.
func (b *GardenClientMapBuilder) WithLogger(logger logrus.FieldLogger) *GardenClientMapBuilder {
	b.logger = logger
	return b
}

// WithRESTConfig sets the restConfig attribute of the builder. This restConfig will be used to construct a new client
// to the garden cluster.
func (b *GardenClientMapBuilder) WithRESTConfig(cfg *rest.Config) *GardenClientMapBuilder {
	b.restConfig = cfg
	return b
}

// Build builds the GardenClientMap using the provided attributes.
func (b *GardenClientMapBuilder) Build() (clientmap.ClientMap, error) {
	if b.logger == nil {
		return nil, fmt.Errorf("logger is required but not set")
	}
	if b.restConfig == nil {
		return nil, fmt.Errorf("restConfig is required but not set")
	}

	return internal.NewGardenClientMap(&internal.GardenClientSetFactory{
		RESTConfig: b.restConfig,
	}, b.logger), nil
}
