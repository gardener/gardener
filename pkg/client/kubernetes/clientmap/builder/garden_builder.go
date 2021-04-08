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

	"github.com/sirupsen/logrus"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GardenClientMapBuilder can build a ClientMap which can be used to construct a ClientMap for requesting and storing
// a client to the garden cluster. Most probably, this ClientMap will only contain one ClientSet, but this can be used
// to retrieve a client to the garden cluster via the same mechanisms as the other types of ClientSets (e.g. through
// a DelegatingClientMap).
type GardenClientMapBuilder struct {
	restConfig      *rest.Config
	uncachedObjects []client.Object
	logger          logrus.FieldLogger
	seedName        string
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

// WithUncached takes a list of runtime objects (plain or lists) that users don't want to cache
// for this client. This function can be called multiple times, it should append to an internal slice.
func (b *GardenClientMapBuilder) WithUncached(objs ...client.Object) *GardenClientMapBuilder {
	b.uncachedObjects = append(b.uncachedObjects, objs...)
	return b
}

// ForSeed sets the seed that will be used to construct a new client to the garden cluster.
func (b *GardenClientMapBuilder) ForSeed(name string) *GardenClientMapBuilder {
	b.seedName = name
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
		RESTConfig:      b.restConfig,
		UncachedObjects: b.uncachedObjects,
		SeedName:        b.seedName,
	}, b.logger), nil
}
