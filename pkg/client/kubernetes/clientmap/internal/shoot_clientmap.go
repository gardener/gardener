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
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	shootpkg "github.com/gardener/gardener/pkg/operation/shoot"
	"github.com/gardener/gardener/pkg/utils"

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	baseconfig "k8s.io/component-base/config"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// shootClientMap is a ClientMap for requesting and storing clients for Shoot clusters.
type shootClientMap struct {
	clientmap.ClientMap
}

// NewShootClientMap creates a new shootClientMap with the given factory and logger.
func NewShootClientMap(factory *ShootClientSetFactory, logger logrus.FieldLogger) clientmap.ClientMap {
	factory.clientKeyToSeedInfo = make(map[ShootClientSetKey]seedInfo)
	return &shootClientMap{
		ClientMap: NewGenericClientMap(factory, logger),
	}
}

// ShootClientSetFactory is a ClientSetFactory that can produce new ClientSets to Shoot clusters.
type ShootClientSetFactory struct {
	// GetGardenClient is a func that will be used to get a client to the garden cluster to retrieve the Shoot's
	// Project name (which is used for determining the Shoot's technical ID).
	GetGardenClient func(ctx context.Context) (kubernetes.Interface, error)
	// GetSeedClient is a func that will be used to get a client to the Shoot's Seed cluster to retrieve the Shoot's
	// kubeconfig secret ('gardener-internal' or 'gardener').
	GetSeedClient func(ctx context.Context, name string) (kubernetes.Interface, error)
	// ClientConnectionConfiguration is the configuration that will be used by created ClientSets.
	ClientConnectionConfig baseconfig.ClientConnectionConfiguration

	// Log is a logger for logging entries related to creating Shoot ClientSets.
	Log logrus.FieldLogger

	clientKeyToSeedInfo map[ShootClientSetKey]seedInfo
}

type seedInfo struct {
	namespace string
	seedName  string
}

// CalculateClientSetHash calculates a SHA256 hash of the kubeconfig in the 'gardener' secret in the Shoot's Seed namespace.
func (f *ShootClientSetFactory) CalculateClientSetHash(ctx context.Context, k clientmap.ClientSetKey) (string, error) {
	key, ok := k.(ShootClientSetKey)
	if !ok {
		return "", fmt.Errorf("unsupported ClientSetKey: expected %T got %T", ShootClientSetKey{}, k)
	}

	seedNamespace, seedClient, err := f.getSeedNamespace(ctx, key)
	if err != nil {
		return "", err
	}

	kubeconfigSecret := &corev1.Secret{}
	if err := seedClient.Client().Get(ctx, client.ObjectKey{Namespace: seedNamespace, Name: v1beta1constants.SecretNameGardener}, kubeconfigSecret); err != nil {
		return "", err
	}

	return utils.ComputeSHA256Hex(kubeconfigSecret.Data[kubernetes.KubeConfig]), nil
}

// NewClientSet creates a new ClientSet for a Shoot cluster.
func (f *ShootClientSetFactory) NewClientSet(ctx context.Context, k clientmap.ClientSetKey) (kubernetes.Interface, error) {
	key, ok := k.(ShootClientSetKey)
	if !ok {
		return nil, fmt.Errorf("unsupported ClientSetKey: expected %T got %T", ShootClientSetKey{}, k)
	}

	seedNamespace, seedClient, err := f.getSeedNamespace(ctx, key)
	if err != nil {
		return nil, err
	}

	secretName := v1beta1constants.SecretNameGardener
	// If the gardenlet runs in the same cluster like the API server of the shoot then use the internal kubeconfig
	// and communicate internally. Otherwise, fall back to the "external" kubeconfig and communicate via the
	// load balancer of the shoot API server.
	addr, err := LookupHost(fmt.Sprintf("%s.%s.svc", v1beta1constants.DeploymentNameKubeAPIServer, seedNamespace))
	if err != nil {
		f.Log.Warnf("service DNS name lookup of kube-apiserver failed (%+v), falling back to external kubeconfig", err)
	} else if len(addr) > 0 {
		secretName = v1beta1constants.SecretNameGardenerInternal
	}

	clientOptions := client.Options{
		Scheme: kubernetes.ShootScheme,
	}

	clientSet, err := NewClientFromSecret(ctx, seedClient.Client(), seedNamespace, secretName,
		kubernetes.WithClientConnectionOptions(f.ClientConnectionConfig),
		kubernetes.WithClientOptions(clientOptions),
		kubernetes.WithDisabledCachedClient(),
	)

	if secretName == v1beta1constants.SecretNameGardenerInternal && err != nil && apierrors.IsNotFound(err) {
		clientSet, err = NewClientFromSecret(ctx, seedClient.Client(), seedNamespace, v1beta1constants.SecretNameGardener,
			kubernetes.WithClientConnectionOptions(f.ClientConnectionConfig),
			kubernetes.WithClientOptions(clientOptions),
			kubernetes.WithDisabledCachedClient(),
		)
	}

	return clientSet, err
}

