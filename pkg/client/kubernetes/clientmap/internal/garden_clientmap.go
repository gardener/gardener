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

	"github.com/sirupsen/logrus"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
)

// gardenClientMap is a ClientMap for requesting and storing a client to the garden cluster. Most probably, this
// ClientMap will only contain one ClientSet, but this can be used to retrieve a client to the garden cluster via the
// same mechanisms as the other types of ClientSets (e.g. through a DelegatingClientMap).
type gardenClientMap struct {
	clientmap.ClientMap
}

// NewGardenClientMap creates a new gardenClientMap with the given factory and logger.
func NewGardenClientMap(factory *GardenClientSetFactory, logger logrus.FieldLogger) clientmap.ClientMap {
	return &gardenClientMap{
		ClientMap: NewGenericClientMap(factory, logger),
	}
}

// GardenClientSetFactory is a ClientSetFactory that can produce new ClientSets to the garden cluster.
type GardenClientSetFactory struct {
	// RESTConfig is a rest.Config that will be used by the created ClientSets.
	RESTConfig *rest.Config
}

// CalculateClientSetHash returns "" as the garden client config cannot change during runtime
func (f *GardenClientSetFactory) CalculateClientSetHash(context.Context, clientmap.ClientSetKey) (string, error) {
	return "", nil
}

// NewClientSet creates a new ClientSet to the garden cluster.
func (f *GardenClientSetFactory) NewClientSet(_ context.Context, k clientmap.ClientSetKey) (kubernetes.Interface, error) {
	_, ok := k.(GardenClientSetKey)
	if !ok {
		return nil, fmt.Errorf("unsupported ClientSetKey: expected %T got %T", GardenClientSetKey{}, k)
	}

	return NewClientSetWithConfig(
		kubernetes.WithRESTConfig(f.RESTConfig),
		kubernetes.WithClientOptions(client.Options{
			Scheme: kubernetes.GardenScheme,
		}),
	)
}

// GardenClientSetKey is a ClientSetKey for the garden cluster.
type GardenClientSetKey struct{}

func (k GardenClientSetKey) Key() string {
	return "garden"
}
