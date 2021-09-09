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

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/internal"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"

	"github.com/sirupsen/logrus"
)

// PlantClientMapBuilder can build a ClientMap which can be used to construct a ClientMap for requesting and storing
// clients for Plant clusters.
type PlantClientMapBuilder struct {
	gardenClientFunc func(ctx context.Context) (kubernetes.Interface, error)

	logger logrus.FieldLogger
}

// NewPlantClientMapBuilder constructs a new PlantClientMapBuilder.
func NewPlantClientMapBuilder() *PlantClientMapBuilder {
	return &PlantClientMapBuilder{}
}

// WithLogger sets the logger attribute of the builder.
func (b *PlantClientMapBuilder) WithLogger(logger logrus.FieldLogger) *PlantClientMapBuilder {
	b.logger = logger
	return b
}

// WithGardenClientMap sets the ClientMap that should be used to retrieve Garden clients.
func (b *PlantClientMapBuilder) WithGardenClientMap(clientMap clientmap.ClientMap) *PlantClientMapBuilder {
	b.gardenClientFunc = func(ctx context.Context) (kubernetes.Interface, error) {
		return clientMap.GetClient(ctx, keys.ForGarden())
	}
	return b
}

// WithGardenClientSet sets the ClientSet that should be used as the Garden client.
func (b *PlantClientMapBuilder) WithGardenClientSet(clientSet kubernetes.Interface) *PlantClientMapBuilder {
	b.gardenClientFunc = func(ctx context.Context) (kubernetes.Interface, error) {
		return clientSet, nil
	}
	return b
}

// Build builds the PlantClientMap using the provided attributes.
func (b *PlantClientMapBuilder) Build() (clientmap.ClientMap, error) {
	if b.logger == nil {
		return nil, fmt.Errorf("logger is required but not set")
	}
	if b.gardenClientFunc == nil {
		return nil, fmt.Errorf("garden client is required but not set")
	}

	return internal.NewPlantClientMap(&internal.PlantClientSetFactory{
		GetGardenClient: b.gardenClientFunc,
	}, b.logger), nil
}
