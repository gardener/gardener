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
	baseconfig "k8s.io/component-base/config"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/utils"
)

// seedClientMap is a ClientMap for requesting and storing clients for Seed clusters.
type seedClientMap struct {
	clientmap.ClientMap
}

// NewSeedClientMap creates a new seedClientMap with the given factory and logger.
func NewSeedClientMap(factory *SeedClientSetFactory, logger logrus.FieldLogger) clientmap.ClientMap {
	return &seedClientMap{
		ClientMap: NewGenericClientMap(factory, logger),
	}
}

// SeedClientSetFactory is a ClientSetFactory that can produce new ClientSets to Seed clusters.
type SeedClientSetFactory struct {
	// GetGardenClient is a func that will be used to get a client to the garden cluster to retrieve the Seed's
	// kubeconfig secret (if InCluster=false).
	GetGardenClient func(ctx context.Context) (kubernetes.Interface, error)
	// If InCluster is set to true, the created ClientSets will use in-cluster communication
	// (using ClientConnectionConfig.Kubeconfig or fallback to mounted ServiceAccount if unset).
	InCluster bool
	// ClientConnectionConfiguration is the configuration that will be used by created ClientSets.
	ClientConnectionConfig baseconfig.ClientConnectionConfiguration
}

// CalculateClientSetHash returns "" if the gardenlet uses in-cluster communication. Otherwise, it calculates a SHA256
// hash of the kubeconfig in the seed secret.
func (f *SeedClientSetFactory) CalculateClientSetHash(ctx context.Context, k clientmap.ClientSetKey) (string, error) {
	key, ok := k.(SeedClientSetKey)
	if !ok {
		return "", fmt.Errorf("unsupported ClientSetKey: expected %T got %T", SeedClientSetKey(""), k)
	}

	if f.InCluster {
		return "", nil
	}

	secretRef, gardenClient, err := f.getSeedSecretRef(ctx, key)
	if err != nil {
		return "", err
	}

	kubeconfigSecret := &corev1.Secret{}
	if err := gardenClient.Client().Get(ctx, client.ObjectKey{Namespace: secretRef.Namespace, Name: secretRef.Name}, kubeconfigSecret); err != nil {
		return "", err
	}

	return utils.ComputeSHA256Hex(kubeconfigSecret.Data[kubernetes.KubeConfig]), nil
}

// NewClientSet creates a new ClientSet for a Seed cluster.
func (f *SeedClientSetFactory) NewClientSet(ctx context.Context, k clientmap.ClientSetKey) (kubernetes.Interface, error) {
	key, ok := k.(SeedClientSetKey)
	if !ok {
		return nil, fmt.Errorf("unsupported ClientSetKey: expected %T got %T", SeedClientSetKey(""), k)
	}

	if f.InCluster {
		return NewClientFromFile(
			"",
			f.ClientConnectionConfig.Kubeconfig,
			kubernetes.WithClientConnectionOptions(f.ClientConnectionConfig),
			kubernetes.WithClientOptions(
				client.Options{
					Scheme: kubernetes.SeedScheme,
				},
			),
		)
	}

	secretRef, gardenClient, err := f.getSeedSecretRef(ctx, key)
	if err != nil {
		return nil, err
	}

	return NewClientFromSecret(ctx, gardenClient.Client(), secretRef.Namespace, secretRef.Name,
		kubernetes.WithClientConnectionOptions(f.ClientConnectionConfig),
		kubernetes.WithClientOptions(client.Options{
			Scheme: kubernetes.SeedScheme,
		}),
	)
}

func (f *SeedClientSetFactory) getSeedSecretRef(ctx context.Context, key SeedClientSetKey) (*corev1.SecretReference, kubernetes.Interface, error) {
	gardenClient, err := f.GetGardenClient(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get garden client: %w", err)
	}

	seed := &gardencorev1beta1.Seed{}
	if err := gardenClient.Client().Get(ctx, client.ObjectKey{Name: key.Key()}, seed); err != nil {
		return nil, nil, fmt.Errorf("failed to get Seed object %q: %w", key.Key(), err)
	}

	if seed.Spec.SecretRef == nil {
		return nil, nil, fmt.Errorf("seed %q does not have a secretRef", key.Key())
	}

	return seed.Spec.SecretRef, gardenClient, nil
}

// SeedClientSetKey is a ClientSetKey for a Seed cluster.
type SeedClientSetKey string

func (k SeedClientSetKey) Key() string {
	return string(k)
}
