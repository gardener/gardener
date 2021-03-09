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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	seedpkg "github.com/gardener/gardener/pkg/operation/seed"
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
	// SeedSelector are the selected seeds that will be used by the created ClientSets.
	SeedSelector SeedSelector
}

// SeedSelector holds options about how to select certain seed(s).
type SeedSelector struct {
	SeedName string
	Selector *metav1.LabelSelector
}

// CalculateClientSetHash returns "" as the garden client config cannot change during runtime
func (f *GardenClientSetFactory) CalculateClientSetHash(context.Context, clientmap.ClientSetKey) (string, error) {
	return "", nil
}

// NewClientSet creates a new ClientSet to the garden cluster.
func (f *GardenClientSetFactory) NewClientSet(ctx context.Context, k clientmap.ClientSetKey) (kubernetes.Interface, error) {
	_, ok := k.(GardenClientSetKey)
	if !ok {
		return nil, fmt.Errorf("unsupported ClientSetKey: expected %T got %T", GardenClientSetKey{}, k)
	}

	configFns := []kubernetes.ConfigFunc{
		kubernetes.WithRESTConfig(f.RESTConfig),
		kubernetes.WithClientOptions(client.Options{
			Scheme: kubernetes.GardenScheme,
		}),
	}

	// Create multi-namespace cache for secrets
	var seedNamespaces []string
	if seedName := f.SeedSelector.SeedName; len(seedName) > 0 {
		seedNamespaces = append(seedNamespaces, seedpkg.ComputeGardenNamespace(seedName))
	}
	if ls := f.SeedSelector.Selector; ls != nil {
		directClient, err := client.New(f.RESTConfig, client.Options{
			Scheme: kubernetes.GardenScheme,
		})
		if err != nil {
			return nil, err
		}

		seeds := &corev1beta1.SeedList{}

		seedSelector, err := metav1.LabelSelectorAsSelector(ls)
		if err != nil {
			return nil, err
		}

		if err := directClient.List(ctx, seeds, client.MatchingLabelsSelector{Selector: seedSelector}); err != nil {
			return nil, err
		}

		for _, seed := range seeds.Items {
			seedNamespaces = append(seedNamespaces, seedpkg.ComputeGardenNamespace(seed.Name))
		}
	}

	// Do not use multi-namespace informers if `CachedRuntimeClients` feature is enabled because today we still need
	// to access other namespace which are automatically watched.
	if len(seedNamespaces) > 0 {
		configFns = append(configFns, kubernetes.WithNewCacheFunc(
			kubernetes.AggregatorCacheFunc(
				kubernetes.NewRuntimeCache,
				map[client.Object]cache.NewCacheFunc{
					&corev1.Secret{}: cache.MultiNamespacedCacheBuilder(seedNamespaces),
				},
				kubernetes.GardenScheme,
			),
		),
		)
	}

	return NewClientSetWithConfig(configFns...)
}

// GardenClientSetKey is a ClientSetKey for the garden cluster.
type GardenClientSetKey struct{}

func (k GardenClientSetKey) Key() string {
	return "garden"
}
