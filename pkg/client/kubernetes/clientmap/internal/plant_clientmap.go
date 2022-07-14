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

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/utils"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// plantClientMap is a ClientMap for requesting and storing clients for Plant clusters.
type plantClientMap struct {
	clientmap.ClientMap
}

// NewPlantClientMap creates a new plantClientMap with the given factory.
func NewPlantClientMap(log logr.Logger, factory *PlantClientSetFactory) clientmap.ClientMap {
	return &plantClientMap{
		ClientMap: NewGenericClientMap(factory, log.WithValues("clientmap", "PlantClientMap"), clock.RealClock{}),
	}
}

// PlantClientSetFactory is a ClientSetFactory that can produce new ClientSets to Plant clusters.
type PlantClientSetFactory struct {
	// GardenReader is a client to the garden cluster to retrieve the Plant's kubeconfig secret.
	GardenReader client.Reader
}

// CalculateClientSetHash calculates a SHA256 hash of the kubeconfig in the plant secret.
func (f *PlantClientSetFactory) CalculateClientSetHash(ctx context.Context, k clientmap.ClientSetKey) (string, error) {
	_, hash, err := f.getSecretAndComputeHash(ctx, k)
	if err != nil {
		return "", err
	}

	return hash, nil
}

// NewClientSet creates a new ClientSet for a Plant cluster.
func (f *PlantClientSetFactory) NewClientSet(ctx context.Context, k clientmap.ClientSetKey) (kubernetes.Interface, string, error) {
	kubeconfigSecret, hash, err := f.getSecretAndComputeHash(ctx, k)
	if err != nil {
		return nil, "", err
	}

	clientSet, err := NewClientFromSecretObject(kubeconfigSecret,
		kubernetes.WithClientOptions(client.Options{
			Scheme: kubernetes.PlantScheme,
		}),
		kubernetes.WithDisabledCachedClient(),
	)
	if err != nil {
		return nil, "", err
	}

	return clientSet, hash, nil
}

func (f *PlantClientSetFactory) getSecretAndComputeHash(ctx context.Context, k clientmap.ClientSetKey) (*corev1.Secret, string, error) {
	key, ok := k.(PlantClientSetKey)
	if !ok {
		return nil, "", fmt.Errorf("unsupported ClientSetKey: expected %T got %T", PlantClientSetKey{}, k)
	}

	secretRef, err := f.getPlantSecretRef(ctx, key)
	if err != nil {
		return nil, "", err
	}

	kubeconfigSecret := &corev1.Secret{}
	if err := f.GardenReader.Get(ctx, client.ObjectKey{Namespace: key.Namespace, Name: secretRef.Name}, kubeconfigSecret); err != nil {
		return nil, "", err
	}

	return kubeconfigSecret, utils.ComputeSHA256Hex(kubeconfigSecret.Data[kubernetes.KubeConfig]), nil
}

func (f *PlantClientSetFactory) getPlantSecretRef(ctx context.Context, key PlantClientSetKey) (*corev1.LocalObjectReference, error) {
	plant := &gardencorev1beta1.Plant{}
	if err := f.GardenReader.Get(ctx, client.ObjectKey{Namespace: key.Namespace, Name: key.Name}, plant); err != nil {
		return nil, fmt.Errorf("failed to get Plant object %q: %w", key.Key(), err)
	}

	if plant.Spec.SecretRef.Name == "" {
		return nil, fmt.Errorf("plant %q does not have a secretRef", key.Key())
	}

	return &plant.Spec.SecretRef, nil
}

// PlantClientSetKey is a ClientSetKey for a Plant cluster.
type PlantClientSetKey struct {
	Namespace, Name string
}

// Key returns the string representation of the ClientSetKey.
func (k PlantClientSetKey) Key() string {
	return k.Namespace + "/" + k.Name
}
