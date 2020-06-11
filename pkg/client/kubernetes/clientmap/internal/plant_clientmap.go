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
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
)

// plantClientMap is a ClientMap for requesting and storing clients for Plant clusters.
type plantClientMap struct {
	clientmap.ClientMap
}

// NewPlantClientMap creates a new plantClientMap with the given factory and logger.
func NewPlantClientMap(factory *PlantClientSetFactory, logger logrus.FieldLogger) clientmap.ClientMap {
	return &plantClientMap{
		ClientMap: NewGenericClientMap(factory, logger),
	}
}

// PlantClientSetFactory is a ClientSetFactory that can produce new ClientSets to Plant clusters.
type PlantClientSetFactory struct {
	// GetGardenClient is a func that will be used to get a client to the garden cluster to retrieve the Plant's
	// kubeconfig secret.
	GetGardenClient func(ctx context.Context) (kubernetes.Interface, error)
}

// NewClientSet creates a new ClientSet for a Plant cluster.
func (f *PlantClientSetFactory) NewClientSet(ctx context.Context, k clientmap.ClientSetKey) (kubernetes.Interface, error) {
	key, ok := k.(PlantClientSetKey)
	if !ok {
		return nil, fmt.Errorf("call to GetClient with unsupported ClientSetKey: expected %T got %T", PlantClientSetKey{}, k)
	}

	gardenClient, err := f.GetGardenClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get garden client: %w", err)
	}

	plant := &gardencorev1beta1.Plant{}
	if err := gardenClient.Client().Get(ctx, client.ObjectKey{Namespace: key.Namespace, Name: key.Name}, plant); err != nil {
		return nil, fmt.Errorf("failed to get Plant object %q: %w", key.Key(), err)
	}

	if plant.Spec.SecretRef.Name == "" {
		return nil, fmt.Errorf("plant %q does not have a secretRef", key.Key())
	}

	return NewClientFromSecret(ctx, gardenClient.Client(), key.Namespace, plant.Spec.SecretRef.Name,
		kubernetes.WithClientOptions(client.Options{
			Scheme: kubernetes.PlantScheme,
		}),
	)
}

// PlantClientSetKey is a ClientSetKey for a Plant cluster.
type PlantClientSetKey struct {
	Namespace, Name string
}

func (k PlantClientSetKey) Key() string {
	return k.Namespace + "/" + k.Name
}
