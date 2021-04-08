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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
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
	// UncachedObjects is a list of objects that will not be cached.
	UncachedObjects []client.Object
	// SeedName is the name of the seed that will be used by the created ClientSets.
	SeedName string
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

	configFns := []kubernetes.ConfigFunc{
		kubernetes.WithRESTConfig(f.RESTConfig),
		kubernetes.WithClientOptions(client.Options{Scheme: kubernetes.GardenScheme}),
		kubernetes.WithUncached(f.UncachedObjects...),
	}

	// Use multi-namespaced caches for Secrets which only consider seed namespaces
	// because Gardenlet is not permitted to open a watch for Secrets on any other namespace.
	if seedName := f.SeedName; len(seedName) > 0 {
		configFns = append(configFns, kubernetes.WithNewCacheFunc(
			kubernetes.AggregatorCacheFunc(
				kubernetes.NewRuntimeCache,
				map[client.Object]cache.NewCacheFunc{
					&corev1.Secret{}: cache.MultiNamespacedCacheBuilder([]string{gardenerutils.ComputeGardenNamespace(seedName)}),
				},
				kubernetes.GardenScheme,
			),
		))
	} else {
		// We need to disable caching for Secrets if Gardenlet is potentially responsible for multiple seeds
		// because we cannot (yet) handle seed additions/removals dynamically for the Garden client at runtime.
		// https://github.com/gardener/gardener/pull/3700#discussion_r593070505
		configFns = append(configFns, kubernetes.WithUncached(&corev1.Secret{}))
	}

	return NewClientSetWithConfig(configFns...)
}

// GardenClientSetKey is a ClientSetKey for the garden cluster.
type GardenClientSetKey struct{}

func (k GardenClientSetKey) Key() string {
	return "garden"
}