var _ clientmap.Invalidate = &ShootClientSetFactory{}

// InvalidateClient invalidates information cached for the given ClientSetKey in the factory.
func (f *ShootClientSetFactory) InvalidateClient(k clientmap.ClientSetKey) error {
	key, ok := k.(ShootClientSetKey)
	if !ok {
		return fmt.Errorf("unsupported ClientSetKey: expected %T got %T", ShootClientSetKey{}, k)
	}
	delete(f.clientKeyToSeedInfo, key)
	return nil
}

func (f *ShootClientSetFactory) seedInfoFromCache(ctx context.Context, key ShootClientSetKey) (string, kubernetes.Interface, error) {
	cache, ok := f.clientKeyToSeedInfo[key]
	if !ok {
		return "", nil, fmt.Errorf("no seed info cached for client %s", key)
	}
	seedClient, err := f.GetSeedClient(ctx, cache.seedName)
	if err != nil {
		return "", nil, fmt.Errorf("failed to get seed client from cached seed info %w", err)
	}

	return cache.namespace, seedClient, nil
}

func (f *ShootClientSetFactory) seedInfoToCache(key ShootClientSetKey, namespace, seedName string) {
	f.clientKeyToSeedInfo[key] = seedInfo{
		namespace: namespace,
		seedName:  seedName,
	}
}

func (f *ShootClientSetFactory) getSeedNamespace(ctx context.Context, key ShootClientSetKey) (string, kubernetes.Interface, error) {
	if namespace, seedClient, err := f.seedInfoFromCache(ctx, key); err == nil {
		return namespace, seedClient, nil
	}

	gardenClient, err := f.GetGardenClient(ctx)
	if err != nil {
		return "", nil, fmt.Errorf("failed to get garden client: %w", err)
	}

	shoot := &gardencorev1beta1.Shoot{}
	if err := gardenClient.Client().Get(ctx, client.ObjectKey{Namespace: key.Namespace, Name: key.Name}, shoot); err != nil {
		return "", nil, fmt.Errorf("failed to get Shoot object %q: %w", key.Key(), err)
	}

	seedName := shoot.Spec.SeedName
	if seedName == nil {
		return "", nil, fmt.Errorf("shoot %q is not scheduled yet", key.Key())
	}

	seedClient, err := f.GetSeedClient(ctx, *seedName)
	if err != nil {
		return "", nil, fmt.Errorf("failed to get seed client: %w", err)
	}

	var namespace string
	if len(shoot.Status.TechnicalID) > 0 {
		namespace = shoot.Status.TechnicalID
	} else {
		project, err := ProjectForNamespaceWithClient(ctx, gardenClient.Client(), shoot.Namespace)
		if err != nil {
			return "", seedClient, fmt.Errorf("failed to get Project for Shoot %q: %w", key.Key(), err)
		}
		namespace = shootpkg.ComputeTechnicalID(project.Name, shoot)
	}

	f.seedInfoToCache(key, namespace, *seedName)

	return namespace, seedClient, nil
}

// ShootClientSetKey is a ClientSetKey for a Shoot cluster.
type ShootClientSetKey struct {
	Namespace, Name string
}

func (k ShootClientSetKey) Key() string {
	return k.Namespace + "/" + k.Name
}
