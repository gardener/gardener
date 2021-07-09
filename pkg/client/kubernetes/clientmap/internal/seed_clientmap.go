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
	baseconfig "k8s.io/component-base/config"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
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
	// ClientConnectionConfiguration is the configuration that will be used by created ClientSets.
	ClientConnectionConfig baseconfig.ClientConnectionConfiguration
}

// CalculateClientSetHash always returns "" and nil.
func (f *SeedClientSetFactory) CalculateClientSetHash(ctx context.Context, k clientmap.ClientSetKey) (string, error) {
	return "", nil
}

// NewClientSet creates a new ClientSet for a Seed cluster.
func (f *SeedClientSetFactory) NewClientSet(ctx context.Context, k clientmap.ClientSetKey) (kubernetes.Interface, error) {
	_, ok := k.(SeedClientSetKey)
	if !ok {
		return nil, fmt.Errorf("unsupported ClientSetKey: expected %T, but got %T", SeedClientSetKey(""), k)
	}

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

// SeedClientSetKey is a ClientSetKey for a Seed cluster.
type SeedClientSetKey string

func (k SeedClientSetKey) Key() string {
	return string(k)
}
